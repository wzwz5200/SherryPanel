//go:build windows

package serverctl

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

var (
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	procGetDiskFreeSpaceEx = kernel32.NewProc("GetDiskFreeSpaceExW")
	procGlobalMemoryStatus = kernel32.NewProc("GlobalMemoryStatusEx")

	winMu   sync.Mutex
	winProc *exec.Cmd
)

type memoryStatusEx struct {
	dwLength                uint32
	dwMemoryLoad            uint32
	ullTotalPhys            uint64
	ullAvailPhys            uint64
	ullTotalPageFile        uint64
	ullAvailPageFile        uint64
	ullTotalVirtual         uint64
	ullAvailVirtual         uint64
	ullAvailExtendedVirtual uint64
}

func isWinRunning() bool {
	winMu.Lock()
	defer winMu.Unlock()
	return winProc != nil && winProc.Process != nil && winProc.ProcessState == nil
}

func stopWinProc() {
	winMu.Lock()
	p := winProc
	winProc = nil
	winMu.Unlock()
	if p != nil && p.Process != nil {
		_ = p.Process.Kill()
		_, _ = p.Process.Wait()
	}
}

func startWinProc(cs2Dir, envFile string) (string, error) {
	bin := cs2BinaryPath(cs2Dir)
	if _, err := os.Stat(bin); err != nil {
		return "", fmt.Errorf("未找到 Windows CS2 可执行文件: %s（请先执行自动下载安装）", bin)
	}
	s := ReadEnv(envFile)
	args := []string{
		"-dedicated", "-console", "-usercon",
		"-port", s.Port, "-maxplayers", s.MaxPlayers,
		"+game_type", s.GameType, "+game_mode", s.GameMode,
		"+map", s.Map, "+hostname", s.Hostname,
		"+rcon_password", s.RCONPassword,
	}
	if s.GSLT != "" {
		args = append(args, "+sv_setsteamaccount", s.GSLT)
	}
	if s.SVPassword != "" {
		args = append(args, "+sv_password", s.SVPassword)
	}
	cmd := exec.Command(bin, args...)
	cmd.Dir = filepath.Dir(bin)
	if err := cmd.Start(); err != nil {
		return "", err
	}
	winMu.Lock()
	winProc = cmd
	winMu.Unlock()
	go func() {
		_ = cmd.Wait()
		winMu.Lock()
		if winProc == cmd {
			winProc = nil
		}
		winMu.Unlock()
	}()
	return fmt.Sprintf("Windows CS2 进程已启动 (PID=%d)", cmd.Process.Pid), nil
}

func ServiceAction(serviceName, cs2Dir, envFile, action string) (string, error) {
	switch action {
	case "stop":
		stopWinProc()
		return "Windows CS2 进程已停止", nil
	case "start":
		if isWinRunning() {
			return "CS2 进程已在运行中", nil
		}
		return startWinProc(cs2Dir, envFile)
	case "restart":
		stopWinProc()
		return startWinProc(cs2Dir, envFile)
	default:
		return "", fmt.Errorf("unsupported action: %s", action)
	}
}

func Metrics(serviceName, cs2Dir string) SystemMetrics {
	m := SystemMetrics{
		OS:            runtime.GOOS,
		Arch:          runtime.GOARCH,
		ServiceActive: isWinRunning(),
		LoadAvg:       "Windows Native",
	}
	if _, err := os.Stat(cs2BinaryPath(cs2Dir)); err == nil {
		m.CS2Installed = true
	}
	if _, err := os.Stat(filepath.Join(cs2Dir, "game", "csgo", "addons", "counterstrikesharp", "plugins", "WeaponPaints")); err == nil {
		m.PluginsReady = true
	}

	root := cs2Dir
	if _, err := os.Stat(root); err != nil {
		root, _ = os.Getwd()
	}
	if vol := filepath.VolumeName(root); vol != "" {
		root = vol + `\`
	}
	var freeAvail, totalBytes, totalFree uint64
	if ptr, err := syscall.UTF16PtrFromString(root); err == nil {
		r1, _, _ := procGetDiskFreeSpaceEx.Call(
			uintptr(unsafe.Pointer(ptr)),
			uintptr(unsafe.Pointer(&freeAvail)),
			uintptr(unsafe.Pointer(&totalBytes)),
			uintptr(unsafe.Pointer(&totalFree)),
		)
		if r1 != 0 {
			m.DiskTotalGB = float64(totalBytes) / (1024 * 1024 * 1024)
			m.DiskFreeGB = float64(freeAvail) / (1024 * 1024 * 1024)
		}
	}

	var ms memoryStatusEx
	ms.dwLength = uint32(unsafe.Sizeof(ms))
	if r1, _, _ := procGlobalMemoryStatus.Call(uintptr(unsafe.Pointer(&ms))); r1 != 0 {
		m.MemTotalMB = int64(ms.ullTotalPhys / (1024 * 1024))
		m.MemAvailMB = int64(ms.ullAvailPhys / (1024 * 1024))
		m.LoadAvg = fmt.Sprintf("MemLoad %d%%", ms.dwMemoryLoad)
	}
	return m
}

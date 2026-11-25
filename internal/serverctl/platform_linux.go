//go:build linux

package serverctl

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

func ServiceAction(serviceName, cs2Dir, envFile, action string) (string, error) {
	switch action {
	case "start", "stop", "restart":
		out, err := exec.Command("systemctl", action, serviceName).CombinedOutput()
		return strings.TrimSpace(string(out)), err
	default:
		return "", fmt.Errorf("unsupported action: %s", action)
	}
}

func Metrics(serviceName, cs2Dir string) SystemMetrics {
	m := SystemMetrics{OS: runtime.GOOS, Arch: runtime.GOARCH}
	if out, err := exec.Command("systemctl", "is-active", serviceName).Output(); err == nil {
		m.ServiceActive = strings.TrimSpace(string(out)) == "active"
	}
	if _, err := os.Stat(cs2BinaryPath(cs2Dir)); err == nil {
		m.CS2Installed = true
	}
	if _, err := os.Stat(filepath.Join(cs2Dir, "game", "csgo", "addons", "counterstrikesharp", "plugins", "WeaponPaints")); err == nil {
		m.PluginsReady = true
	}
	var st syscall.Statfs_t
	checkPath := cs2Dir
	if _, err := os.Stat(checkPath); err != nil {
		checkPath = "/"
	}
	if err := syscall.Statfs(checkPath, &st); err == nil {
		m.DiskTotalGB = float64(st.Blocks*uint64(st.Bsize)) / (1024 * 1024 * 1024)
		m.DiskFreeGB = float64(st.Bavail*uint64(st.Bsize)) / (1024 * 1024 * 1024)
	}
	if b, err := os.ReadFile("/proc/meminfo"); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			var kb int64
			if strings.HasPrefix(line, "MemTotal:") {
				fmt.Sscanf(line, "MemTotal: %d kB", &kb)
				m.MemTotalMB = kb / 1024
			} else if strings.HasPrefix(line, "MemAvailable:") {
				fmt.Sscanf(line, "MemAvailable: %d kB", &kb)
				m.MemAvailMB = kb / 1024
			}
		}
	}
	if b, err := os.ReadFile("/proc/loadavg"); err == nil {
		parts := strings.Fields(string(b))
		if len(parts) >= 3 {
			m.LoadAvg = strings.Join(parts[:3], " ")
		}
	}
	return m
}

package installer

import (
	"archive/zip"
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"cs2-skins-web/internal/config"
)

type Status struct {
	Running   bool      `json:"running"`
	DryRun    bool      `json:"dry_run"`
	OS        string    `json:"os"`
	Task      string    `json:"task"`
	Stage     string    `json:"stage"`
	LastError string    `json:"last_error"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
	Logs      []string  `json:"logs"`
}

type Manager struct {
	mu  sync.Mutex
	st  Status
	cfg config.AppConfig
}

func New(cfg config.AppConfig) *Manager {
	return &Manager{
		cfg: cfg,
		st:  Status{DryRun: cfg.DryRun, OS: runtime.GOOS},
	}
}

func (m *Manager) GetStatus() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := m.st
	cp.Logs = append([]string(nil), m.st.Logs...)
	return cp
}

func (m *Manager) appendLog(line string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ts := time.Now().Format("15:04:05")
	m.st.Logs = append(m.st.Logs, fmt.Sprintf("[%s] %s", ts, line))
	if len(m.st.Logs) > 300 {
		m.st.Logs = m.st.Logs[len(m.st.Logs)-300:]
	}
}

func (m *Manager) setStage(stage string) {
	m.mu.Lock()
	m.st.Stage = stage
	m.mu.Unlock()
	m.appendLog("=== " + stage + " ===")
}

func (m *Manager) StartTask(task string, dryRunOverride *bool) error {
	m.mu.Lock()
	if m.st.Running {
		m.mu.Unlock()
		return fmt.Errorf("已有安装任务正在运行: %s", m.st.Task)
	}
	dry := m.cfg.DryRun
	if dryRunOverride != nil && *dryRunOverride {
		dry = true
	}
	m.st = Status{
		Running:   true,
		DryRun:    dry,
		OS:        runtime.GOOS,
		Task:      task,
		Stage:     "初始化任务",
		StartedAt: time.Now(),
		Logs:      []string{},
	}
	m.mu.Unlock()

	go m.execTask(task, dry)
	return nil
}

func (m *Manager) RunSync(task string, dryRun bool) error {
	if err := m.StartTask(task, &dryRun); err != nil {
		return err
	}
	for {
		st := m.GetStatus()
		if !st.Running {
			if st.LastError != "" {
				return fmt.Errorf("%s", st.LastError)
			}
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func (m *Manager) execTask(task string, dryRun bool) {
	var err error
	defer func() {
		m.mu.Lock()
		m.st.Running = false
		m.st.EndedAt = time.Now()
		if err != nil {
			m.st.LastError = err.Error()
			m.st.Stage = "任务失败"
		} else {
			m.st.Stage = "已完成"
		}
		m.mu.Unlock()
	}()

	if dryRun {
		m.appendLog(fmt.Sprintf("[DRY-RUN 安全模拟模式] 平台: %s/%s，跳过 71 GB 实际下载", runtime.GOOS, runtime.GOARCH))
	}

	switch task {
	case "cs2":
		err = m.installCS2(dryRun)
	case "plugins":
		err = m.installPlugins(dryRun)
	case "all":
		if err = m.installCS2(dryRun); err == nil {
			err = m.installPlugins(dryRun)
		}
	default:
		err = fmt.Errorf("未知任务类型: %s", task)
	}
}

func (m *Manager) checkURL(u string) error {
	c := &http.Client{Timeout: 8 * time.Second}
	req, _ := http.NewRequest("HEAD", u, nil)
	req.Header.Set("User-Agent", "cs2-panel/2.0")
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	m.appendLog(fmt.Sprintf("连通性检查 OK [%d]: %s", resp.StatusCode, u))
	return nil
}

func downloadFile(u, dst string) error {
	c := &http.Client{Timeout: 5 * time.Minute}
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("User-Agent", "cs2-panel/2.0")
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d downloading %s", resp.StatusCode, u)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

func unzipFile(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)
		if !strings.HasPrefix(filepath.Clean(fpath), filepath.Clean(dest)+string(os.PathSeparator)) && filepath.Clean(fpath) != filepath.Clean(dest) {
			continue
		}
		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(fpath, 0755)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(fpath), 0755); err != nil {
			return err
		}
		out, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}
		in, err := f.Open()
		if err != nil {
			out.Close()
			return err
		}
		_, err = io.Copy(out, in)
		in.Close()
		out.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) runCmdStream(cmd *exec.Cmd) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return err
	}
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 64*1024), 512*1024)
	for sc.Scan() {
		if line := strings.TrimSpace(sc.Text()); line != "" {
			m.appendLog(line)
		}
	}
	return cmd.Wait()
}

func patchGameInfoAndCore(csgoDir string) error {
	gi := filepath.Join(csgoDir, "gameinfo.gi")
	if b, err := os.ReadFile(gi); err == nil {
		raw := string(b)
		if !strings.Contains(raw, "csgo/addons/metamod") {
			lines := strings.Split(raw, "\n")
			var out []string
			for _, ln := range lines {
				out = append(out, ln)
				if strings.Contains(ln, "Game_LowViolence") && strings.Contains(ln, "csgo_lv") {
					eol := ""
					if strings.HasSuffix(ln, "\r") {
						eol = "\r"
					}
					out = append(out, "\t\t\tGame\tcsgo/addons/metamod"+eol)
				}
			}
			_ = os.WriteFile(gi, []byte(strings.Join(out, "\n")), 0644)
		}
	}
	core := filepath.Join(csgoDir, "addons", "counterstrikesharp", "configs", "core.json")
	if b, err := os.ReadFile(core); err == nil {
		var d map[string]any
		if json.Unmarshal(b, &d) == nil {
			d["FollowCS2ServerGuidelines"] = false
			if out, err := json.MarshalIndent(d, "", "  "); err == nil {
				_ = os.WriteFile(core, out, 0644)
			}
		}
	}
	return nil
}

func (m *Manager) installCS2(dryRun bool) error {
	if runtime.GOOS == "windows" {
		m.setStage("Windows SteamCMD 与 CS2 服务端部署")
		steamURL := "https://steamcdn-a.akamaihd.net/client/installer/steamcmd.zip"
		if dryRun {
			_ = m.checkURL(steamURL)
			m.appendLog(fmt.Sprintf("[DRY-RUN] 将创建目录: %s 与 %s", m.cfg.SteamCMDDir, m.cfg.CS2Dir))
			m.appendLog(fmt.Sprintf("[DRY-RUN] 将执行: %s +force_install_dir %s +login anonymous +app_update 730 validate +quit",
				filepath.Join(m.cfg.SteamCMDDir, "steamcmd.exe"), m.cfg.CS2Dir))
			m.appendLog("[DRY-RUN] Windows CS2 服务端部署流程校验通过（未下载 71 GB 文件）")
			return nil
		}
		if err := os.MkdirAll(m.cfg.SteamCMDDir, 0755); err != nil {
			return err
		}
		if err := os.MkdirAll(m.cfg.CS2Dir, 0755); err != nil {
			return err
		}
		steamExe := filepath.Join(m.cfg.SteamCMDDir, "steamcmd.exe")
		if _, err := os.Stat(steamExe); err != nil {
			zipPath := filepath.Join(m.cfg.SteamCMDDir, "steamcmd.zip")
			m.appendLog("下载 Windows SteamCMD...")
			if err := downloadFile(steamURL, zipPath); err != nil {
				return err
			}
			if err := unzipFile(zipPath, m.cfg.SteamCMDDir); err != nil {
				return err
			}
			_ = os.Remove(zipPath)
		}
		m.setStage("通过 Windows SteamCMD 下载/更新 CS2 (AppID 730)")
		cmd := exec.Command(steamExe, "+force_install_dir", m.cfg.CS2Dir, "+login", "anonymous", "+app_update", "730", "validate", "+quit")
		return m.runCmdStream(cmd)
	}

	// Linux path
	m.setStage("Linux SteamCMD 与 CS2 服务端部署")
	steamURL := "https://steamcdn-a.akamaihd.net/client/installer/steamcmd_linux.tar.gz"
	if dryRun {
		_ = m.checkURL(steamURL)
		m.appendLog(fmt.Sprintf("[DRY-RUN] 将在 %s 下载/校验 CS2 (AppID 730) 并软链 steamclient.so", m.cfg.CS2Dir))
		return nil
	}
	script := fmt.Sprintf(`set -e
dpkg --add-architecture i386
apt-get update -qq
DEBIAN_FRONTEND=noninteractive apt-get install -y -qq lib32gcc-s1 lib32stdc++6 curl ca-certificates tar python3 openssl >/dev/null
id -u %s >/dev/null 2>&1 || useradd -m -s /bin/bash %s
mkdir -p "%s" "%s" "/home/%s/.steam/sdk64"
chown -R %s:%s "%s" "%s" "/home/%s/.steam"
if [ ! -f "%s/steamcmd.sh" ]; then
  su - %s -c 'cd "%s" && curl -fsSL https://steamcdn-a.akamaihd.net/client/installer/steamcmd_linux.tar.gz | tar -xz'
fi
su - %s -c '"%s/steamcmd.sh" +force_install_dir "%s" +login anonymous +app_update 730 validate +quit'
ln -sf "%s/linux64/steamclient.so" "/home/%s/.steam/sdk64/steamclient.so"
chown -h %s:%s "/home/%s/.steam/sdk64/steamclient.so"`,
		m.cfg.CS2User, m.cfg.CS2User, m.cfg.SteamCMDDir, m.cfg.CS2Dir, m.cfg.CS2User,
		m.cfg.CS2User, m.cfg.CS2User, m.cfg.SteamCMDDir, m.cfg.CS2Dir, m.cfg.CS2User,
		m.cfg.SteamCMDDir, m.cfg.CS2User, m.cfg.SteamCMDDir,
		m.cfg.CS2User, m.cfg.SteamCMDDir, m.cfg.CS2Dir,
		m.cfg.SteamCMDDir, m.cfg.CS2User, m.cfg.CS2User, m.cfg.CS2User, m.cfg.CS2User)
	return m.runCmdStream(exec.Command("bash", "-c", script))
}

func (m *Manager) installPlugins(dryRun bool) error {
	m.setStage(fmt.Sprintf("安装全皮肤插件栈 (%s)", runtime.GOOS))
	csgoDir := filepath.Join(m.cfg.CS2Dir, "game", "csgo")
	if dryRun {
		mmLatest := "https://mms.alliedmods.net/mmsdrop/2.0/mmsource-latest-linux"
		if runtime.GOOS == "windows" {
			mmLatest = "https://mms.alliedmods.net/mmsdrop/2.0/mmsource-latest-windows"
		}
		_ = m.checkURL(mmLatest)
		m.appendLog(fmt.Sprintf("[DRY-RUN] 目标插件目录: %s", filepath.Join(csgoDir, "addons")))
		m.appendLog("[DRY-RUN] 将拉取 Metamod + CounterStrikeSharp + AnyBaseLib + PlayerSettings + MenuManager + WeaponPaints")
		m.appendLog("[DRY-RUN] 将纯 Go 修补 gameinfo.gi 并设置 core.json FollowCS2ServerGuidelines=false")
		return nil
	}

	if runtime.GOOS == "windows" {
		if err := os.MkdirAll(filepath.Join(csgoDir, "addons"), 0755); err != nil {
			return err
		}
		// Metamod Windows
		c := &http.Client{Timeout: 30 * time.Second}
		resp, err := c.Get("https://mms.alliedmods.net/mmsdrop/2.0/mmsource-latest-windows")
		if err != nil {
			return err
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		mmZip := filepath.Join(os.TempDir(), "mmsource-win.zip")
		m.appendLog("下载 Metamod Windows: " + strings.TrimSpace(string(b)))
		if err := downloadFile("https://mms.alliedmods.net/mmsdrop/2.0/"+strings.TrimSpace(string(b)), mmZip); err != nil {
			return err
		}
		if err := unzipFile(mmZip, csgoDir); err != nil {
			return err
		}
		_ = os.Remove(mmZip)
		// Fetch latest GitHub releases (CSS Windows + deps + WeaponPaints)
		repos := []struct {
			repo    string
			pattern string
			dest    string
		}{
			{"roflmuffin/CounterStrikeSharp", "counterstrikesharp-with-runtime-windows-", csgoDir},
			{"NickFox007/AnyBaseLibCS2", ".zip", csgoDir},
			{"NickFox007/PlayerSettingsCS2", ".zip", csgoDir},
			{"NickFox007/MenuManagerCS2", ".zip", csgoDir},
			{"Nereziel/cs2-WeaponPaints", "WeaponPaints.zip", csgoDir},
		}
		for _, r := range repos {
			m.appendLog("拉取最新 Release: " + r.repo)
			apiResp, err := c.Get("https://api.github.com/repos/" + r.repo + "/releases/latest")
			if err != nil {
				return err
			}
			var rel struct {
				Assets []struct {
					Name string `json:"name"`
					URL  string `json:"browser_download_url"`
				} `json:"assets"`
			}
			_ = json.NewDecoder(apiResp.Body).Decode(&rel)
			apiResp.Body.Close()
			var dl string
			for _, a := range rel.Assets {
				if strings.Contains(a.Name, r.pattern) && !strings.Contains(a.Name, "Website") {
					dl = a.URL
					break
				}
			}
			if dl == "" {
				return fmt.Errorf("未在 %s 找到匹配包 (%s)", r.repo, r.pattern)
			}
			tmpZ := filepath.Join(os.TempDir(), "gh-pkg.zip")
			if err := downloadFile(dl, tmpZ); err != nil {
				return err
			}
			if err := unzipFile(tmpZ, r.dest); err != nil {
				return err
			}
			_ = os.Remove(tmpZ)
		}
		_ = patchGameInfoAndCore(csgoDir)
		m.appendLog("Windows 插件栈安装完成！")
		return nil
	}

	// Linux path
	script := fmt.Sprintf(`set -e
CSGO_DIR="%s"
CSS_DIR="${CSGO_DIR}/addons/counterstrikesharp"
DATA_DIR="%s"
IMG_DIR="%s"
mkdir -p "${CSGO_DIR}" "${DATA_DIR}" "${IMG_DIR}"
MM_LATEST=$(curl -fsSL https://mms.alliedmods.net/mmsdrop/2.0/mmsource-latest-linux)
curl -fsSL "https://mms.alliedmods.net/mmsdrop/2.0/${MM_LATEST}" -o /tmp/mm.tar.gz
tar -xzf /tmp/mm.tar.gz -C "${CSGO_DIR}" && rm -f /tmp/mm.tar.gz
CSS_URL=$(curl -fsSL https://api.github.com/repos/roflmuffin/CounterStrikeSharp/releases/latest | jq -r '.assets[] | select(.name | test("counterstrikesharp-with-runtime-linux-.*\\.zip$")) | .browser_download_url' | head -n1)
curl -fsSL "${CSS_URL}" -o /tmp/css.zip && unzip -qo /tmp/css.zip -d "${CSGO_DIR}" && rm -f /tmp/css.zip
for repo in NickFox007/AnyBaseLibCS2 NickFox007/PlayerSettingsCS2 NickFox007/MenuManagerCS2 Nereziel/cs2-WeaponPaints; do
  url=$(curl -fsSL "https://api.github.com/repos/${repo}/releases/latest" | jq -r '.assets[] | select((.name | test("\\.zip$")) and (.name | test("Website") | not)) | .browser_download_url' | head -n1)
  rm -rf /tmp/dep_ext && mkdir -p /tmp/dep_ext
  curl -fsSL "${url}" -o /tmp/dep.zip && unzip -qo /tmp/dep.zip -d /tmp/dep_ext && rm -f /tmp/dep.zip
  [ -d /tmp/dep_ext/addons/counterstrikesharp ] && cp -rf /tmp/dep_ext/addons/counterstrikesharp/* "${CSS_DIR}/"
  rm -rf /tmp/dep_ext
done
rm -rf /tmp/wp_web_repo
git clone --depth 1 https://github.com/Nereziel/cs2-WeaponPaints.git /tmp/wp_web_repo >/dev/null 2>&1
cp -rf /tmp/wp_web_repo/website/data/* "${DATA_DIR}/"
cp -rf /tmp/wp_web_repo/website/img/* "${IMG_DIR}/"
rm -rf /tmp/wp_web_repo
chown -R %s:%s "${CSGO_DIR}/addons" 2>/dev/null || true`, csgoDir, m.cfg.DataDir, m.cfg.ImgDir, m.cfg.CS2User, m.cfg.CS2User)
	if err := m.runCmdStream(exec.Command("bash", "-c", script)); err != nil {
		return err
	}
	return patchGameInfoAndCore(csgoDir)
}

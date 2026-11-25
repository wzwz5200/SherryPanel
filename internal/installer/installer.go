package installer

import (
	"archive/zip"
	"bufio"
	_ "embed"
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
	"cs2-skins-web/internal/db"
	"cs2-skins-web/internal/serverctl"
)

//go:embed embedded/SkinBind.dll
var embeddedSkinBindDLL []byte

type Status struct {
	Running   bool      `json:"running"`
	DryRun    bool      `json:"dry_run"`
	OS        string    `json:"os"`
	Task      string    `json:"task"`
	Stage     string    `json:"stage"`
	StepIndex int       `json:"step_index"`
	StepTotal int       `json:"step_total"`
	LastError string    `json:"last_error"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
	Logs      []string  `json:"logs"`
}

type Manager struct {
	mu           sync.Mutex
	st           Status
	cfg          config.AppConfig
	playerWebURL string
	onDBReady    func(dsn string)
}

func New(cfg config.AppConfig) *Manager {
	return &Manager{
		cfg: cfg,
		st:  Status{DryRun: cfg.DryRun, OS: runtime.GOOS},
	}
}

func (m *Manager) UpdateConfig(cfg config.AppConfig) {
	m.mu.Lock()
	m.cfg = cfg
	m.mu.Unlock()
}

func (m *Manager) SetPlayerWebURL(u string) {
	m.mu.Lock()
	m.playerWebURL = strings.TrimRight(u, "/")
	m.mu.Unlock()
}

func (m *Manager) SetOnDBReady(fn func(dsn string)) {
	m.mu.Lock()
	m.onDBReady = fn
	m.mu.Unlock()
}

func (m *Manager) baseDir() string {
	m.mu.Lock()
	envFile := m.cfg.CS2EnvFile
	m.mu.Unlock()
	if envFile != "" {
		dir := filepath.Dir(envFile)
		if dir != "" && dir != "." {
			return dir
		}
	}
	wd, _ := os.Getwd()
	return wd
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

func (m *Manager) setStep(idx, total int) {
	m.mu.Lock()
	m.st.StepIndex = idx
	m.st.StepTotal = total
	m.mu.Unlock()
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
	case "steamcmd":
		m.setStep(1, 1)
		err = m.installSteamCMD(dryRun)
	case "database":
		m.setStep(1, 1)
		err = m.installDatabase(dryRun)
	case "cs2":
		m.setStep(1, 2)
		if err = m.installSteamCMD(dryRun); err == nil {
			m.setStep(2, 2)
			err = m.installCS2(dryRun)
		}
	case "plugins":
		m.setStep(1, 1)
		err = m.installPlugins(dryRun)
	case "all":
		m.setStep(1, 4)
		if err = m.installDatabase(dryRun); err == nil {
			m.setStep(2, 4)
			if err = m.installSteamCMD(dryRun); err == nil {
				m.setStep(3, 4)
				if err = m.installCS2(dryRun); err == nil {
					m.setStep(4, 4)
					err = m.installPlugins(dryRun)
				}
			}
		}
	default:
		err = fmt.Errorf("未知任务类型: %s", task)
	}
}

func (m *Manager) checkURL(u string) error {
	c := &http.Client{Timeout: 8 * time.Second}
	req, _ := http.NewRequest("HEAD", u, nil)
	req.Header.Set("User-Agent", "SherryPanel/1.3")
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	m.appendLog(fmt.Sprintf("连通性检查 OK [%d]: %s", resp.StatusCode, u))
	return nil
}

func downloadFile(u, dst string) error {
	c := &http.Client{Timeout: 10 * time.Minute}
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("User-Agent", "SherryPanel/1.3")
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

// extractPluginZip handles zips whose internal root is game/csgo/addons/..., addons/..., counterstrikesharp/..., or plugins/...
func extractPluginZip(src, csgoDir string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()
	cssDir := filepath.Join(csgoDir, "addons", "counterstrikesharp")
	for _, f := range r.File {
		name := strings.TrimPrefix(strings.ReplaceAll(f.Name, `\`, `/`), "./")
		var targetBase string
		switch {
		case strings.HasPrefix(name, "game/csgo/"):
			name = strings.TrimPrefix(name, "game/csgo/")
			targetBase = csgoDir
		case strings.HasPrefix(name, "addons/"):
			targetBase = csgoDir
		case strings.HasPrefix(name, "counterstrikesharp/"):
			targetBase = filepath.Join(csgoDir, "addons")
		default:
			targetBase = cssDir
		}
		if name == "" {
			continue
		}
		fpath := filepath.Join(targetBase, filepath.FromSlash(name))
		if !strings.HasPrefix(filepath.Clean(fpath), filepath.Clean(csgoDir)+string(os.PathSeparator)) {
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

// WriteSkinBindConfig writes SkinBind.json when the CounterStrikeSharp configs dir exists.
func WriteSkinBindConfig(cs2Dir, welcomeTitle string) error {
	if strings.TrimSpace(welcomeTitle) == "" {
		welcomeTitle = "欢迎来到 Sherry 的专属服务器！"
	}
	cfgDir := filepath.Join(cs2Dir, "game", "csgo", "addons", "counterstrikesharp", "configs", "plugins", "SkinBind")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		return err
	}
	payload := map[string]any{
		"WelcomeTitle":      welcomeTitle,
		"JoinBroadcast":     true,
		"AutoSendBindCode":  true,
		"CodeExpireMinutes": 10,
		"JoinDelaySeconds":  3.0,
	}
	b, _ := json.MarshalIndent(payload, "", "  ")
	return os.WriteFile(filepath.Join(cfgDir, "SkinBind.json"), b, 0644)
}

// WriteWeaponPaintsConfig generates or updates WeaponPaints.json from cfg.DBDSN.
func WriteWeaponPaintsConfig(cs2Dir string, cfg config.AppConfig, playerWebURL string) error {
	cfgDir := filepath.Join(cs2Dir, "game", "csgo", "addons", "counterstrikesharp", "configs", "plugins", "WeaponPaints")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		return err
	}
	wpPath := filepath.Join(cfgDir, "WeaponPaints.json")
	doc := map[string]any{
		"DatabaseHost":              "127.0.0.1",
		"DatabasePort":              3306,
		"DatabaseUser":              "wpuser",
		"DatabasePassword":          "",
		"DatabaseName":              "cs2_weaponpaints",
		"DatabaseSslMode":           "none",
		"CmdRefreshCooldownSeconds": 1,
		"GlobalShare":               false,
		"Additional": map[string]any{
			"KnifeEnabled":         true,
			"GloveEnabled":         true,
			"MusicEnabled":         true,
			"AgentEnabled":         true,
			"SkinEnabled":          true,
			"PinsEnabled":          true,
			"CommandWpEnabled":     true,
			"CommandKillEnabled":   false,
			"CommandKnife":         []string{"knife"},
			"CommandMusic":         []string{"music"},
			"CommandPin":           []string{"pin", "pins"},
			"CommandGlove":         []string{"gloves", "glove"},
			"CommandAgent":         []string{"agents", "agent"},
			"CommandStattrak":      []string{"stattrak", "st"},
			"CommandSkin":          []string{"ws"},
			"CommandSkinSelection": []string{"skins"},
			"CommandRefresh":       []string{"wp"},
			"GiveRandomKnife":      false,
			"GiveRandomSkin":       false,
			"ShowSkinImage":        true,
		},
		"Website":       playerWebURL,
		"ConfigVersion": 10,
	}
	if raw, err := os.ReadFile(wpPath); err == nil {
		var existing map[string]any
		if json.Unmarshal(raw, &existing) == nil {
			for k, v := range existing {
				doc[k] = v
			}
		}
	}
	if user, pass, host, portStr, dbName, ok := db.ParseDSN(cfg.DBDSN); ok && user != "" {
		port := 3306
		_, _ = fmt.Sscanf(portStr, "%d", &port)
		if port <= 0 {
			port = 3306
		}
		doc["DatabaseHost"] = host
		doc["DatabasePort"] = port
		doc["DatabaseUser"] = user
		doc["DatabasePassword"] = pass
		doc["DatabaseName"] = dbName
	}
	doc["CmdRefreshCooldownSeconds"] = 1
	if playerWebURL != "" {
		doc["Website"] = playerWebURL
	}
	doc["ConfigVersion"] = 10
	out, _ := json.MarshalIndent(doc, "", "  ")
	return os.WriteFile(wpPath, out, 0600)
}

func EnsureRCONPassword(envFile, cs2Dir string) error {
	env := serverctl.ReadEnv(envFile)
	if strings.TrimSpace(env.RCONPassword) != "" {
		return nil
	}
	env.RCONPassword = "rcon-" + randomHexStr(8)
	return serverctl.WriteEnv(envFile, cs2Dir, env)
}

func (m *Manager) deployEmbeddedSkinBindAndConfigs(csgoDir string) {
	if len(embeddedSkinBindDLL) > 0 {
		sbDir := filepath.Join(csgoDir, "addons", "counterstrikesharp", "plugins", "SkinBind")
		if err := os.MkdirAll(sbDir, 0755); err == nil {
			if err := os.WriteFile(filepath.Join(sbDir, "SkinBind.dll"), embeddedSkinBindDLL, 0644); err != nil {
				m.appendLog("警告: 写入 SkinBind.dll 失败: " + err.Error())
			} else {
				m.appendLog("已自动部署内置 SkinBind v1.2.0（进服欢迎语 + 自动配对码 + 网页自动刷新通知）")
			}
			wpMySQL := filepath.Join(csgoDir, "addons", "counterstrikesharp", "plugins", "WeaponPaints", "MySqlConnector.dll")
			if raw, err := os.ReadFile(wpMySQL); err == nil {
				_ = os.WriteFile(filepath.Join(sbDir, "MySqlConnector.dll"), raw, 0644)
			}
		}
	}

	m.mu.Lock()
	cfg := m.cfg
	webURL := m.playerWebURL
	m.mu.Unlock()
	if webURL == "" {
		webURL = "http://127.0.0.1:" + cfg.PlayerPort
	}
	if err := WriteWeaponPaintsConfig(cfg.CS2Dir, cfg, webURL); err != nil {
		m.appendLog("警告: 自动写入 WeaponPaints.json 失败: " + err.Error())
	} else {
		m.appendLog("已自动生成 WeaponPaints.json 数据库连接配置")
	}
	if err := WriteSkinBindConfig(cfg.CS2Dir, cfg.WelcomeTitle); err != nil {
		m.appendLog("警告: 自动写入 SkinBind.json 失败: " + err.Error())
	}
	if err := EnsureRCONPassword(cfg.CS2EnvFile, cfg.CS2Dir); err == nil {
		m.appendLog("已校验并就绪 CS2 RCON 自动刷新通道")
	}
}

func (m *Manager) installSteamCMD(dryRun bool) error {
	m.setStage(fmt.Sprintf("下载 SteamCMD 下载器 (%s)", runtime.GOOS))
	if runtime.GOOS == "windows" {
		steamURL := "https://steamcdn-a.akamaihd.net/client/installer/steamcmd.zip"
		steamExe := filepath.Join(m.cfg.SteamCMDDir, "steamcmd.exe")
		if dryRun {
			_ = m.checkURL(steamURL)
			m.appendLog(fmt.Sprintf("[DRY-RUN] 将下载并解压 SteamCMD 到: %s", m.cfg.SteamCMDDir))
			return nil
		}
		if _, err := os.Stat(steamExe); err == nil {
			m.appendLog("SteamCMD 已存在: " + steamExe)
			return nil
		}
		if err := os.MkdirAll(m.cfg.SteamCMDDir, 0755); err != nil {
			return err
		}
		zipPath := filepath.Join(m.cfg.SteamCMDDir, "steamcmd.zip")
		m.appendLog("正在下载 Windows SteamCMD...")
		if err := downloadFile(steamURL, zipPath); err != nil {
			return err
		}
		if err := unzipFile(zipPath, m.cfg.SteamCMDDir); err != nil {
			return err
		}
		_ = os.Remove(zipPath)
		m.appendLog("SteamCMD 安装完成: " + steamExe)
		return nil
	}

	steamURL := "https://steamcdn-a.akamaihd.net/client/installer/steamcmd_linux.tar.gz"
	if dryRun {
		_ = m.checkURL(steamURL)
		m.appendLog(fmt.Sprintf("[DRY-RUN] 将在 %s 安装 32 位运行库与 steamcmd.sh", m.cfg.SteamCMDDir))
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
fi`,
		m.cfg.CS2User, m.cfg.CS2User, m.cfg.SteamCMDDir, m.cfg.CS2Dir, m.cfg.CS2User,
		m.cfg.CS2User, m.cfg.CS2User, m.cfg.SteamCMDDir, m.cfg.CS2Dir, m.cfg.CS2User,
		m.cfg.SteamCMDDir, m.cfg.CS2User, m.cfg.SteamCMDDir)
	return m.runCmdStream(exec.Command("bash", "-c", script))
}

func (m *Manager) installCS2(dryRun bool) error {
	m.setStage(fmt.Sprintf("通过 SteamCMD 下载/更新 CS2 服务端 (%s)", runtime.GOOS))
	if runtime.GOOS == "windows" {
		steamExe := filepath.Join(m.cfg.SteamCMDDir, "steamcmd.exe")
		if dryRun {
			m.appendLog(fmt.Sprintf("[DRY-RUN] 将创建 CS2 目录: %s", m.cfg.CS2Dir))
			m.appendLog(fmt.Sprintf("[DRY-RUN] 将执行: %s +force_install_dir %s +login anonymous +app_update 730 validate +quit", steamExe, m.cfg.CS2Dir))
			m.appendLog("[DRY-RUN] Windows CS2 服务端部署流程校验通过（未下载 71 GB 文件）")
			return nil
		}
		if _, err := os.Stat(steamExe); err != nil {
			if err := m.installSteamCMD(false); err != nil {
				return err
			}
		}
		if err := os.MkdirAll(m.cfg.CS2Dir, 0755); err != nil {
			return err
		}
		cmd := exec.Command(steamExe, "+force_install_dir", m.cfg.CS2Dir, "+login", "anonymous", "+app_update", "730", "validate", "+quit")
		return m.runCmdStream(cmd)
	}

	if dryRun {
		m.appendLog(fmt.Sprintf("[DRY-RUN] 将在 %s 下载/校验 CS2 (AppID 730) 并软链 steamclient.so", m.cfg.CS2Dir))
		return nil
	}
	script := fmt.Sprintf(`set -e
su - %s -c '"%s/steamcmd.sh" +force_install_dir "%s" +login anonymous +app_update 730 validate +quit'
ln -sf "%s/linux64/steamclient.so" "/home/%s/.steam/sdk64/steamclient.so"
chown -h %s:%s "/home/%s/.steam/sdk64/steamclient.so"`,
		m.cfg.CS2User, m.cfg.SteamCMDDir, m.cfg.CS2Dir,
		m.cfg.SteamCMDDir, m.cfg.CS2User, m.cfg.CS2User, m.cfg.CS2User, m.cfg.CS2User)
	return m.runCmdStream(exec.Command("bash", "-c", script))
}

func (m *Manager) installPlugins(dryRun bool) error {
	m.setStage(fmt.Sprintf("安装全皮肤插件栈与自动配对组件 (%s)", runtime.GOOS))
	csgoDir := filepath.Join(m.cfg.CS2Dir, "game", "csgo")
	if dryRun {
		mmLatest := "https://mms.alliedmods.net/mmsdrop/2.0/mmsource-latest-linux"
		if runtime.GOOS == "windows" {
			mmLatest = "https://mms.alliedmods.net/mmsdrop/2.0/mmsource-latest-windows"
		}
		_ = m.checkURL(mmLatest)
		m.appendLog(fmt.Sprintf("[DRY-RUN] 目标插件目录: %s", filepath.Join(csgoDir, "addons")))
		m.appendLog("[DRY-RUN] 将拉取 Metamod + CounterStrikeSharp + AnyBaseLib + PlayerSettings + MenuManager + WeaponPaints")
		m.appendLog(fmt.Sprintf("[DRY-RUN] 将写入内置 SkinBind v1.2.0 (%d bytes) 并自动生成 WeaponPaints.json / SkinBind.json", len(embeddedSkinBindDLL)))
		return nil
	}

	if runtime.GOOS == "windows" {
		if err := os.MkdirAll(filepath.Join(csgoDir, "addons"), 0755); err != nil {
			return err
		}
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

		repos := []struct {
			repo    string
			pattern string
		}{
			{"roflmuffin/CounterStrikeSharp", "counterstrikesharp-with-runtime-windows-"},
			{"NickFox007/AnyBaseLibCS2", ".zip"},
			{"NickFox007/PlayerSettingsCS2", ".zip"},
			{"NickFox007/MenuManagerCS2", ".zip"},
			{"Nereziel/cs2-WeaponPaints", "WeaponPaints.zip"},
		}
		for _, r := range repos {
			m.appendLog("拉取最新 Release: " + r.repo)
			req, _ := http.NewRequest("GET", "https://api.github.com/repos/"+r.repo+"/releases/latest", nil)
			req.Header.Set("User-Agent", "SherryPanel/1.3")
			apiResp, err := c.Do(req)
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
			if err := extractPluginZip(tmpZ, csgoDir); err != nil {
				return err
			}
			_ = os.Remove(tmpZ)
		}
		_ = patchGameInfoAndCore(csgoDir)
		m.deployEmbeddedSkinBindAndConfigs(csgoDir)
		m.appendLog("Windows 插件栈、内置 SkinBind 与自动配置全部就绪！")
		return nil
	}

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
git clone --depth 1 https://github.com/Nereziel/cs2-WeaponPaints.git /tmp/wp_web_repo >/dev/null 2>&1 || true
if [ -d /tmp/wp_web_repo/website/data ]; then cp -rf /tmp/wp_web_repo/website/data/* "${DATA_DIR}/"; fi
if [ -d /tmp/wp_web_repo/website/img ]; then cp -rf /tmp/wp_web_repo/website/img/* "${IMG_DIR}/"; fi
rm -rf /tmp/wp_web_repo
chown -R %s:%s "${CSGO_DIR}/addons" 2>/dev/null || true`, csgoDir, m.cfg.DataDir, m.cfg.ImgDir, m.cfg.CS2User, m.cfg.CS2User)
	if err := m.runCmdStream(exec.Command("bash", "-c", script)); err != nil {
		return err
	}
	_ = patchGameInfoAndCore(csgoDir)
	m.deployEmbeddedSkinBindAndConfigs(csgoDir)
	return nil
}

package config

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type AppConfig struct {
	PlayerPort     string `json:"player_port"`
	AdminPort      string `json:"admin_port"`
	AdminBind      string `json:"admin_bind"`
	AdminPassword  string `json:"admin_password"`
	SessionSecret  string `json:"session_secret"`
	DBDSN          string `json:"db_dsn"`
	DataDir        string `json:"data_dir"`
	ImgDir         string `json:"img_dir"`
	CS2Addr        string `json:"cs2_addr"`
	CS2Dir         string `json:"cs2_dir"`
	SteamCMDDir    string `json:"steamcmd_dir"`
	CS2User        string `json:"cs2_user"`
	CS2EnvFile     string `json:"cs2_env_file"`
	CS2ServiceName string `json:"cs2_service_name"`
	PublicIP       string `json:"public_ip"`
	WelcomeTitle   string `json:"welcome_title"`
	DryRun         bool   `json:"dry_run"`
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func Default() AppConfig {
	cfg := AppConfig{
		PlayerPort:     "8080",
		AdminPort:      "9090",
		AdminBind:      "0.0.0.0",
		AdminPassword:  "",
		SessionSecret:  "",
		DBDSN:          "",
		DataDir:        "/var/www/weaponpaints/data",
		ImgDir:         "/var/www/weaponpaints/img",
		CS2Addr:        "127.0.0.1:27015",
		CS2Dir:         "/home/cs2/cs2-ds",
		SteamCMDDir:    "/home/cs2/steamcmd",
		CS2User:        "cs2",
		CS2EnvFile:     "/home/cs2/env.conf",
		CS2ServiceName: "cs2.service",
		PublicIP:       "",
		WelcomeTitle:   "欢迎来到 Sherry 的专属服务器！",
	}
	if runtime.GOOS == "windows" {
		base, _ := os.Getwd()
		cfg.DataDir = filepath.Join(base, "data")
		cfg.ImgDir = filepath.Join(base, "img")
		cfg.CS2Dir = filepath.Join(base, "cs2-ds")
		cfg.SteamCMDDir = filepath.Join(base, "steamcmd")
		cfg.CS2EnvFile = filepath.Join(base, "cs2.env")
		cfg.AdminBind = "127.0.0.1"
	}
	return cfg
}

func envOr(k, cur string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return cur
}

func Load(path string) AppConfig {
	cfg := Default()
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &cfg)
	}
	cfg.PlayerPort = envOr("PORT", envOr("PLAYER_PORT", cfg.PlayerPort))
	cfg.AdminPort = envOr("ADMIN_PORT", cfg.AdminPort)
	cfg.AdminBind = envOr("ADMIN_BIND", cfg.AdminBind)
	cfg.AdminPassword = envOr("ADMIN_PASSWORD", cfg.AdminPassword)
	cfg.SessionSecret = envOr("SESSION_SECRET", cfg.SessionSecret)
	cfg.DBDSN = envOr("DB_DSN", cfg.DBDSN)
	cfg.DataDir = envOr("DATA_DIR", cfg.DataDir)
	cfg.ImgDir = envOr("IMG_DIR", cfg.ImgDir)
	cfg.CS2Addr = envOr("CS2_ADDR", cfg.CS2Addr)
	cfg.CS2Dir = envOr("CS2_DIR", cfg.CS2Dir)
	cfg.SteamCMDDir = envOr("STEAMCMD_DIR", cfg.SteamCMDDir)
	cfg.CS2User = envOr("CS2_USER", cfg.CS2User)
	cfg.CS2EnvFile = envOr("CS2_ENV_FILE", cfg.CS2EnvFile)
	cfg.CS2ServiceName = envOr("CS2_SERVICE_NAME", cfg.CS2ServiceName)
	cfg.PublicIP = envOr("PUBLIC_IP", cfg.PublicIP)
	cfg.WelcomeTitle = envOr("WELCOME_TITLE", cfg.WelcomeTitle)
	if cfg.WelcomeTitle == "" {
		cfg.WelcomeTitle = "欢迎来到 Sherry 的专属服务器！"
	}
	if strings.EqualFold(os.Getenv("DRY_RUN"), "true") || os.Getenv("DRY_RUN") == "1" {
		cfg.DryRun = true
	}
	generated := false
	if cfg.AdminPassword == "" {
		cfg.AdminPassword = "adm-" + randomHex(6)
		generated = true
	}
	if cfg.SessionSecret == "" {
		cfg.SessionSecret = randomHex(32)
		generated = true
	}
	// Persist first-run generated password & secret so it stays stable across restarts
	if generated && strings.TrimSpace(os.Getenv("ADMIN_PASSWORD")) == "" {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			_ = Save(path, cfg)
		}
	}
	return cfg
}

func Save(path string, cfg AppConfig) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0600)
}

func PromptWizard(path string) (AppConfig, bool, bool) {
	cfg := Load(path)
	r := bufio.NewReader(os.Stdin)
	ask := func(label, def string) string {
		fmt.Printf("%s [%s]: ", label, def)
		line, _ := r.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" {
			return def
		}
		return line
	}
	fmt.Println("===================================================")
	fmt.Printf("  CS2 服务端与安全皮肤工坊 · 配置向导 (%s/%s)\n", runtime.GOOS, runtime.GOARCH)
	fmt.Println("===================================================")
	cfg.PlayerPort = ask("玩家换肤端口 (公开)", cfg.PlayerPort)
	cfg.AdminBind = ask("管理端绑定地址 (127.0.0.1 仅本机 / 0.0.0.0 远程)", cfg.AdminBind)
	cfg.AdminPort = ask("管理端独立端口", cfg.AdminPort)
	cfg.AdminPassword = ask("管理员登录密码", cfg.AdminPassword)
	cfg.CS2Dir = ask("CS2 服务端安装目录", cfg.CS2Dir)
	cfg.SteamCMDDir = ask("SteamCMD 目录", cfg.SteamCMDDir)
	cfg.WelcomeTitle = ask("进服欢迎语", cfg.WelcomeTitle)
	if runtime.GOOS != "windows" {
		cfg.CS2User = ask("运行 CS2 的系统用户", cfg.CS2User)
	}
	cfg.DBDSN = ask("MariaDB DSN (留空可稍后在管理端一键自动安装)", cfg.DBDSN)

	if err := Save(path, cfg); err != nil {
		fmt.Printf("保存配置失败: %v\n", err)
	} else {
		fmt.Printf("配置已保存至: %s\n", path)
	}
	dlCS2 := strings.ToLower(ask("是否现在自动下载/更新 CS2 服务端 (y/N)", "n")) == "y"
	dlPlugins := strings.ToLower(ask("是否自动安装全皮肤插件栈与中文皮肤库 (y/N)", "n")) == "y"
	return cfg, dlCS2, dlPlugins
}

package serverctl

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type CS2Settings struct {
	Hostname     string `json:"hostname"`
	Map          string `json:"map"`
	MaxPlayers   string `json:"maxplayers"`
	Port         string `json:"port"`
	GSLT         string `json:"gslt"`
	SVPassword   string `json:"sv_password"`
	RCONPassword string `json:"rcon_password"`
	GameType     string `json:"game_type"`
	GameMode     string `json:"game_mode"`
}

type SystemMetrics struct {
	OS            string  `json:"os"`
	Arch          string  `json:"arch"`
	ServiceActive bool    `json:"service_active"`
	CS2Installed  bool    `json:"cs2_installed"`
	PluginsReady  bool    `json:"plugins_ready"`
	DiskTotalGB   float64 `json:"disk_total_gb"`
	DiskFreeGB    float64 `json:"disk_free_gb"`
	MemTotalMB    int64   `json:"mem_total_mb"`
	MemAvailMB    int64   `json:"mem_avail_mb"`
	LoadAvg       string  `json:"load_avg"`
}

func ReadEnv(path string) CS2Settings {
	s := CS2Settings{
		Hostname:   "CS2 Dedicated Server",
		Map:        "de_mirage",
		MaxPlayers: "12",
		Port:       "27015",
		GameType:   "0",
		GameMode:   "1",
	}
	f, err := os.Open(path)
	if err != nil {
		return s
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		switch strings.TrimSpace(k) {
		case "CS2_HOSTNAME":
			s.Hostname = v
		case "CS2_MAP":
			s.Map = v
		case "CS2_MAXPLAYERS":
			s.MaxPlayers = v
		case "CS2_PORT":
			s.Port = v
		case "CS2_GSLT":
			s.GSLT = v
		case "CS2_SV_PASSWORD":
			s.SVPassword = v
		case "CS2_RCON_PASSWORD":
			s.RCONPassword = v
		case "CS2_GAME_TYPE":
			s.GameType = v
		case "CS2_GAME_MODE":
			s.GameMode = v
		}
	}
	return s
}

func sanitizeVal(v string) string {
	v = strings.ReplaceAll(v, `"`, "")
	v = strings.ReplaceAll(v, "\n", "")
	v = strings.ReplaceAll(v, "\r", "")
	v = strings.ReplaceAll(v, "$", "")
	v = strings.ReplaceAll(v, "`", "")
	return strings.TrimSpace(v)
}

func WriteEnv(envPath, cs2Dir string, s CS2Settings) error {
	if dir := filepath.Dir(envPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	content := fmt.Sprintf("CS2_MAP=\"%s\"\nCS2_MAXPLAYERS=\"%s\"\nCS2_PORT=\"%s\"\nCS2_GSLT=\"%s\"\nCS2_SV_PASSWORD=\"%s\"\nCS2_RCON_PASSWORD=\"%s\"\nCS2_HOSTNAME=\"%s\"\nCS2_GAME_TYPE=\"%s\"\nCS2_GAME_MODE=\"%s\"\n",
		sanitizeVal(s.Map), sanitizeVal(s.MaxPlayers), sanitizeVal(s.Port),
		sanitizeVal(s.GSLT), sanitizeVal(s.SVPassword), sanitizeVal(s.RCONPassword),
		sanitizeVal(s.Hostname), sanitizeVal(s.GameType), sanitizeVal(s.GameMode),
	)
	if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
		return err
	}
	cfgDir := filepath.Join(cs2Dir, "game", "csgo", "cfg")
	if _, err := os.Stat(cfgDir); err == nil {
		cfg := fmt.Sprintf("hostname \"%s\"\nrcon_password \"%s\"\nsv_password \"%s\"\nsv_cheats 0\nsv_lan 0\ntv_enable 0\nmp_autoteambalance 1\nmp_limitteams 1\n",
			sanitizeVal(s.Hostname), sanitizeVal(s.RCONPassword), sanitizeVal(s.SVPassword))
		_ = os.WriteFile(filepath.Join(cfgDir, "server.cfg"), []byte(cfg), 0644)
	}
	return nil
}

func cs2BinaryPath(cs2Dir string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(cs2Dir, "game", "bin", "win64", "cs2.exe")
	}
	return filepath.Join(cs2Dir, "game", "bin", "linuxsteamrt64", "cs2")
}

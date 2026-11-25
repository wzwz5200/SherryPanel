package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"cs2-skins-web/internal/a2s"
	"cs2-skins-web/internal/auth"
	"cs2-skins-web/internal/catalog"
	"cs2-skins-web/internal/config"
	"cs2-skins-web/internal/db"
	"cs2-skins-web/internal/installer"
	"cs2-skins-web/internal/rcon"
	"cs2-skins-web/internal/serverctl"
)

var steamIDRe = regexp.MustCompile(`^7656119[0-9]{10}$`)

type Server struct {
	mu          sync.RWMutex
	Cfg         config.AppConfig
	CfgPath     string
	Store       *db.Store
	Auth        *auth.Manager
	Catalog     *catalog.Catalog
	CatalogJSON []byte
	Installer   *installer.Manager
	PublicIP    string
	refreshCool map[string]time.Time
}

func (s *Server) getStore() *db.Store {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Store
}

func (s *Server) getCfg() config.AppConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Cfg
}

// OnDatabaseReady hot-attaches a newly initialized MariaDB connection without restarting.
func (s *Server) OnDatabaseReady(dsn string) {
	st, err := db.Open(dsn)
	if err != nil {
		return
	}
	_ = st.EnsureSchema()

	s.mu.Lock()
	old := s.Store
	s.Store = st
	s.Cfg.DBDSN = dsn
	cfgCopy := s.Cfg
	cfgPath := s.CfgPath
	s.mu.Unlock()

	if old != nil && old.SQL != nil && old.SQL != st.SQL {
		_ = old.SQL.Close()
	}
	s.Auth.SetDB(st.SQL)
	if cfgPath != "" {
		_ = config.Save(cfgPath, cfgCopy)
	}
	if s.Installer != nil {
		s.Installer.UpdateConfig(cfgCopy)
	}
	wpDir := filepath.Join(cfgCopy.CS2Dir, "game", "csgo", "addons", "counterstrikesharp", "plugins", "WeaponPaints")
	if _, err := os.Stat(wpDir); err == nil {
		host := s.PublicIP
		if host == "" {
			host = "127.0.0.1"
		}
		_ = installer.WriteWeaponPaintsConfig(cfgCopy.CS2Dir, cfgCopy, fmt.Sprintf("http://%s:%s", host, cfgCopy.PlayerPort))
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]any{"error": msg})
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func validSteamID(id string) string {
	id = strings.TrimSpace(id)
	if !steamIDRe.MatchString(id) {
		return ""
	}
	return id
}

// =========================================================================
// PLAYER SERVER ROUTES (Public Port :8080 — ZERO Admin/RCON/Secret Routes!)
// =========================================================================
func (s *Server) RegisterPlayer(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/meta", s.handlePlayerMeta)
	mux.HandleFunc("GET /api/catalog", s.handleCatalog)
	mux.HandleFunc("POST /api/auth/redeem", s.handlePlayerRedeem)
	mux.HandleFunc("POST /api/auth/logout", s.handlePlayerLogout)

	mux.HandleFunc("GET /api/loadout", s.handleGetLoadout)
	mux.HandleFunc("PUT /api/loadout/skin", s.handleSetSkin)
	mux.HandleFunc("PUT /api/loadout/knife", s.handleSetKnife)
	mux.HandleFunc("PUT /api/loadout/glove", s.handleSetGlove)
	mux.HandleFunc("PUT /api/loadout/agent", s.handleSetAgent)
	mux.HandleFunc("PUT /api/loadout/music", s.handleSetMusic)
	mux.HandleFunc("PUT /api/loadout/pin", s.handleSetPin)
	mux.HandleFunc("DELETE /api/loadout", s.handleResetLoadout)
	mux.HandleFunc("POST /api/loadout/refresh", s.handleRefreshLoadout)
}

func (s *Server) handlePlayerMeta(w http.ResponseWriter, r *http.Request) {
	cfg := s.getCfg()
	st := s.getStore()
	srvStatus := a2s.Query(cfg.CS2Addr)
	metrics := serverctl.Metrics(cfg.CS2ServiceName, cfg.CS2Dir)
	sid := s.Auth.GetPlayerSteamID(r)
	writeJSON(w, 200, map[string]any{
		"mode":          "player",
		"authenticated": sid != "",
		"steamid":       sid,
		"db_connected":  st != nil,
		"cs2_installed": metrics.CS2Installed,
		"plugins_ready": metrics.PluginsReady,
		"admin_port":    cfg.AdminPort,
		"auto_refresh":  true,
		"public_ip":     s.PublicIP,
		"server":        srvStatus,
	})
}

func (s *Server) handleCatalog(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(200)
	if len(s.CatalogJSON) == 0 {
		_, _ = w.Write([]byte(`{"weapons":[],"knives":[],"gloves":[],"agents":[],"music":[],"pins":[]}`))
		return
	}
	_, _ = w.Write(s.CatalogJSON)
}

func (s *Server) handlePlayerRedeem(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if !s.Auth.CheckRateLimit(ip) {
		writeErr(w, 429, "尝试次数过多，请 5 分钟后再试")
		return
	}
	var req struct {
		Code  string `json:"code"`
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, "请求格式错误")
		return
	}
	sid, err := s.Auth.RedeemCodeOrToken(req.Code, req.Token)
	if err != nil {
		s.Auth.RecordFailure(ip)
		writeErr(w, 401, err.Error())
		return
	}
	s.Auth.SetPlayerCookie(w, sid)
	writeJSON(w, 200, map[string]any{"ok": true, "steamid": sid})
}

func (s *Server) handlePlayerLogout(w http.ResponseWriter, r *http.Request) {
	s.Auth.ClearPlayerCookie(w)
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) requirePlayer(w http.ResponseWriter, r *http.Request) (string, *db.Store) {
	sid := s.Auth.GetPlayerSteamID(r)
	if sid == "" {
		writeErr(w, 401, "未验证玩家身份：请在游戏内输入 !bind 获取 6 位配对码，或通过房主发放的专属链接访问")
		return "", nil
	}
	st := s.getStore()
	if st == nil {
		writeErr(w, 503, "数据库未初始化，请先打开独立管理端「新手引导」一键全自动部署")
		return "", nil
	}
	return sid, st
}

func (s *Server) autoRefreshPlayer(steamID string) (bool, string) {
	if validSteamID(steamID) == "" {
		return false, "无效的 SteamID"
	}
	cfg := s.getCfg()
	env := serverctl.ReadEnv(cfg.CS2EnvFile)
	if strings.TrimSpace(env.RCONPassword) == "" {
		return false, "已保存皮肤配置，进服后自动生效"
	}
	cmd := fmt.Sprintf("wp_refresh %s; css_skinbind_synced %s", steamID, steamID)
	if _, err := s.execRCONFast(cmd); err != nil {
		return false, "已保存皮肤配置，进服后自动生效"
	}
	return true, "已保存并自动刷新游戏内皮肤（无需输入 !wp）"
}

func (s *Server) handleGetLoadout(w http.ResponseWriter, r *http.Request) {
	sid, st := s.requirePlayer(w, r)
	if sid == "" {
		return
	}
	lo, err := st.GetLoadout(sid)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, lo)
}

func (s *Server) handleSetSkin(w http.ResponseWriter, r *http.Request) {
	sid, st := s.requirePlayer(w, r)
	if sid == "" {
		return
	}
	var req struct {
		Team          int     `json:"team"`
		DefIndex      int     `json:"weapon_defindex"`
		PaintID       int     `json:"weapon_paint_id"`
		Wear          float64 `json:"weapon_wear"`
		Seed          int     `json:"weapon_seed"`
		Nametag       string  `json:"weapon_nametag"`
		StatTrak      bool    `json:"weapon_stattrak"`
		StatTrakCount int     `json:"weapon_stattrak_count"`
		EquipKnife    bool    `json:"equip_knife"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, "请求格式错误")
		return
	}
	if req.EquipKnife && s.Catalog != nil {
		for _, k := range s.Catalog.Knives {
			if k.DefIndex == req.DefIndex {
				if err := st.SetKnife(sid, req.Team, k.WeaponName); err != nil {
					writeErr(w, 500, err.Error())
					return
				}
				break
			}
		}
	}
	err := st.SetSkin(sid, req.Team, db.SkinRecord{
		DefIndex:      req.DefIndex,
		PaintID:       req.PaintID,
		Wear:          req.Wear,
		Seed:          req.Seed,
		Nametag:       strings.TrimSpace(req.Nametag),
		StatTrak:      req.StatTrak,
		StatTrakCount: req.StatTrakCount,
	})
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	lo, _ := st.GetLoadout(sid)
	refreshed, msg := s.autoRefreshPlayer(sid)
	writeJSON(w, 200, map[string]any{"ok": true, "auto_refreshed": refreshed, "message": msg, "loadout": lo})
}

func (s *Server) handleSetKnife(w http.ResponseWriter, r *http.Request) {
	sid, st := s.requirePlayer(w, r)
	if sid == "" {
		return
	}
	var req struct {
		Team  int    `json:"team"`
		Knife string `json:"knife"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, "请求格式错误")
		return
	}
	if err := st.SetKnife(sid, req.Team, strings.TrimSpace(req.Knife)); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	lo, _ := st.GetLoadout(sid)
	refreshed, msg := s.autoRefreshPlayer(sid)
	writeJSON(w, 200, map[string]any{"ok": true, "auto_refreshed": refreshed, "message": msg, "loadout": lo})
}

func (s *Server) handleSetGlove(w http.ResponseWriter, r *http.Request) {
	sid, st := s.requirePlayer(w, r)
	if sid == "" {
		return
	}
	var req struct {
		Team     int     `json:"team"`
		DefIndex int     `json:"weapon_defindex"`
		PaintID  int     `json:"weapon_paint_id"`
		Wear     float64 `json:"weapon_wear"`
		Seed     int     `json:"weapon_seed"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, "请求格式错误")
		return
	}
	if err := st.SetGlove(sid, req.Team, req.DefIndex, req.PaintID, req.Wear, req.Seed); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	lo, _ := st.GetLoadout(sid)
	refreshed, msg := s.autoRefreshPlayer(sid)
	writeJSON(w, 200, map[string]any{"ok": true, "auto_refreshed": refreshed, "message": msg, "loadout": lo})
}

func (s *Server) handleSetAgent(w http.ResponseWriter, r *http.Request) {
	sid, st := s.requirePlayer(w, r)
	if sid == "" {
		return
	}
	var req struct {
		Team  int    `json:"team"`
		Model string `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, "请求格式错误")
		return
	}
	if err := st.SetAgent(sid, req.Team, strings.TrimSpace(req.Model)); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	lo, _ := st.GetLoadout(sid)
	refreshed, msg := s.autoRefreshPlayer(sid)
	writeJSON(w, 200, map[string]any{"ok": true, "auto_refreshed": refreshed, "message": msg, "loadout": lo})
}

func (s *Server) handleSetMusic(w http.ResponseWriter, r *http.Request) {
	sid, st := s.requirePlayer(w, r)
	if sid == "" {
		return
	}
	var req struct {
		Team    int `json:"team"`
		MusicID int `json:"music_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, "请求格式错误")
		return
	}
	if err := st.SetMusic(sid, req.Team, req.MusicID); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	lo, _ := st.GetLoadout(sid)
	refreshed, msg := s.autoRefreshPlayer(sid)
	writeJSON(w, 200, map[string]any{"ok": true, "auto_refreshed": refreshed, "message": msg, "loadout": lo})
}

func (s *Server) handleSetPin(w http.ResponseWriter, r *http.Request) {
	sid, st := s.requirePlayer(w, r)
	if sid == "" {
		return
	}
	var req struct {
		Team  int `json:"team"`
		PinID int `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, "请求格式错误")
		return
	}
	if err := st.SetPin(sid, req.Team, req.PinID); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	lo, _ := st.GetLoadout(sid)
	refreshed, msg := s.autoRefreshPlayer(sid)
	writeJSON(w, 200, map[string]any{"ok": true, "auto_refreshed": refreshed, "message": msg, "loadout": lo})
}

func (s *Server) handleResetLoadout(w http.ResponseWriter, r *http.Request) {
	sid, st := s.requirePlayer(w, r)
	if sid == "" {
		return
	}
	if err := st.ResetLoadout(sid); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	lo, _ := st.GetLoadout(sid)
	refreshed, msg := s.autoRefreshPlayer(sid)
	writeJSON(w, 200, map[string]any{"ok": true, "auto_refreshed": refreshed, "message": msg, "loadout": lo})
}

func (s *Server) handleRefreshLoadout(w http.ResponseWriter, r *http.Request) {
	sid, _ := s.requirePlayer(w, r)
	if sid == "" {
		return
	}
	s.mu.Lock()
	if s.refreshCool == nil {
		s.refreshCool = map[string]time.Time{}
	}
	if last, ok := s.refreshCool[sid]; ok && time.Since(last) < 2*time.Second {
		s.mu.Unlock()
		writeErr(w, 429, "刷新过于频繁，请 2 秒后再试")
		return
	}
	s.refreshCool[sid] = time.Now()
	s.mu.Unlock()

	refreshed, msg := s.autoRefreshPlayer(sid)
	writeJSON(w, 200, map[string]any{
		"ok":             true,
		"auto_refreshed": refreshed,
		"message":        msg,
	})
}

// =========================================================================
// ADMIN SERVER ROUTES (Isolated Port :9090 — Strict Admin Password Auth!)
// =========================================================================
func (s *Server) RegisterAdmin(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/meta", s.handleAdminMeta)
	mux.HandleFunc("POST /api/admin/login", s.handleAdminLogin)
	mux.HandleFunc("POST /api/admin/logout", s.handleAdminLogout)

	mux.HandleFunc("GET /api/setup/state", s.requireAdmin(s.handleSetupState))
	mux.HandleFunc("POST /api/setup/config", s.requireAdmin(s.handleSetupConfig))
	mux.HandleFunc("GET /api/server/status", s.requireAdmin(s.handleServerStatus))
	mux.HandleFunc("PUT /api/server/settings", s.requireAdmin(s.handleSaveSettings))
	mux.HandleFunc("POST /api/server/action", s.requireAdmin(s.handleServerAction))
	mux.HandleFunc("POST /api/server/rcon", s.requireAdmin(s.handleRCON))
	mux.HandleFunc("POST /api/server/preset", s.requireAdmin(s.handlePreset))
	mux.HandleFunc("GET /api/server/cvars", s.requireAdmin(s.handleGetCVars))
	mux.HandleFunc("POST /api/server/cvar", s.requireAdmin(s.handleSetCVars))
	mux.HandleFunc("GET /api/installer/status", s.requireAdmin(s.handleInstallerStatus))
	mux.HandleFunc("POST /api/installer/run", s.requireAdmin(s.handleInstallerRun))
	mux.HandleFunc("GET /api/admin/players", s.requireAdmin(s.handleAdminPlayers))
	mux.HandleFunc("POST /api/admin/pairing", s.requireAdmin(s.handleAdminCreatePairing))
}

func (s *Server) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.Auth.IsAdminAuthed(r) {
			writeErr(w, 401, "需要管理员登录验证")
			return
		}
		next(w, r)
	}
}

func (s *Server) handleAdminMeta(w http.ResponseWriter, r *http.Request) {
	cfg := s.getCfg()
	authed := s.Auth.IsAdminAuthed(r)
	resp := map[string]any{
		"mode":                "admin",
		"admin_authenticated": authed,
		"os":                  runtime.GOOS,
		"arch":                runtime.GOARCH,
		"dry_run":             cfg.DryRun,
		"player_port":         cfg.PlayerPort,
		"public_ip":           s.PublicIP,
	}
	if authed {
		env := serverctl.ReadEnv(cfg.CS2EnvFile)
		port := env.Port
		if port == "" {
			port = "27015"
		}
		host := s.PublicIP
		if host == "" {
			host = "127.0.0.1"
		}
		cmd := "connect " + host + ":" + port
		if env.SVPassword != "" {
			cmd += "; password " + env.SVPassword
		}
		resp["connect_cmd"] = cmd
		resp["server"] = a2s.Query(cfg.CS2Addr)
	}
	writeJSON(w, 200, resp)
}

func (s *Server) handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if !s.Auth.CheckRateLimit(ip) {
		writeErr(w, 429, "密码错误次数过多，请 5 分钟后再试")
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, "请求格式错误")
		return
	}
	if !s.Auth.VerifyAdminPassword(req.Password) {
		s.Auth.RecordFailure(ip)
		writeErr(w, 401, "管理员密码错误")
		return
	}
	s.Auth.SetAdminCookie(w)
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) handleAdminLogout(w http.ResponseWriter, r *http.Request) {
	s.Auth.ClearAdminCookie(w)
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) buildSetupState() map[string]any {
	cfg := s.getCfg()
	st := s.getStore()
	metrics := serverctl.Metrics(cfg.CS2ServiceName, cfg.CS2Dir)
	env := serverctl.ReadEnv(cfg.CS2EnvFile)

	steamBin := filepath.Join(cfg.SteamCMDDir, "steamcmd.sh")
	if runtime.GOOS == "windows" {
		steamBin = filepath.Join(cfg.SteamCMDDir, "steamcmd.exe")
	}
	_, errSteam := os.Stat(steamBin)

	sbDll := filepath.Join(cfg.CS2Dir, "game", "csgo", "addons", "counterstrikesharp", "plugins", "SkinBind", "SkinBind.dll")
	_, errSB := os.Stat(sbDll)

	host := s.PublicIP
	if host == "" {
		host = "127.0.0.1"
	}
	return map[string]any{
		"ready":          metrics.CS2Installed && metrics.PluginsReady && st != nil,
		"steamcmd_ready": errSteam == nil,
		"cs2_installed":  metrics.CS2Installed,
		"plugins_ready":  metrics.PluginsReady,
		"skinbind_ready": errSB == nil,
		"db_connected":   st != nil,
		"server_running": metrics.ServiceActive,
		"rcon_ready":     strings.TrimSpace(env.RCONPassword) != "",
		"disk_free_gb":   metrics.DiskFreeGB,
		"disk_total_gb":  metrics.DiskTotalGB,
		"cs2_dir":        cfg.CS2Dir,
		"steamcmd_dir":   cfg.SteamCMDDir,
		"welcome_title":  cfg.WelcomeTitle,
		"player_url":     fmt.Sprintf("http://%s:%s", host, cfg.PlayerPort),
		"os":             runtime.GOOS,
		"arch":           runtime.GOARCH,
		"dry_run":        cfg.DryRun,
		"settings":       env,
	}
}

func (s *Server) handleSetupState(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.buildSetupState())
}

func randomRCONPass() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return "rcon-" + hex.EncodeToString(b)
}

func (s *Server) handleSetupConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CS2Dir       string                 `json:"cs2_dir"`
		SteamCMDDir  string                 `json:"steamcmd_dir"`
		WelcomeTitle string                 `json:"welcome_title"`
		Settings     *serverctl.CS2Settings `json:"settings"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, "请求格式错误")
		return
	}

	s.mu.Lock()
	if strings.TrimSpace(req.CS2Dir) != "" {
		s.Cfg.CS2Dir = strings.TrimSpace(req.CS2Dir)
	}
	if strings.TrimSpace(req.SteamCMDDir) != "" {
		s.Cfg.SteamCMDDir = strings.TrimSpace(req.SteamCMDDir)
	}
	if strings.TrimSpace(req.WelcomeTitle) != "" {
		s.Cfg.WelcomeTitle = strings.TrimSpace(req.WelcomeTitle)
	}
	cfg := s.Cfg
	cfgPath := s.CfgPath
	s.mu.Unlock()

	if cfgPath != "" {
		if err := config.Save(cfgPath, cfg); err != nil {
			writeErr(w, 500, "保存 panel-config.json 失败: "+err.Error())
			return
		}
	}

	curEnv := serverctl.ReadEnv(cfg.CS2EnvFile)
	if req.Settings != nil {
		if strings.TrimSpace(req.Settings.Hostname) != "" {
			curEnv.Hostname = req.Settings.Hostname
		}
		if strings.TrimSpace(req.Settings.Map) != "" {
			curEnv.Map = req.Settings.Map
		}
		if strings.TrimSpace(req.Settings.MaxPlayers) != "" {
			curEnv.MaxPlayers = req.Settings.MaxPlayers
		}
		if strings.TrimSpace(req.Settings.Port) != "" {
			curEnv.Port = req.Settings.Port
		}
		curEnv.GSLT = req.Settings.GSLT
		curEnv.SVPassword = req.Settings.SVPassword
		if strings.TrimSpace(req.Settings.RCONPassword) != "" {
			curEnv.RCONPassword = req.Settings.RCONPassword
		}
	}
	if strings.TrimSpace(curEnv.RCONPassword) == "" {
		curEnv.RCONPassword = randomRCONPass()
	}
	if err := serverctl.WriteEnv(cfg.CS2EnvFile, cfg.CS2Dir, curEnv); err != nil {
		writeErr(w, 500, "保存服务端配置失败: "+err.Error())
		return
	}

	cssDir := filepath.Join(cfg.CS2Dir, "game", "csgo", "addons", "counterstrikesharp")
	if _, err := os.Stat(cssDir); err == nil {
		_ = installer.WriteSkinBindConfig(cfg.CS2Dir, cfg.WelcomeTitle)
	}

	if s.Installer != nil {
		s.Installer.UpdateConfig(cfg)
	}
	writeJSON(w, 200, map[string]any{
		"ok":    true,
		"state": s.buildSetupState(),
	})
}

func (s *Server) handleAdminPlayers(w http.ResponseWriter, r *http.Request) {
	var list []string
	if st := s.getStore(); st != nil {
		list = st.KnownSteamIDs("")
	}
	writeJSON(w, 200, map[string]any{"players": list})
}

func (s *Server) handleAdminCreatePairing(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SteamID string `json:"steamid"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, "请求格式错误")
		return
	}
	sid := validSteamID(req.SteamID)
	if sid == "" {
		writeErr(w, 400, "请输入有效的 17 位 SteamID64（以 7656119 开头）")
		return
	}
	rec, err := s.Auth.CreatePairing(sid, 24*time.Hour)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	cfg := s.getCfg()
	host := s.PublicIP
	if host == "" {
		host = "127.0.0.1"
	}
	magicURL := fmt.Sprintf("http://%s:%s/?token=%s", host, cfg.PlayerPort, rec.Token)
	writeJSON(w, 200, map[string]any{
		"ok":         true,
		"steamid":    rec.SteamID,
		"code":       rec.Code,
		"token":      rec.Token,
		"magic_url":  magicURL,
		"expires_at": rec.ExpiresAt,
	})
}

func (s *Server) handleServerStatus(w http.ResponseWriter, r *http.Request) {
	cfg := s.getCfg()
	st := a2s.Query(cfg.CS2Addr)
	metrics := serverctl.Metrics(cfg.CS2ServiceName, cfg.CS2Dir)
	settings := serverctl.ReadEnv(cfg.CS2EnvFile)
	writeJSON(w, 200, map[string]any{
		"server":   st,
		"metrics":  metrics,
		"settings": settings,
		"dry_run":  cfg.DryRun,
		"paths": map[string]string{
			"cs2_dir":      cfg.CS2Dir,
			"steamcmd_dir": cfg.SteamCMDDir,
			"env_file":     cfg.CS2EnvFile,
		},
	})
}

func (s *Server) handleSaveSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Settings serverctl.CS2Settings `json:"settings"`
		Restart  bool                  `json:"restart"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, "配置参数格式错误")
		return
	}
	cfg := s.getCfg()
	if err := serverctl.WriteEnv(cfg.CS2EnvFile, cfg.CS2Dir, req.Settings); err != nil {
		writeErr(w, 500, "写入配置失败: "+err.Error())
		return
	}
	restarted := false
	if req.Restart {
		if _, err := serverctl.ServiceAction(cfg.CS2ServiceName, cfg.CS2Dir, cfg.CS2EnvFile, "restart"); err != nil {
			writeErr(w, 500, "配置已保存，但重启 CS2 失败: "+err.Error())
			return
		}
		restarted = true
	}
	writeJSON(w, 200, map[string]any{
		"ok":        true,
		"restarted": restarted,
		"settings":  serverctl.ReadEnv(cfg.CS2EnvFile),
	})
}

func (s *Server) handleServerAction(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Action string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, "请求格式错误")
		return
	}
	cfg := s.getCfg()
	out, err := serverctl.ServiceAction(cfg.CS2ServiceName, cfg.CS2Dir, cfg.CS2EnvFile, req.Action)
	if err != nil {
		writeErr(w, 500, "执行失败: "+err.Error()+" "+out)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "output": out})
}

func (s *Server) execRCONWith(cmd string, dialTimeout, firstWait time.Duration) (string, error) {
	cfg := s.getCfg()
	env := serverctl.ReadEnv(cfg.CS2EnvFile)
	port := env.Port
	if port == "" {
		port = "27015"
	}
	addr := cfg.CS2Addr
	if addr == "" {
		addr = "127.0.0.1:" + port
	}
	out, err := rcon.ExecWithTimeout(addr, env.RCONPassword, cmd, dialTimeout, firstWait)
	if err != nil && strings.HasPrefix(addr, "127.0.0.1:") {
		if c, derr := net.Dial("udp", "8.8.8.8:80"); derr == nil {
			lip := c.LocalAddr().(*net.UDPAddr).IP.String()
			c.Close()
			if lip != "" && lip != "127.0.0.1" {
				if out2, err2 := rcon.ExecWithTimeout(lip+":"+port, env.RCONPassword, cmd, dialTimeout, firstWait); err2 == nil {
					return out2, nil
				}
			}
		}
	}
	return out, err
}

func (s *Server) execRCONSmart(cmd string) (string, error) {
	return s.execRCONWith(cmd, 3*time.Second, 2000*time.Millisecond)
}

func (s *Server) execRCONFast(cmd string) (string, error) {
	return s.execRCONWith(cmd, 1200*time.Millisecond, 350*time.Millisecond)
}

func (s *Server) handleRCON(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Command string `json:"command"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, "请求格式错误")
		return
	}
	cmd := strings.TrimSpace(req.Command)
	if cmd == "" {
		writeErr(w, 400, "指令不能为空")
		return
	}
	out, err := s.execRCONSmart(cmd)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "command": cmd, "output": out})
}

func (s *Server) handlePreset(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Preset string `json:"preset"`
		Arg    string `json:"arg"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, "请求格式错误")
		return
	}
	var cmd string
	switch req.Preset {
	case "changelevel":
		m := strings.TrimSpace(req.Arg)
		if !regexp.MustCompile(`^[a-zA-Z0-9_]+$`).MatchString(m) {
			writeErr(w, 400, "无效的地图名")
			return
		}
		cmd = "changelevel " + m
	case "practice":
		cmd = "sv_cheats 1; bot_kick; mp_limitteams 0; mp_autoteambalance 0; mp_roundtime 60; mp_roundtime_defuse 60; mp_maxmoney 60000; mp_startmoney 60000; mp_buytime 9999; mp_buy_anywhere 1; sv_infinite_ammo 1; ammo_grenade_limit_total 5; sv_grenade_trajectory_prac_pipreview 1; sv_showimpacts 1; mp_warmup_end; mp_restartgame 1"
	case "competitive":
		cmd = "sv_cheats 0; sv_infinite_ammo 0; sv_showimpacts 0; sv_grenade_trajectory_prac_pipreview 0; mp_buy_anywhere 0; mp_buytime 20; mp_roundtime_defuse 1.92; mp_startmoney 800; mp_maxmoney 16000; mp_autoteambalance 1; mp_limitteams 1; mp_restartgame 1"
	case "restart_match":
		cmd = "mp_restartgame 1"
	case "end_warmup":
		cmd = "mp_warmup_end"
	case "kick_bots":
		cmd = "bot_kick"
	case "add_ct":
		cmd = "bot_add_ct"
	case "add_t":
		cmd = "bot_add_t"
	case "pause":
		cmd = "mp_pause_match"
	case "unpause":
		cmd = "mp_unpause_match"
	case "status":
		cmd = "status"
	default:
		writeErr(w, 400, "未知快捷预设: "+req.Preset)
		return
	}
	out, err := s.execRCONSmart(cmd)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "output": out})
}

func (s *Server) handleInstallerStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.Installer.GetStatus())
}

func (s *Server) handleInstallerRun(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Task   string `json:"task"`
		DryRun *bool  `json:"dry_run"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, "请求格式错误")
		return
	}
	if err := s.Installer.StartTask(req.Task, req.DryRun); err != nil {
		writeErr(w, 409, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "task": req.Task})
}

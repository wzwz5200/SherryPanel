package api

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"runtime"
	"strings"
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
	Cfg         config.AppConfig
	Store       *db.Store
	Auth        *auth.Manager
	Catalog     *catalog.Catalog
	CatalogJSON []byte
	Installer   *installer.Manager
	PublicIP    string
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
}

func (s *Server) handlePlayerMeta(w http.ResponseWriter, r *http.Request) {
	st := a2s.Query(s.Cfg.CS2Addr)
	sid := s.Auth.GetPlayerSteamID(r)
	writeJSON(w, 200, map[string]any{
		"mode":          "player",
		"authenticated": sid != "",
		"steamid":       sid,
		"db_connected":  s.Store != nil,
		"public_ip":     s.PublicIP,
		"server":        st,
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

func (s *Server) requirePlayer(w http.ResponseWriter, r *http.Request) string {
	sid := s.Auth.GetPlayerSteamID(r)
	if sid == "" {
		writeErr(w, 401, "未验证玩家身份：请在游戏内输入 !bind 获取 6 位配对码，或通过房主发放的专属链接访问")
		return ""
	}
	if s.Store == nil {
		writeErr(w, 503, "数据库未连接，请先由管理员配置 MariaDB")
		return ""
	}
	return sid
}

func (s *Server) handleGetLoadout(w http.ResponseWriter, r *http.Request) {
	sid := s.requirePlayer(w, r)
	if sid == "" {
		return
	}
	lo, err := s.Store.GetLoadout(sid)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, lo)
}

func (s *Server) handleSetSkin(w http.ResponseWriter, r *http.Request) {
	sid := s.requirePlayer(w, r)
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
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, "请求格式错误")
		return
	}
	err := s.Store.SetSkin(sid, req.Team, db.SkinRecord{
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
	lo, _ := s.Store.GetLoadout(sid)
	writeJSON(w, 200, map[string]any{"ok": true, "loadout": lo})
}

func (s *Server) handleSetKnife(w http.ResponseWriter, r *http.Request) {
	sid := s.requirePlayer(w, r)
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
	if err := s.Store.SetKnife(sid, req.Team, strings.TrimSpace(req.Knife)); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	lo, _ := s.Store.GetLoadout(sid)
	writeJSON(w, 200, map[string]any{"ok": true, "loadout": lo})
}

func (s *Server) handleSetGlove(w http.ResponseWriter, r *http.Request) {
	sid := s.requirePlayer(w, r)
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
	if err := s.Store.SetGlove(sid, req.Team, req.DefIndex, req.PaintID, req.Wear, req.Seed); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	lo, _ := s.Store.GetLoadout(sid)
	writeJSON(w, 200, map[string]any{"ok": true, "loadout": lo})
}

func (s *Server) handleSetAgent(w http.ResponseWriter, r *http.Request) {
	sid := s.requirePlayer(w, r)
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
	if err := s.Store.SetAgent(sid, req.Team, strings.TrimSpace(req.Model)); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	lo, _ := s.Store.GetLoadout(sid)
	writeJSON(w, 200, map[string]any{"ok": true, "loadout": lo})
}

func (s *Server) handleSetMusic(w http.ResponseWriter, r *http.Request) {
	sid := s.requirePlayer(w, r)
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
	if err := s.Store.SetMusic(sid, req.Team, req.MusicID); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	lo, _ := s.Store.GetLoadout(sid)
	writeJSON(w, 200, map[string]any{"ok": true, "loadout": lo})
}

func (s *Server) handleSetPin(w http.ResponseWriter, r *http.Request) {
	sid := s.requirePlayer(w, r)
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
	if err := s.Store.SetPin(sid, req.Team, req.PinID); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	lo, _ := s.Store.GetLoadout(sid)
	writeJSON(w, 200, map[string]any{"ok": true, "loadout": lo})
}

func (s *Server) handleResetLoadout(w http.ResponseWriter, r *http.Request) {
	sid := s.requirePlayer(w, r)
	if sid == "" {
		return
	}
	if err := s.Store.ResetLoadout(sid); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	lo, _ := s.Store.GetLoadout(sid)
	writeJSON(w, 200, map[string]any{"ok": true, "loadout": lo})
}

// =========================================================================
// ADMIN SERVER ROUTES (Isolated Port :9090 — Strict Admin Password Auth!)
// =========================================================================
func (s *Server) RegisterAdmin(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/meta", s.handleAdminMeta)
	mux.HandleFunc("POST /api/admin/login", s.handleAdminLogin)
	mux.HandleFunc("POST /api/admin/logout", s.handleAdminLogout)

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
	authed := s.Auth.IsAdminAuthed(r)
	resp := map[string]any{
		"mode":                "admin",
		"admin_authenticated": authed,
		"os":                  runtime.GOOS,
		"arch":                runtime.GOARCH,
		"dry_run":             s.Cfg.DryRun,
		"player_port":         s.Cfg.PlayerPort,
		"public_ip":           s.PublicIP,
	}
	if authed {
		env := serverctl.ReadEnv(s.Cfg.CS2EnvFile)
		port := env.Port
		if port == "" {
			port = "27015"
		}
		cmd := "connect " + s.PublicIP + ":" + port
		if env.SVPassword != "" {
			cmd += "; password " + env.SVPassword
		}
		resp["connect_cmd"] = cmd
		resp["server"] = a2s.Query(s.Cfg.CS2Addr)
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

func (s *Server) handleAdminPlayers(w http.ResponseWriter, r *http.Request) {
	var list []string
	if s.Store != nil {
		list = s.Store.KnownSteamIDs("")
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
	host := s.PublicIP
	if host == "" {
		host = "127.0.0.1"
	}
	magicURL := fmt.Sprintf("http://%s:%s/?token=%s", host, s.Cfg.PlayerPort, rec.Token)
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
	st := a2s.Query(s.Cfg.CS2Addr)
	metrics := serverctl.Metrics(s.Cfg.CS2ServiceName, s.Cfg.CS2Dir)
	settings := serverctl.ReadEnv(s.Cfg.CS2EnvFile)
	writeJSON(w, 200, map[string]any{
		"server":   st,
		"metrics":  metrics,
		"settings": settings,
		"dry_run":  s.Cfg.DryRun,
		"paths": map[string]string{
			"cs2_dir":      s.Cfg.CS2Dir,
			"steamcmd_dir": s.Cfg.SteamCMDDir,
			"env_file":     s.Cfg.CS2EnvFile,
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
	if err := serverctl.WriteEnv(s.Cfg.CS2EnvFile, s.Cfg.CS2Dir, req.Settings); err != nil {
		writeErr(w, 500, "写入配置失败: "+err.Error())
		return
	}
	restarted := false
	if req.Restart {
		if _, err := serverctl.ServiceAction(s.Cfg.CS2ServiceName, s.Cfg.CS2Dir, s.Cfg.CS2EnvFile, "restart"); err != nil {
			writeErr(w, 500, "配置已保存，但重启 CS2 失败: "+err.Error())
			return
		}
		restarted = true
	}
	writeJSON(w, 200, map[string]any{
		"ok":        true,
		"restarted": restarted,
		"settings":  serverctl.ReadEnv(s.Cfg.CS2EnvFile),
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
	out, err := serverctl.ServiceAction(s.Cfg.CS2ServiceName, s.Cfg.CS2Dir, s.Cfg.CS2EnvFile, req.Action)
	if err != nil {
		writeErr(w, 500, "执行失败: "+err.Error()+" "+out)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "output": out})
}

func (s *Server) execRCONSmart(cmd string) (string, error) {
	env := serverctl.ReadEnv(s.Cfg.CS2EnvFile)
	port := env.Port
	if port == "" {
		port = "27015"
	}
	addr := s.Cfg.CS2Addr
	if addr == "" {
		addr = "127.0.0.1:" + port
	}
	out, err := rcon.Exec(addr, env.RCONPassword, cmd)
	if err != nil && strings.HasPrefix(addr, "127.0.0.1:") {
		if c, derr := net.Dial("udp", "8.8.8.8:80"); derr == nil {
			lip := c.LocalAddr().(*net.UDPAddr).IP.String()
			c.Close()
			if lip != "" && lip != "127.0.0.1" {
				if out2, err2 := rcon.Exec(lip+":"+port, env.RCONPassword, cmd); err2 == nil {
					return out2, nil
				}
			}
		}
	}
	return out, err
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

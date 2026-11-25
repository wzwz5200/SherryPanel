package api

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
)

var allowedCVars = map[string]bool{
	"sv_cheats":                            true,
	"sv_showimpacts":                       true,
	"sv_infinite_ammo":                     true,
	"sv_grenade_trajectory_prac_pipreview": true,
	"sv_grenade_trajectory_prac_trailtime": true,
	"sv_regeneration_force_on":             true,
	"ammo_grenade_limit_total":             true,
	"mp_buy_anywhere":                      true,
	"mp_buytime":                           true,
	"mp_roundtime":                         true,
	"mp_roundtime_defuse":                  true,
	"mp_startmoney":                        true,
	"mp_maxmoney":                          true,
	"mp_freezetime":                        true,
	"mp_friendlyfire":                      true,
	"mp_autoteambalance":                   true,
	"mp_limitteams":                        true,
	"mp_respawn_on_death_ct":               true,
	"mp_respawn_on_death_t":                true,
	"bot_difficulty":                       true,
	"bot_stop":                             true,
	"bot_quota":                            true,
}

var cvarValRe = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?$`)
var cvarLineRe = regexp.MustCompile(`(?m)^([a-zA-Z0-9_]+)\s*=\s*([^\s\r\n]+)`)

func (s *Server) handleGetCVars(w http.ResponseWriter, r *http.Request) {
	keys := []string{
		"sv_cheats", "sv_showimpacts", "sv_infinite_ammo",
		"sv_grenade_trajectory_prac_pipreview", "sv_regeneration_force_on",
		"ammo_grenade_limit_total", "mp_buy_anywhere", "mp_roundtime_defuse",
		"mp_startmoney", "mp_maxmoney", "mp_freezetime", "mp_friendlyfire",
		"mp_autoteambalance", "mp_limitteams", "mp_respawn_on_death_t",
		"mp_respawn_on_death_ct", "bot_difficulty", "bot_stop",
	}
	out, err := s.execRCONSmart(strings.Join(keys, "; "))
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	vals := map[string]string{}
	for _, m := range cvarLineRe.FindAllStringSubmatch(out, -1) {
		k := strings.TrimSpace(m[1])
		v := strings.Trim(strings.TrimSpace(m[2]), `"`)
		if allowedCVars[k] {
			if v == "true" {
				v = "1"
			} else if v == "false" {
				v = "0"
			}
			vals[k] = v
		}
	}
	writeJSON(w, 200, map[string]any{"ok": true, "cvars": vals})
}

func (s *Server) handleSetCVars(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Changes      map[string]string `json:"changes"`
		Action       string            `json:"action"`
		RestartRound bool              `json:"restart_round"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, "请求格式错误")
		return
	}
	var cmds []string
	for k, v := range req.Changes {
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if !allowedCVars[k] {
			writeErr(w, 400, "不允许修改该参数: "+k)
			return
		}
		if !cvarValRe.MatchString(v) {
			writeErr(w, 400, "参数值必须为数字: "+k+"="+v)
			return
		}
		cmds = append(cmds, k+" "+v)
	}
	switch req.Action {
	case "":
	case "rethrow_grenade":
		cmds = append(cmds, "sv_rethrow_last_grenade")
	case "clear_smokes":
		cmds = append(cmds, "ent_fire smokegrenade_projectile kill; ent_fire inferno kill")
	case "bot_place":
		cmds = append(cmds, "bot_place")
	case "swap_teams":
		cmds = append(cmds, "mp_swapteams")
	default:
		writeErr(w, 400, "未知快捷动作: "+req.Action)
		return
	}
	if req.RestartRound {
		cmds = append(cmds, "mp_restartgame 1")
	}
	if len(cmds) == 0 {
		writeErr(w, 400, "未指定任何修改")
		return
	}
	out, err := s.execRCONSmart(strings.Join(cmds, "; "))
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "executed": strings.Join(cmds, "; "), "output": out})
}

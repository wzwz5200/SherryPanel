package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type Store struct {
	SQL *sql.DB
}

type SkinRecord struct {
	Team          int     `json:"weapon_team"`
	DefIndex      int     `json:"weapon_defindex"`
	PaintID       int     `json:"weapon_paint_id"`
	Wear          float64 `json:"weapon_wear"`
	Seed          int     `json:"weapon_seed"`
	Nametag       string  `json:"weapon_nametag"`
	StatTrak      bool    `json:"weapon_stattrak"`
	StatTrakCount int     `json:"weapon_stattrak_count"`
}

type Loadout struct {
	SteamID string                `json:"steamid"`
	Skins   map[int]SkinRecord    `json:"skins"`    // keyed by team*100000 + defindex? No, split by team!
	SkinsT  map[int]SkinRecord    `json:"skins_t"`  // team 2
	SkinsCT map[int]SkinRecord    `json:"skins_ct"` // team 3
	KnifeT  string                `json:"knife_t"`
	KnifeCT string                `json:"knife_ct"`
	GloveT  int                   `json:"glove_t"`
	GloveCT int                   `json:"glove_ct"`
	AgentT  string                `json:"agent_t"`
	AgentCT string                `json:"agent_ct"`
	MusicT  int                   `json:"music_t"`
	MusicCT int                   `json:"music_ct"`
	PinT    int                   `json:"pin_t"`
	PinCT   int                   `json:"pin_ct"`
}

func Open(dsn string) (*Store, error) {
	conn, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	conn.SetMaxOpenConns(15)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(5 * time.Minute)
	if err := conn.Ping(); err != nil {
		return nil, err
	}
	return &Store{SQL: conn}, nil
}

func teamsFor(team int) []int {
	if team == 2 {
		return []int{2}
	}
	if team == 3 {
		return []int{3}
	}
	return []int{2, 3}
}

func (s *Store) KnownSteamIDs(defaultID string) []string {
	seen := map[string]bool{}
	res := []string{}
	if defaultID != "" {
		seen[defaultID] = true
		res = append(res, defaultID)
	}
	q := `
		SELECT DISTINCT steamid FROM wp_player_skins
		UNION SELECT DISTINCT steamid FROM wp_player_knife
		UNION SELECT DISTINCT steamid FROM wp_player_gloves
		UNION SELECT DISTINCT steamid FROM wp_player_agents
		UNION SELECT DISTINCT steamid FROM wp_player_music
		UNION SELECT DISTINCT steamid FROM wp_player_pins
	`
	rows, err := s.SQL.Query(q)
	if err != nil {
		return res
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil && id != "" && !seen[id] {
			seen[id] = true
			res = append(res, id)
		}
	}
	return res
}

func (s *Store) GetLoadout(steamID string) (*Loadout, error) {
	lo := &Loadout{
		SteamID: steamID,
		SkinsT:  map[int]SkinRecord{},
		SkinsCT: map[int]SkinRecord{},
	}

	// Skins
	rows, err := s.SQL.Query(`
		SELECT weapon_team, weapon_defindex, weapon_paint_id, weapon_wear, weapon_seed,
		       COALESCE(weapon_nametag, ''), weapon_stattrak, weapon_stattrak_count
		FROM wp_player_skins WHERE steamid = ?`, steamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var r SkinRecord
		var st int
		if err := rows.Scan(&r.Team, &r.DefIndex, &r.PaintID, &r.Wear, &r.Seed, &r.Nametag, &st, &r.StatTrakCount); err == nil {
			r.StatTrak = st != 0
			if r.Team == 2 {
				lo.SkinsT[r.DefIndex] = r
			} else if r.Team == 3 {
				lo.SkinsCT[r.DefIndex] = r
			}
		}
	}

	// Knives
	if kr, err := s.SQL.Query(`SELECT weapon_team, knife FROM wp_player_knife WHERE steamid = ?`, steamID); err == nil {
		defer kr.Close()
		for kr.Next() {
			var tm int
			var k string
			if kr.Scan(&tm, &k) == nil {
				if tm == 2 {
					lo.KnifeT = k
				} else if tm == 3 {
					lo.KnifeCT = k
				}
			}
		}
	}

	// Gloves
	if gr, err := s.SQL.Query(`SELECT weapon_team, weapon_defindex FROM wp_player_gloves WHERE steamid = ?`, steamID); err == nil {
		defer gr.Close()
		for gr.Next() {
			var tm, def int
			if gr.Scan(&tm, &def) == nil {
				if tm == 2 {
					lo.GloveT = def
				} else if tm == 3 {
					lo.GloveCT = def
				}
			}
		}
	}

	// Agents
	var act, at sql.NullString
	if err := s.SQL.QueryRow(`SELECT agent_ct, agent_t FROM wp_player_agents WHERE steamid = ?`, steamID).Scan(&act, &at); err == nil {
		lo.AgentCT = act.String
		lo.AgentT = at.String
	}

	// Music
	if mr, err := s.SQL.Query(`SELECT weapon_team, music_id FROM wp_player_music WHERE steamid = ?`, steamID); err == nil {
		defer mr.Close()
		for mr.Next() {
			var tm, mid int
			if mr.Scan(&tm, &mid) == nil {
				if tm == 2 {
					lo.MusicT = mid
				} else if tm == 3 {
					lo.MusicCT = mid
				}
			}
		}
	}

	// Pins
	if pr, err := s.SQL.Query(`SELECT weapon_team, id FROM wp_player_pins WHERE steamid = ?`, steamID); err == nil {
		defer pr.Close()
		for pr.Next() {
			var tm, pid int
			if pr.Scan(&tm, &pid) == nil {
				if tm == 2 {
					lo.PinT = pid
				} else if tm == 3 {
					lo.PinCT = pid
				}
			}
		}
	}

	return lo, nil
}

func (s *Store) SetSkin(steamID string, team int, rec SkinRecord) error {
	if rec.Wear < 0.000001 {
		rec.Wear = 0.000001
	}
	if rec.Wear > 1.0 {
		rec.Wear = 1.0
	}
	if rec.Seed < 0 {
		rec.Seed = 0
	}
	if rec.Seed > 1000 {
		rec.Seed = 1000
	}
	var nametag any = nil
	if rec.Nametag != "" {
		nametag = rec.Nametag
	}
	st := 0
	if rec.StatTrak {
		st = 1
	}
	tx, err := s.SQL.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, tm := range teamsFor(team) {
		if rec.PaintID == 0 && rec.Nametag == "" && !rec.StatTrak {
			if _, err := tx.Exec(`DELETE FROM wp_player_skins WHERE steamid=? AND weapon_team=? AND weapon_defindex=?`, steamID, tm, rec.DefIndex); err != nil {
				return err
			}
			continue
		}
		_, err := tx.Exec(`
			INSERT INTO wp_player_skins
				(steamid, weapon_team, weapon_defindex, weapon_paint_id, weapon_wear, weapon_seed, weapon_nametag, weapon_stattrak, weapon_stattrak_count)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE
				weapon_paint_id=VALUES(weapon_paint_id),
				weapon_wear=VALUES(weapon_wear),
				weapon_seed=VALUES(weapon_seed),
				weapon_nametag=VALUES(weapon_nametag),
				weapon_stattrak=VALUES(weapon_stattrak),
				weapon_stattrak_count=VALUES(weapon_stattrak_count)`,
			steamID, tm, rec.DefIndex, rec.PaintID, rec.Wear, rec.Seed, nametag, st, rec.StatTrakCount)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) SetKnife(steamID string, team int, knife string) error {
	tx, err := s.SQL.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, tm := range teamsFor(team) {
		if knife == "" || knife == "weapon_knife" {
			if _, err := tx.Exec(`DELETE FROM wp_player_knife WHERE steamid=? AND weapon_team=?`, steamID, tm); err != nil {
				return err
			}
		} else {
			if _, err := tx.Exec(`INSERT INTO wp_player_knife (steamid, weapon_team, knife) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE knife=VALUES(knife)`, steamID, tm, knife); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func (s *Store) SetGlove(steamID string, team int, defIndex, paintID int, wear float64, seed int) error {
	if wear < 0.000001 {
		wear = 0.000001
	}
	if wear > 1.0 {
		wear = 1.0
	}
	tx, err := s.SQL.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, tm := range teamsFor(team) {
		if defIndex == 0 {
			if _, err := tx.Exec(`DELETE FROM wp_player_gloves WHERE steamid=? AND weapon_team=?`, steamID, tm); err != nil {
				return err
			}
		} else {
			if _, err := tx.Exec(`INSERT INTO wp_player_gloves (steamid, weapon_team, weapon_defindex) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE weapon_defindex=VALUES(weapon_defindex)`, steamID, tm, defIndex); err != nil {
				return err
			}
			if _, err := tx.Exec(`
				INSERT INTO wp_player_skins (steamid, weapon_team, weapon_defindex, weapon_paint_id, weapon_wear, weapon_seed)
				VALUES (?, ?, ?, ?, ?, ?)
				ON DUPLICATE KEY UPDATE weapon_paint_id=VALUES(weapon_paint_id), weapon_wear=VALUES(weapon_wear), weapon_seed=VALUES(weapon_seed)`,
				steamID, tm, defIndex, paintID, wear, seed); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func (s *Store) SetAgent(steamID string, team int, model string) error {
	var val any = nil
	if model != "" && model != "null" {
		val = model
	}
	if team == 2 {
		_, err := s.SQL.Exec(`INSERT INTO wp_player_agents (steamid, agent_t) VALUES (?, ?) ON DUPLICATE KEY UPDATE agent_t=VALUES(agent_t)`, steamID, val)
		return err
	}
	if team == 3 {
		_, err := s.SQL.Exec(`INSERT INTO wp_player_agents (steamid, agent_ct) VALUES (?, ?) ON DUPLICATE KEY UPDATE agent_ct=VALUES(agent_ct)`, steamID, val)
		return err
	}
	return fmt.Errorf("agent requires team 2 (T) or 3 (CT)")
}

func (s *Store) SetMusic(steamID string, team int, musicID int) error {
	tx, err := s.SQL.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, tm := range teamsFor(team) {
		if musicID == 0 {
			if _, err := tx.Exec(`DELETE FROM wp_player_music WHERE steamid=? AND weapon_team=?`, steamID, tm); err != nil {
				return err
			}
		} else {
			if _, err := tx.Exec(`INSERT INTO wp_player_music (steamid, weapon_team, music_id) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE music_id=VALUES(music_id)`, steamID, tm, musicID); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func (s *Store) SetPin(steamID string, team int, pinID int) error {
	tx, err := s.SQL.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, tm := range teamsFor(team) {
		if pinID == 0 {
			if _, err := tx.Exec(`DELETE FROM wp_player_pins WHERE steamid=? AND weapon_team=?`, steamID, tm); err != nil {
				return err
			}
		} else {
			if _, err := tx.Exec(`INSERT INTO wp_player_pins (steamid, weapon_team, id) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE id=VALUES(id)`, steamID, tm, pinID); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func (s *Store) ResetLoadout(steamID string) error {
	tx, err := s.SQL.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, tbl := range []string{"wp_player_skins", "wp_player_knife", "wp_player_gloves", "wp_player_agents", "wp_player_music", "wp_player_pins"} {
		if _, err := tx.Exec("DELETE FROM "+tbl+" WHERE steamid=?", steamID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

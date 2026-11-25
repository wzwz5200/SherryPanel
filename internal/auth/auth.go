package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	AdminCookieName  = "cs2_admin_session"
	PlayerCookieName = "cs2_player_session"
)

type PairingRecord struct {
	Token     string    `json:"token"`
	Code      string    `json:"code"`
	SteamID   string    `json:"steamid"`
	ExpiresAt time.Time `json:"expires_at"`
}

type Manager struct {
	secret        []byte
	adminPassword string
	db            *sql.DB

	mu       sync.Mutex
	memCodes map[string]PairingRecord // code -> record (fallback when DB is absent)
	memToks  map[string]PairingRecord // token -> record
	failLog  map[string][]time.Time   // ip -> failed attempts
}

func New(secret, adminPassword string, sqlDB *sql.DB) *Manager {
	m := &Manager{
		secret:        []byte(secret),
		adminPassword: adminPassword,
		memCodes:      map[string]PairingRecord{},
		memToks:       map[string]PairingRecord{},
		failLog:       map[string][]time.Time{},
	}
	m.SetDB(sqlDB)
	return m
}

func (m *Manager) SetDB(sqlDB *sql.DB) {
	m.mu.Lock()
	m.db = sqlDB
	m.mu.Unlock()
	if sqlDB != nil {
		_, _ = sqlDB.Exec(`
			CREATE TABLE IF NOT EXISTS wp_web_tokens (
				token VARCHAR(64) NOT NULL PRIMARY KEY,
				code VARCHAR(16) NOT NULL,
				steamid VARCHAR(20) NOT NULL,
				expires_at DATETIME NOT NULL,
				used TINYINT(1) NOT NULL DEFAULT 0,
				INDEX idx_code (code),
				INDEX idx_exp (expires_at)
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
		`)
	}
}

func (m *Manager) CheckRateLimit(ip string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-5 * time.Minute)
	var recent []time.Time
	for _, t := range m.failLog[ip] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	m.failLog[ip] = recent
	return len(recent) < 10
}

func (m *Manager) RecordFailure(ip string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failLog[ip] = append(m.failLog[ip], time.Now())
}

func (m *Manager) VerifyAdminPassword(pw string) bool {
	if m.adminPassword == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(pw), []byte(m.adminPassword)) == 1
}

func (m *Manager) sign(payload string) string {
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + sig
}

func (m *Manager) verify(cookieVal string) (string, bool) {
	parts := strings.Split(cookieVal, ".")
	if len(parts) != 2 {
		return "", false
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", false
	}
	expected := m.sign(string(raw))
	if subtle.ConstantTimeCompare([]byte(cookieVal), []byte(expected)) != 1 {
		return "", false
	}
	return string(raw), true
}

func (m *Manager) SetAdminCookie(w http.ResponseWriter) {
	exp := time.Now().Add(24 * time.Hour).Unix()
	val := m.sign(fmt.Sprintf("admin|%d", exp))
	http.SetCookie(w, &http.Cookie{
		Name:     AdminCookieName,
		Value:    val,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   86400,
	})
}

func (m *Manager) ClearAdminCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: AdminCookieName, Value: "", Path: "/", HttpOnly: true, MaxAge: -1})
}

func (m *Manager) IsAdminAuthed(r *http.Request) bool {
	c, err := r.Cookie(AdminCookieName)
	if err != nil {
		return false
	}
	payload, ok := m.verify(c.Value)
	if !ok {
		return false
	}
	parts := strings.Split(payload, "|")
	if len(parts) != 2 || parts[0] != "admin" {
		return false
	}
	exp, err := strconv.ParseInt(parts[1], 10, 64)
	return err == nil && time.Now().Unix() < exp
}

func (m *Manager) SetPlayerCookie(w http.ResponseWriter, steamID string) {
	exp := time.Now().Add(30 * 24 * time.Hour).Unix()
	val := m.sign(fmt.Sprintf("player|%s|%d", steamID, exp))
	http.SetCookie(w, &http.Cookie{
		Name:     PlayerCookieName,
		Value:    val,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   30 * 86400,
	})
}

func (m *Manager) ClearPlayerCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: PlayerCookieName, Value: "", Path: "/", HttpOnly: true, MaxAge: -1})
}

func (m *Manager) GetPlayerSteamID(r *http.Request) string {
	c, err := r.Cookie(PlayerCookieName)
	if err != nil {
		return ""
	}
	payload, ok := m.verify(c.Value)
	if !ok {
		return ""
	}
	parts := strings.Split(payload, "|")
	if len(parts) != 3 || parts[0] != "player" {
		return ""
	}
	exp, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil || time.Now().Unix() >= exp {
		return ""
	}
	return parts[1]
}

func randomDigits(n int) string {
	var sb strings.Builder
	for i := 0; i < n; i++ {
		v, _ := rand.Int(rand.Reader, big.NewInt(10))
		sb.WriteByte(byte('0' + v.Int64()))
	}
	return sb.String()
}

func randomToken(nBytes int) string {
	b := make([]byte, nBytes)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (m *Manager) CreatePairing(steamID string, ttl time.Duration) (PairingRecord, error) {
	rec := PairingRecord{
		Token:     randomToken(18),
		Code:      randomDigits(6),
		SteamID:   steamID,
		ExpiresAt: time.Now().UTC().Add(ttl),
	}
	m.mu.Lock()
	dbConn := m.db
	m.mu.Unlock()
	if dbConn != nil {
		_, _ = dbConn.Exec(`DELETE FROM wp_web_tokens WHERE expires_at < UTC_TIMESTAMP() OR used = 1`)
		_, err := dbConn.Exec(`INSERT INTO wp_web_tokens (token, code, steamid, expires_at, used) VALUES (?, ?, ?, ?, 0)`,
			rec.Token, rec.Code, rec.SteamID, rec.ExpiresAt.Format("2006-01-02 15:04:05"))
		if err != nil {
			return rec, err
		}
		return rec, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.memCodes[rec.Code] = rec
	m.memToks[rec.Token] = rec
	return rec, nil
}

func (m *Manager) RedeemCodeOrToken(code, token string) (string, error) {
	code = strings.TrimSpace(code)
	token = strings.TrimSpace(token)
	if code == "" && token == "" {
		return "", fmt.Errorf("请提供配对码或专属令牌")
	}
	m.mu.Lock()
	dbConn := m.db
	m.mu.Unlock()
	if dbConn != nil {
		var dbTok, sid string
		var err error
		if token != "" {
			err = dbConn.QueryRow(`SELECT token, steamid FROM wp_web_tokens WHERE token = ? AND used = 0 AND expires_at > UTC_TIMESTAMP() LIMIT 1`, token).Scan(&dbTok, &sid)
		} else {
			err = dbConn.QueryRow(`SELECT token, steamid FROM wp_web_tokens WHERE code = ? AND used = 0 AND expires_at > UTC_TIMESTAMP() LIMIT 1`, code).Scan(&dbTok, &sid)
		}
		if err != nil {
			return "", fmt.Errorf("配对码或专属链接无效或已过期，请在游戏内重新输入 !bind 或联系房主重新生成")
		}
		_, _ = dbConn.Exec(`UPDATE wp_web_tokens SET used = 1 WHERE token = ?`, dbTok)
		return sid, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	var rec PairingRecord
	var ok bool
	if token != "" {
		rec, ok = m.memToks[token]
	} else {
		rec, ok = m.memCodes[code]
	}
	if !ok || time.Now().UTC().After(rec.ExpiresAt) {
		return "", fmt.Errorf("配对码或专属链接无效或已过期")
	}
	delete(m.memCodes, rec.Code)
	delete(m.memToks, rec.Token)
	return rec.SteamID, nil
}

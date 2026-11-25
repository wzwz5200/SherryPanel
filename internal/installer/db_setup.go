package installer

import (
	"archive/zip"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"cs2-skins-web/internal/db"
)

func randomHexStr(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func isTCPReady(addr string) bool {
	c, err := net.DialTimeout("tcp", addr, 400*time.Millisecond)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}

// EnsureManagedMariaDBRunning starts the portable MariaDB instance on Windows if installed.
func EnsureManagedMariaDBRunning(baseDir string) (bool, error) {
	if runtime.GOOS != "windows" {
		return false, nil
	}
	daemon := filepath.Join(baseDir, "mariadb", "bin", "mariadbd.exe")
	if _, err := os.Stat(daemon); err != nil {
		daemon = filepath.Join(baseDir, "mariadb", "bin", "mysqld.exe")
		if _, err := os.Stat(daemon); err != nil {
			return false, nil
		}
	}
	dataDir := filepath.Join(baseDir, "mariadb-data")
	if _, err := os.Stat(filepath.Join(dataDir, "mysql")); err != nil {
		return false, nil
	}
	if isTCPReady("127.0.0.1:3306") {
		return true, nil
	}
	return startManagedMariaDB(daemon, dataDir, "")
}

func startManagedMariaDB(daemon, dataDir, initFile string) (bool, error) {
	_ = os.MkdirAll(dataDir, 0755)
	logFile, err := os.OpenFile(filepath.Join(dataDir, "mariadb.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		logFile = nil
	}
	args := []string{
		"--datadir=" + dataDir,
		"--port=3306",
		"--bind-address=127.0.0.1",
	}
	if initFile != "" {
		args = append(args, "--init-file="+initFile)
	}
	cmd := exec.Command(daemon, args...)
	if logFile != nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	}
	if err := cmd.Start(); err != nil {
		if logFile != nil {
			_ = logFile.Close()
		}
		return false, fmt.Errorf("启动便携版 MariaDB 失败: %w", err)
	}
	if logFile != nil {
		_ = logFile.Close()
	}

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if isTCPReady("127.0.0.1:3306") {
			return true, nil
		}
		time.Sleep(350 * time.Millisecond)
	}
	return false, fmt.Errorf("MariaDB 进程已启动但 3306 端口超时未就绪，请查看 %s", filepath.Join(dataDir, "mariadb.log"))
}

func resolveWindowsMariaDBURL() string {
	const fallback = "https://archive.mariadb.org/mariadb-11.4.4/winx64-packages/mariadb-11.4.4-winx64.zip"
	c := &http.Client{Timeout: 12 * time.Second}
	req, _ := http.NewRequest("GET", "https://downloads.mariadb.org/rest-api/mariadb/11.4/", nil)
	req.Header.Set("User-Agent", "SherryPanel/1.3")
	resp, err := c.Do(req)
	if err != nil {
		return fallback
	}
	defer resp.Body.Close()
	var parsed struct {
		Releases map[string]struct {
			Files []struct {
				FileName string `json:"file_name"`
				OS       string `json:"os"`
				CPU      string `json:"cpu"`
				Type     string `json:"package_type"`
				URL      string `json:"file_download_url"`
			} `json:"files"`
		} `json:"releases"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return fallback
	}
	for _, rel := range parsed.Releases {
		for _, f := range rel.Files {
			if f.OS == "Windows" && f.CPU == "x86_64" && f.Type == "ZIP file" && !strings.Contains(strings.ToLower(f.FileName), "debugsymbols") && f.URL != "" {
				return f.URL
			}
		}
	}
	return fallback
}

func unzipStripTopLevel(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		name := strings.ReplaceAll(f.Name, `\`, `/`)
		if idx := strings.IndexByte(name, '/'); idx >= 0 {
			name = name[idx+1:]
		}
		if name == "" {
			continue
		}
		fpath := filepath.Join(dest, filepath.FromSlash(name))
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

func (m *Manager) notifyDBReady(dsn string) {
	m.mu.Lock()
	m.cfg.DBDSN = dsn
	cb := m.onDBReady
	m.mu.Unlock()
	if cb != nil {
		cb(dsn)
	}
}

func (m *Manager) installDatabase(dryRun bool) error {
	m.setStage("自动部署并初始化皮肤数据库 (MariaDB)")

	// If existing DSN already works, just ensure schema & attach.
	if m.cfg.DBDSN != "" {
		if st, err := db.Open(m.cfg.DBDSN); err == nil {
			if err := st.EnsureSchema(); err == nil {
				_ = st.SQL.Close()
				m.appendLog("检测到现有 MariaDB 数据库连接正常，已完成 7 张皮肤表结构校验")
				m.notifyDBReady(m.cfg.DBDSN)
				return nil
			}
			_ = st.SQL.Close()
		}
	}

	if dryRun {
		if runtime.GOOS == "windows" {
			_ = m.checkURL("https://downloads.mariadb.org/rest-api/mariadb/11.4/")
			m.appendLog(fmt.Sprintf("[DRY-RUN] 将在 %s 下载便携版 MariaDB 11.4 (winx64.zip)", filepath.Join(m.baseDir(), "mariadb")))
		} else {
			m.appendLog("[DRY-RUN] 将通过 apt-get 安装并启动 mariadb-server")
		}
		m.appendLog("[DRY-RUN] 将自动创建 cs2_weaponpaints 库、wpuser 随机密码账号及 7 张 wp_* 皮肤/配对表")
		m.appendLog("[DRY-RUN] 将自动热连接面板数据库并同步写入 WeaponPaints.json")
		return nil
	}

	if runtime.GOOS == "windows" {
		return m.installWindowsMariaDB()
	}
	return m.installLinuxMariaDB()
}

func (m *Manager) installWindowsMariaDB() error {
	base := m.baseDir()
	dbRoot := filepath.Join(base, "mariadb")
	dataDir := filepath.Join(base, "mariadb-data")
	daemon := filepath.Join(dbRoot, "bin", "mariadbd.exe")

	if _, err := os.Stat(daemon); err != nil {
		dlURL := resolveWindowsMariaDBURL()
		m.appendLog("下载官方便携版 MariaDB: " + dlURL)
		tmpZip := filepath.Join(os.TempDir(), "sherry-mariadb-winx64.zip")
		if err := downloadFile(dlURL, tmpZip); err != nil {
			return fmt.Errorf("下载 MariaDB 失败: %w", err)
		}
		m.appendLog("正在解压便携版 MariaDB 至 " + dbRoot)
		if err := os.MkdirAll(dbRoot, 0755); err != nil {
			return err
		}
		if err := unzipStripTopLevel(tmpZip, dbRoot); err != nil {
			_ = os.Remove(tmpZip)
			return fmt.Errorf("解压 MariaDB 失败: %w", err)
		}
		_ = os.Remove(tmpZip)
	}

	if _, err := os.Stat(daemon); err != nil {
		daemon = filepath.Join(dbRoot, "bin", "mysqld.exe")
	}
	if _, err := os.Stat(daemon); err != nil {
		return fmt.Errorf("未在 %s 找到 mariadbd.exe", filepath.Join(dbRoot, "bin"))
	}

	if _, err := os.Stat(filepath.Join(dataDir, "mysql")); err != nil {
		m.appendLog("首次初始化 MariaDB 数据目录: " + dataDir)
		_ = os.MkdirAll(dataDir, 0755)
		initBin := filepath.Join(dbRoot, "bin", "mariadb-install-db.exe")
		if _, err := os.Stat(initBin); err != nil {
			initBin = filepath.Join(dbRoot, "bin", "mysql_install_db.exe")
		}
		if err := m.runCmdStream(exec.Command(initBin, "--datadir="+dataDir)); err != nil {
			return fmt.Errorf("初始化 MariaDB 数据目录失败: %w", err)
		}
	}

	pw := randomHexStr(12)
	initSQLPath := filepath.Join(dataDir, "sherry-init.sql")
	initSQL := fmt.Sprintf(`CREATE DATABASE IF NOT EXISTS cs2_weaponpaints CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;
CREATE USER IF NOT EXISTS 'wpuser'@'127.0.0.1' IDENTIFIED BY '%s';
ALTER USER 'wpuser'@'127.0.0.1' IDENTIFIED BY '%s';
CREATE USER IF NOT EXISTS 'wpuser'@'localhost' IDENTIFIED BY '%s';
ALTER USER 'wpuser'@'localhost' IDENTIFIED BY '%s';
GRANT ALL PRIVILEGES ON cs2_weaponpaints.* TO 'wpuser'@'127.0.0.1';
GRANT ALL PRIVILEGES ON cs2_weaponpaints.* TO 'wpuser'@'localhost';
FLUSH PRIVILEGES;
`, pw, pw, pw, pw)

	if isTCPReady("127.0.0.1:3306") {
		m.appendLog("检测到 127.0.0.1:3306 已有 MariaDB 运行，尝试自动建库...")
		// Try passwordless local root TCP first
		if st, err := db.Open("root@tcp(127.0.0.1:3306)/?parseTime=true"); err == nil {
			for _, line := range strings.Split(initSQL, ";\n") {
				q := strings.TrimSpace(line)
				if q != "" {
					_, _ = st.SQL.Exec(q)
				}
			}
			_ = st.SQL.Close()
		} else {
			return fmt.Errorf("端口 3306 已被其他数据库占用，请先在 panel-config.json 中填写 db_dsn 或释放 3306 端口")
		}
	} else {
		if err := os.WriteFile(initSQLPath, []byte(initSQL), 0600); err != nil {
			return err
		}
		m.appendLog("启动便携版 MariaDB 并初始化 cs2_weaponpaints 数据库...")
		_, err := startManagedMariaDB(daemon, dataDir, initSQLPath)
		_ = os.Remove(initSQLPath)
		if err != nil {
			return err
		}
	}

	dsn := fmt.Sprintf("wpuser:%s@tcp(127.0.0.1:3306)/cs2_weaponpaints?parseTime=true", pw)
	st, err := db.Open(dsn)
	if err != nil {
		return fmt.Errorf("连接新建 MariaDB 失败: %w", err)
	}
	if err := st.EnsureSchema(); err != nil {
		_ = st.SQL.Close()
		return fmt.Errorf("创建皮肤数据表失败: %w", err)
	}
	_ = st.SQL.Close()

	m.appendLog("便携版 MariaDB 与 7 张皮肤数据表初始化成功！")
	m.notifyDBReady(dsn)
	return nil
}

func (m *Manager) installLinuxMariaDB() error {
	pw := randomHexStr(12)
	script := fmt.Sprintf(`set -e
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq mariadb-server >/dev/null
systemctl enable --now mariadb
mysql -e "CREATE DATABASE IF NOT EXISTS cs2_weaponpaints CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;
CREATE USER IF NOT EXISTS 'wpuser'@'127.0.0.1' IDENTIFIED BY '%s';
ALTER USER 'wpuser'@'127.0.0.1' IDENTIFIED BY '%s';
CREATE USER IF NOT EXISTS 'wpuser'@'localhost' IDENTIFIED BY '%s';
ALTER USER 'wpuser'@'localhost' IDENTIFIED BY '%s';
GRANT ALL PRIVILEGES ON cs2_weaponpaints.* TO 'wpuser'@'127.0.0.1';
GRANT ALL PRIVILEGES ON cs2_weaponpaints.* TO 'wpuser'@'localhost';
FLUSH PRIVILEGES;"`, pw, pw, pw, pw)
	if err := m.runCmdStream(exec.Command("bash", "-c", script)); err != nil {
		return err
	}
	dsn := fmt.Sprintf("wpuser:%s@tcp(127.0.0.1:3306)/cs2_weaponpaints?parseTime=true", pw)
	st, err := db.Open(dsn)
	if err != nil {
		return err
	}
	if err := st.EnsureSchema(); err != nil {
		_ = st.SQL.Close()
		return err
	}
	_ = st.SQL.Close()
	m.appendLog("Linux MariaDB 与 7 张皮肤数据表初始化成功！")
	m.notifyDBReady(dsn)
	return nil
}

package main

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"cs2-skins-web/internal/api"
	"cs2-skins-web/internal/auth"
	"cs2-skins-web/internal/catalog"
	"cs2-skins-web/internal/config"
	"cs2-skins-web/internal/db"
	"cs2-skins-web/internal/installer"
)

//go:embed all:web/dist
var distFS embed.FS

func detectPublicIP(configured string) string {
	if configured != "" {
		return configured
	}
	req, _ := http.NewRequest("GET", "http://metadata.google.internal/computeMetadata/v1/instance/network-interfaces/0/access-configs/0/external-ip", nil)
	req.Header.Set("Metadata-Flavor", "Google")
	c := &http.Client{Timeout: 2 * time.Second}
	if resp, err := c.Do(req); err == nil {
		defer resp.Body.Close()
		if b, err := io.ReadAll(resp.Body); err == nil {
			if ip := strings.TrimSpace(string(b)); ip != "" {
				return ip
			}
		}
	}
	return "127.0.0.1"
}

func spaHandler(sub fs.FS) http.HandlerFunc {
	fileServer := http.FileServer(http.FS(sub))
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path != "" {
			if _, err := fs.Stat(sub, path); err == nil {
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		indexBytes, err := fs.ReadFile(sub, "index.html")
		if err != nil {
			http.Error(w, "frontend not built", 500)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(indexBytes)
	}
}

func defaultConfigPath() string {
	if v := strings.TrimSpace(os.Getenv("PANEL_CONFIG")); v != "" {
		return v
	}
	if runtime.GOOS == "windows" {
		wd, _ := os.Getwd()
		return filepath.Join(wd, "panel-config.json")
	}
	return "/opt/cs2-skins-web/panel-config.json"
}

func main() {
	cfgPath := defaultConfigPath()

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "setup", "init":
			cfg, dlCS2, dlPlugins := config.PromptWizard(cfgPath)
			mgr := installer.New(cfg)
			if dlCS2 && dlPlugins {
				if err := mgr.RunSync("all", cfg.DryRun); err != nil {
					log.Fatalf("自动安装失败: %v", err)
				}
			} else if dlCS2 {
				if err := mgr.RunSync("cs2", cfg.DryRun); err != nil {
					log.Fatalf("CS2 下载失败: %v", err)
				}
			} else if dlPlugins {
				if err := mgr.RunSync("plugins", cfg.DryRun); err != nil {
					log.Fatalf("插件安装失败: %v", err)
				}
			}
			fmt.Println("向导完成！运行可执行文件即可启动分离式玩家端与管理端。")
			return
		case "install":
			task := "all"
			dry := false
			for _, a := range os.Args[2:] {
				if a == "--dry-run" {
					dry = true
				} else {
					task = a
				}
			}
			cfg := config.Load(cfgPath)
			if cfg.DryRun {
				dry = true
			}
			mgr := installer.New(cfg)
			if err := mgr.RunSync(task, dry); err != nil {
				log.Fatalf("安装失败: %v", err)
			}
			for _, ln := range mgr.GetStatus().Logs {
				fmt.Println(ln)
			}
			return
		case "help", "-h", "--help":
			fmt.Printf("CS2 安全分离式服务端管理与免登录皮肤工坊 (%s/%s)\n", runtime.GOOS, runtime.GOARCH)
			fmt.Println("用法:")
			fmt.Println("  ./cs2-panel                      启动玩家换肤端 (:8080) + 独立管理端 (:9090)")
			fmt.Println("  ./cs2-panel setup                交互式配置向导并可选自动下载 CS2 服务端")
			fmt.Println("  ./cs2-panel install [cs2|plugins|all] [--dry-run]  自动下载安装服务端/插件")
			return
		}
	}

	cfg := config.Load(cfgPath)

	var store *db.Store
	var sqlDB *sql.DB
	if cfg.DBDSN != "" {
		if st, err := db.Open(cfg.DBDSN); err != nil {
			log.Printf("WARN: MariaDB unavailable (%v); continuing without DB", err)
		} else {
			store = st
			sqlDB = st.SQL
			defer store.SQL.Close()
		}
	}

	var cat *catalog.Catalog
	var catJSON []byte
	if c, err := catalog.Load(cfg.DataDir, cfg.ImgDir); err != nil {
		log.Printf("WARN: skin catalog unavailable (%v)", err)
	} else {
		cat = c
		catJSON, _ = json.Marshal(c)
	}

	authMgr := auth.New(cfg.SessionSecret, cfg.AdminPassword, sqlDB)
	pubIP := detectPublicIP(cfg.PublicIP)
	srv := &api.Server{
		Cfg:         cfg,
		Store:       store,
		Auth:        authMgr,
		Catalog:     cat,
		CatalogJSON: catJSON,
		Installer:   installer.New(cfg),
		PublicIP:    pubIP,
	}

	sub, err := fs.Sub(distFS, "web/dist")
	if err != nil {
		log.Fatalf("embed sub failed: %v", err)
	}

	// 1. Public Player Skin Workshop Server (ZERO admin routes!)
	playerMux := http.NewServeMux()
	srv.RegisterPlayer(playerMux)
	imgHandler := http.StripPrefix("/img/", http.FileServer(http.Dir(cfg.ImgDir)))
	playerMux.Handle("GET /img/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=604800")
		imgHandler.ServeHTTP(w, r)
	}))
	playerMux.HandleFunc("/", spaHandler(sub))

	// 2. Isolated Admin Management Server (Strict password login!)
	adminMux := http.NewServeMux()
	srv.RegisterAdmin(adminMux)
	adminMux.HandleFunc("/", spaHandler(sub))

	adminAddr := cfg.AdminBind + ":" + cfg.AdminPort
	playerAddr := ":" + cfg.PlayerPort

	errCh := make(chan error, 2)
	go func() {
		log.Printf("[ADMIN ] Isolated Server Manager listening on http://%s (OS=%s/%s dry_run=%v)", adminAddr, runtime.GOOS, runtime.GOARCH, cfg.DryRun)
		s := &http.Server{Addr: adminAddr, Handler: adminMux, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second}
		errCh <- s.ListenAndServe()
	}()
	go func() {
		log.Printf("[PLAYER] Public Skin Workshop listening on http://0.0.0.0%s (public IP: %s)", playerAddr, pubIP)
		s := &http.Server{Addr: playerAddr, Handler: playerMux, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second}
		errCh <- s.ListenAndServe()
	}()

	log.Fatal(<-errCh)
}

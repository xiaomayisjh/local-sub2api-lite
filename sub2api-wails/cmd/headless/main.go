package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"sub2api-wails/internal/config"
	"sub2api-wails/internal/pkg/logger"
	"sub2api-wails/internal/repository"
	"sub2api-wails/internal/server"

	"github.com/gin-gonic/gin"
)

func main() {
	exePath, err := os.Executable()
	if err != nil {
		log.Fatalf("get executable path: %v", err)
	}
	dataDir := filepath.Join(filepath.Dir(exePath), "data")
	os.MkdirAll(dataDir, 0755)

	configPath := filepath.Join(dataDir, "config.yaml")
	cfg, err := config.LoadForBootstrap()
	if err != nil {
		if err := createDefaultConfig(configPath); err != nil {
			log.Fatalf("create default config: %v", err)
		}
		cfg, err = config.LoadForBootstrap()
		if err != nil {
			log.Fatalf("load config: %v", err)
		}
	}

	cfg.Database.Type = "sqlite"
	cfg.Database.SQLitePath = filepath.Join(dataDir, "sub2api.db")
	cfg.RunMode = config.RunModeSimple

	if portStr := os.Getenv("SERVER_PORT"); portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			cfg.Server.Port = p
		}
	}

	if err := logger.Init(logger.OptionsFromConfig(cfg.Log)); err != nil {
		log.Fatalf("init logger: %v", err)
	}

	entClient, db, err := repository.InitEnt(cfg)
	if err != nil {
		log.Fatalf("init database: %v", err)
	}
	defer entClient.Close()

	r := gin.New()
	r.Use(gin.Recovery())

	srv, err := server.Initialize(r, cfg, entClient, db)
	if err != nil {
		log.Fatalf("init server: %v", err)
	}

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv.Addr = addr

	log.Printf("API server starting on %s (headless mode)", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

func createDefaultConfig(path string) error {
	defaultCfg := `server:
  host: "0.0.0.0"
  port: 8080
  mode: "release"

database:
  type: sqlite
  sqlite_path: "sub2api.db"

log:
  level: "info"
  format: "json"
  output:
    to_stdout: true
    to_file: false

jwt:
  secret: "sub2api-wails-local-secret-change-me"
  expire_hours: 168

run_mode: simple
timezone: "Asia/Shanghai"

billing:
  enabled: false

default:
  admin_email: "admin@sub2api.local"
  admin_password: "admin123"

gateway:
  upstream_timeout_seconds: 300
`
	return os.WriteFile(path, []byte(defaultCfg), 0644)
}

func init() {
	time.Local = time.FixedZone("CST", 8*3600)
}

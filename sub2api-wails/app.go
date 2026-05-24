package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"sub2api-wails/internal/config"
	"sub2api-wails/internal/pkg/logger"
	"sub2api-wails/internal/pkg/redismem"
	"sub2api-wails/internal/repository"
	"sub2api-wails/internal/server"

	"github.com/gin-gonic/gin"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx       context.Context
	cfg       *config.Config
	server    *http.Server
	entClient interface{ Close() error }
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) GetServerPort() int {
	if a.cfg != nil {
		return a.cfg.Server.Port
	}
	return 8080
}

func (a *App) GetServerStatus() string {
	if a.server != nil {
		return "running"
	}
	return "stopped"
}

func (a *App) StartServer() error {
	if a.server != nil {
		return fmt.Errorf("server already running")
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}
	dataDir := filepath.Join(filepath.Dir(exePath), "data")
	os.MkdirAll(dataDir, 0755)

	configPath := filepath.Join(dataDir, "config.yaml")
	cfg, err := config.LoadForBootstrap()
	if err != nil {
		if err := createDefaultConfig(configPath); err != nil {
			return fmt.Errorf("create default config: %w", err)
		}
		cfg, err = config.LoadForBootstrap()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
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
		return fmt.Errorf("init logger: %w", err)
	}

	entClient, db, err := repository.InitEnt(cfg)
	if err != nil {
		return fmt.Errorf("init database: %w", err)
	}
	a.entClient = entClient

	redisStub := redismem.NewRedisStub()
	_ = redisStub

	r := gin.New()
	r.Use(gin.Recovery())

	srv, err := server.Initialize(r, cfg, entClient, db)
	if err != nil {
		entClient.Close()
		return fmt.Errorf("init server: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	a.cfg = cfg

	a.server = &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Server error: %v", err)
		}
	}()

	log.Printf("API server started on %s", addr)
	_ = srv
	return nil
}

func (a *App) StopServer() error {
	if a.server == nil {
		return fmt.Errorf("server not running")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := a.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown server: %w", err)
	}
	a.server = nil
	if a.entClient != nil {
		a.entClient.Close()
		a.entClient = nil
	}
	return nil
}

func (a *App) OpenInBrowser() {
	if a.cfg != nil {
		url := fmt.Sprintf("http://localhost:%d", a.cfg.Server.Port)
		wailsRuntime.BrowserOpenURL(a.ctx, url)
	}
}

func (a *App) shutdown() {
	if a.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		a.server.Shutdown(ctx)
	}
	if a.entClient != nil {
		a.entClient.Close()
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
  admin_email: "admin@localhost"
  admin_password: "admin123"

gateway:
  upstream_timeout_seconds: 300
`
	return os.WriteFile(path, []byte(defaultCfg), 0644)
}

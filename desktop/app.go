package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strconv"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/localconfig"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/serverapp"
	"github.com/Wei-Shaw/sub2api/internal/setup"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App binds Wails lifecycle to the embedded Sub2API HTTP server.
type App struct {
	ctx               context.Context
	dataDir           string
	cfg               *config.Config
	app               *serverapp.Application
	serverMu          sync.Mutex
	startOnce         sync.Once
	startErr          error
	portSwitchMessage string
}

// NewApp creates the desktop application shell.
func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	dataDir, err := localconfig.EnsureDesktopEnvironment()
	if err != nil {
		a.startErr = err
		return
	}
	a.dataDir = dataDir

	cfg, err := config.LoadForBootstrap()
	if err != nil {
		a.startErr = err
		return
	}
	a.cfg = cfg
	if err := logger.Init(logger.OptionsFromConfig(cfg.Log)); err != nil {
		a.startErr = err
		return
	}

	// Start embedded HTTP server before WebView DOM is ready so the API is
	// available as soon as the window opens (and for health checks).
	go a.ensureServer()
}

func (a *App) domReady(ctx context.Context) {
	go func() {
		a.ensureServer()
		if a.startErr != nil {
			wailsruntime.MessageDialog(ctx, wailsruntime.MessageDialogOptions{
				Title:   "启动失败",
				Message: a.startErr.Error(),
				Type:    wailsruntime.ErrorDialog,
			})
			return
		}
		if msg := a.portSwitchMessage; msg != "" {
			wailsruntime.MessageDialog(ctx, wailsruntime.MessageDialogOptions{
				Title:   "端口已调整",
				Message: msg,
				Type:    wailsruntime.InfoDialog,
			})
		}
		a.waitForHealthy(60 * time.Second)
		a.navigateToWebUI(ctx)
	}()
}

// navigateToWebUI opens the embedded HTTP admin UI without reloading an already-active SPA session.
func (a *App) navigateToWebUI(ctx context.Context) {
	redirect := url.PathEscape("/admin/dashboard")
	target := a.ServerAddress() + "/login?redirect=" + redirect
	// Skip hard navigation when the Vue app is already running (prevents white-screen reload on /login).
	js := fmt.Sprintf(`(function(t){try{var u=new URL(window.location.href),n=new URL(t);if(u.origin===n.origin&&u.pathname!=="/"&&!document.getElementById("status"))return}catch(e){}window.location.replace(t)})(%s);`, strconv.Quote(target))
	wailsruntime.WindowExecJS(ctx, js)
}

func (a *App) ensureServer() {
	a.startOnce.Do(func() {
		if err := a.prepareListenPort(); err != nil {
			a.startErr = err
			return
		}
		buildInfo := handler.BuildInfo{
			Version:   "local-desktop",
			BuildType: "desktop",
		}
		application, err := serverapp.Initialize(buildInfo)
		if err != nil {
			a.startErr = err
			return
		}
		a.app = application
		go func() {
			if err := application.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				a.startErr = err
			}
		}()
		a.waitForHealthy(60 * time.Second)
	})
}

func (a *App) waitForHealthy(timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	addr := a.ServerAddress()
	for time.Now().Before(deadline) {
		resp, err := http.Get(addr + "/api/v1/settings/public")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func (a *App) shutdown(ctx context.Context) {
	a.serverMu.Lock()
	defer a.serverMu.Unlock()
	if a.app != nil {
		_ = a.app.Server.Shutdown(ctx)
		a.app.Cleanup()
	}
	repository.CloseEmbeddedRedis()
	logger.Sync()
}

// ServerAddress returns the local HTTP URL for the embedded server.
func (a *App) ServerAddress() string {
	port := 8080
	if a.cfg != nil && a.cfg.Server.Port > 0 {
		port = a.cfg.Server.Port
	}
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}

// GetLocalInfo returns desktop runtime information for the UI.
func (a *App) GetLocalInfo() map[string]string {
	a.ensureServer()
	return map[string]string{
		"data_dir":           a.dataDir,
		"server_address":     a.ServerAddress(),
		"default_api_key":    repository.GetDefaultAPIKeyPlaintext(),
		"admin_password":     repository.GetGeneratedAdminPassword(),
		"run_mode":           "local",
	}
}

// GetConfig returns current host/port settings.
func (a *App) GetConfig() map[string]any {
	if a.cfg == nil {
		return map[string]any{}
	}
	return map[string]any{
		"host": a.cfg.Server.Host,
		"port": a.cfg.Server.Port,
	}
}

// CheckPort reports whether a port can be used on the configured listen host.
func (a *App) CheckPort(port int) map[string]any {
	host := a.listenHost()
	current := a.currentPort()
	available := port == current || localconfig.IsPortAvailable(host, port)
	msg := ""
	if !available {
		msg = "端口已被其他程序占用"
	}
	return map[string]any{
		"port":         port,
		"available":    available,
		"current_port": current,
		"host":         host,
		"message":      msg,
	}
}

// SetPort updates server.port in config.yaml (requires restart).
func (a *App) SetPort(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("invalid port: %d", port)
	}
	host := a.listenHost()
	current := a.currentPort()
	if port != current && !localconfig.IsPortAvailable(host, port) {
		return fmt.Errorf("port %d is already in use on %s", port, host)
	}
	path := filepath.Join(a.dataDir, setup.ConfigFileName)
	if err := localconfig.UpdateServerPort(path, port); err != nil {
		return err
	}
	cfg, err := config.LoadForBootstrap()
	if err != nil {
		return err
	}
	a.cfg = cfg
	return nil
}

func (a *App) prepareListenPort() error {
	if a.cfg == nil {
		return fmt.Errorf("config not loaded")
	}
	host := a.listenHost()
	preferred := a.currentPort()
	configPath := filepath.Join(a.dataDir, setup.ConfigFileName)
	port, switchedFrom, err := localconfig.ResolveListenPort(host, preferred, true, configPath)
	if err != nil {
		return err
	}
	if switchedFrom > 0 && port != switchedFrom {
		a.portSwitchMessage = fmt.Sprintf(
			"配置的端口 %d 已被占用，已自动改用 %d。\n新端口已写入 config.yaml，可在「本地设置」中修改。",
			switchedFrom, port,
		)
	}
	cfg, err := config.LoadForBootstrap()
	if err != nil {
		return err
	}
	a.cfg = cfg
	if cfg.Server.Port > 0 && a.ctx != nil {
		wailsruntime.WindowExecJS(a.ctx, fmt.Sprintf(
			"if (history.replaceState) { history.replaceState(null, '', '?port=%d'); }",
			cfg.Server.Port,
		))
	}
	return nil
}

func (a *App) listenHost() string {
	if a.cfg != nil && a.cfg.Server.Host != "" {
		return a.cfg.Server.Host
	}
	return "127.0.0.1"
}

func (a *App) currentPort() int {
	if a.cfg != nil && a.cfg.Server.Port > 0 {
		return a.cfg.Server.Port
	}
	return localconfig.DefaultHTTPPort
}

// OpenDataDir opens the data directory in the OS file manager.
func (a *App) OpenDataDir() error {
	var cmd *exec.Cmd
	switch goruntime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", a.dataDir)
	case "darwin":
		cmd = exec.Command("open", a.dataDir)
	default:
		cmd = exec.Command("xdg-open", a.dataDir)
	}
	return cmd.Start()
}

// GetDefaultAPIKey exposes the auto-generated API key to the UI.
func (a *App) GetDefaultAPIKey() string {
	return repository.GetDefaultAPIKeyPlaintext()
}

// ParsePort is a helper for the frontend.
func (a *App) ParsePort(portStr string) (int, error) {
	return strconv.Atoi(portStr)
}

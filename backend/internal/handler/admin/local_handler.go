package admin

import (
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/localconfig"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/setup"

	"github.com/gin-gonic/gin"
)

// LocalHandler exposes desktop-only runtime information to the admin UI.
type LocalHandler struct {
	cfg *config.Config
}

func NewLocalHandler(cfg *config.Config) *LocalHandler {
	return &LocalHandler{cfg: cfg}
}

type localInfoResponse struct {
	DataDir         string `json:"data_dir"`
	DefaultAPIKey   string `json:"default_api_key"`
	AdminPassword   string `json:"generated_admin_password,omitempty"`
	ServerHost      string `json:"server_host"`
	ServerPort      int    `json:"server_port"`
	RunMode         string `json:"run_mode"`
	ConfigPath      string `json:"config_path"`
}

type localPortCheckResponse struct {
	Port        int    `json:"port"`
	Available   bool   `json:"available"`
	CurrentPort int    `json:"current_port"`
	Host        string `json:"host"`
	Message     string `json:"message,omitempty"`
}

type localPortUpdateRequest struct {
	Port int `json:"port" binding:"required"`
}

type localPortUpdateResponse struct {
	Port         int    `json:"port"`
	NeedRestart  bool   `json:"need_restart"`
	ConfigPath   string `json:"config_path"`
	Message      string `json:"message"`
}

// GetLocalInfo returns local-mode bootstrap secrets and paths (admin only).
func (h *LocalHandler) GetLocalInfo(c *gin.Context) {
	if h.cfg == nil || !h.cfg.IsLocalMode() {
		response.Forbidden(c, "local info is only available in local run mode")
		return
	}
	response.Success(c, localInfoResponse{
		DataDir:       setup.GetDataDir(),
		DefaultAPIKey: repository.GetDefaultAPIKeyPlaintext(),
		AdminPassword: repository.GetGeneratedAdminPassword(),
		ServerHost:    h.cfg.Server.Host,
		ServerPort:    h.cfg.Server.Port,
		RunMode:       h.cfg.RunMode,
		ConfigPath:    filepath.Join(setup.GetDataDir(), setup.ConfigFileName),
	})
}

// CheckLocalPort reports whether a TCP port is available on the configured host.
func (h *LocalHandler) CheckLocalPort(c *gin.Context) {
	if h.cfg == nil || !h.cfg.IsLocalMode() {
		response.Forbidden(c, "local port check is only available in local run mode")
		return
	}
	port, err := strconv.Atoi(c.Query("port"))
	if err != nil || port < 1 || port > 65535 {
		response.BadRequest(c, "invalid port")
		return
	}
	host := h.cfg.Server.Host
	if host == "" {
		host = "127.0.0.1"
	}
	current := h.cfg.Server.Port
	if current <= 0 {
		current = localconfig.DefaultHTTPPort
	}
	available := port == current || localconfig.IsPortAvailable(host, port)
	msg := ""
	if !available {
		msg = "端口已被其他程序占用"
	}
	response.Success(c, localPortCheckResponse{
		Port:        port,
		Available:   available,
		CurrentPort: current,
		Host:        host,
		Message:     msg,
	})
}

// UpdateLocalPort writes server.port to config.yaml (restart required).
func (h *LocalHandler) UpdateLocalPort(c *gin.Context) {
	if h.cfg == nil || !h.cfg.IsLocalMode() {
		response.Forbidden(c, "local port update is only available in local run mode")
		return
	}
	var req localPortUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}
	if req.Port < 1 || req.Port > 65535 {
		response.BadRequest(c, "invalid port")
		return
	}
	host := h.cfg.Server.Host
	if host == "" {
		host = "127.0.0.1"
	}
	current := h.cfg.Server.Port
	if current <= 0 {
		current = localconfig.DefaultHTTPPort
	}
	if req.Port != current && !localconfig.IsPortAvailable(host, req.Port) {
		response.ErrorWithDetails(c, http.StatusConflict, "port is already in use", "port_in_use", nil)
		return
	}
	configPath := filepath.Join(setup.GetDataDir(), setup.ConfigFileName)
	if err := localconfig.UpdateServerPort(configPath, req.Port); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, localPortUpdateResponse{
		Port:        req.Port,
		NeedRestart: true,
		ConfigPath:  configPath,
		Message:     "端口已保存，请重启桌面应用后生效",
	})
}

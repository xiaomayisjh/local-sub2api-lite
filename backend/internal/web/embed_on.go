//go:build embed

package web

import (
	"context"
	"embed"
	"io/fs"
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

//go:embed all:dist
var frontendFS embed.FS

// PublicSettingsProvider is an interface to fetch public settings
type PublicSettingsProvider interface {
	GetPublicSettingsForInjection(ctx context.Context) (any, error)
}

// FrontendServer serves the embedded frontend with settings injection
type FrontendServer struct {
	*frontendServerCore
}

// NewFrontendServer creates a new frontend server with settings injection
func NewFrontendServer(settingsProvider PublicSettingsProvider) (*FrontendServer, error) {
	distFS, err := fs.Sub(frontendFS, "dist")
	if err != nil {
		return nil, err
	}

	core, err := newFrontendServerCore(distFS, settingsProvider, filepath.Join("data", "public"))
	if err != nil {
		return nil, err
	}
	return &FrontendServer{frontendServerCore: core}, nil
}

func (s *FrontendServer) InvalidateCache() {
	if s == nil || s.frontendServerCore == nil {
		return
	}

	s.frontendServerCore.InvalidateCache()
}

// ServeEmbeddedFrontend returns a middleware for serving embedded frontend
// This is the legacy function for backward compatibility when no settings provider is available
func ServeEmbeddedFrontend() gin.HandlerFunc {
	distFS, err := fs.Sub(frontendFS, "dist")
	if err != nil {
		panic("failed to get dist subdirectory: " + err.Error())
	}
	fileServer := http.FileServer(http.FS(distFS))
	overrideDir := filepath.Join("data", "public")

	return serveFrontendAssets(distFS, fileServer, overrideDir)
}

func HasEmbeddedFrontend() bool {
	_, err := frontendFS.ReadFile("dist/index.html")
	return err == nil
}

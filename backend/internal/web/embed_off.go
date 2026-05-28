//go:build !embed

// Package web provides web assets for the application.
package web

import (
	"context"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

// PublicSettingsProvider is an interface to fetch public settings.
type PublicSettingsProvider interface {
	GetPublicSettingsForInjection(ctx context.Context) (any, error)
}

// FrontendServer serves the built frontend from internal/web/dist in non-embed builds.
type FrontendServer struct {
	*frontendServerCore
}

// NewFrontendServer creates a filesystem-backed frontend server for local source builds.
func NewFrontendServer(settingsProvider PublicSettingsProvider) (*FrontendServer, error) {
	distFS := os.DirFS(frontendDistDir())
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

func frontendDistDir() string {
	candidates := []string{
		filepath.Join("internal", "web", "dist"),
		filepath.Join("backend", "internal", "web", "dist"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(filepath.Join(candidate, "index.html")); err == nil {
			return candidate
		}
	}
	return candidates[0]
}

// ServeEmbeddedFrontend returns a middleware for serving frontend assets without settings injection.
func ServeEmbeddedFrontend() gin.HandlerFunc {
	distFS := os.DirFS(frontendDistDir())
	fileServer := http.FileServer(http.FS(distFS))
	overrideDir := filepath.Join("data", "public")
	return serveFrontendAssets(distFS, fileServer, overrideDir)
}

func HasEmbeddedFrontend() bool {
	_, err := fs.Stat(os.DirFS(frontendDistDir()), "index.html")
	return err == nil
}

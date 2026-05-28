package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type coreTestSettingsProvider struct{}

func (coreTestSettingsProvider) GetPublicSettingsForInjection(context.Context) (any, error) {
	return map[string]string{"site_name": "Core Test"}, nil
}

func TestFrontendServerCoreServesSPARoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dist := fstest.MapFS{
		"index.html": {
			Data: []byte(`<!doctype html><html><head><title>Sub2API</title></head><body><div id="app"></div></body></html>`),
		},
		"assets/app.js": {
			Data: []byte(`console.log("ok")`),
		},
	}
	core, err := newFrontendServerCore(dist, coreTestSettingsProvider{}, "")
	require.NoError(t, err)

	router := gin.New()
	router.Use(core.Middleware())
	router.GET("/api/v1/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	for _, path := range []string{"/", "/login", "/admin/accounts"} {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, path, nil)
			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code)
			require.Contains(t, rec.Header().Get("Content-Type"), "text/html")
			require.Contains(t, rec.Body.String(), "window.__APP_CONFIG__=")
			require.Contains(t, rec.Body.String(), "<title>Core Test - AI API Gateway</title>")
		})
	}

	t.Run("serves static assets", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		require.Contains(t, rec.Body.String(), `console.log("ok")`)
	})

	t.Run("bypasses api routes", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/ping", nil)
		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, "pong", rec.Body.String())
	})
}

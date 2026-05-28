package main

import (
	"embed"
	"log"
	"os"
	"strings"

	_ "github.com/Wei-Shaw/sub2api/ent/runtime" // ent schema defaults (created_at, etc.)
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	// IsDebugBuild 在 desktop_debug.go / desktop_release.go 中按构建 tag 设值。
	// debug 构建默认启用：DevTools、上下文菜单、详细日志；无需环境变量。
	// 用户仍可通过环境变量在 release 构建里临时打开（用于排查问题）。
	openDevTools := IsDebugBuild || envBool("SUB2API_DESKTOP_OPEN_DEVTOOLS")
	enableContextMenu := IsDebugBuild || envBool("SUB2API_DESKTOP_DEBUG")

	err := wails.Run(&options.App{
		Title:            buildWindowTitle(),
		Width:            1280,
		Height:           800,
		BackgroundColour: &options.RGBA{R: 15, G: 23, B: 42, A: 255},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		EnableDefaultContextMenu: enableContextMenu,
		Debug: options.Debug{
			OpenInspectorOnStartup: openDevTools,
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
		},
		OnStartup:  app.startup,
		OnDomReady: app.domReady,
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}

func buildWindowTitle() string {
	if IsDebugBuild {
		return "Sub2API Local (Debug)"
	}
	return "Sub2API Local"
}

func envBool(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

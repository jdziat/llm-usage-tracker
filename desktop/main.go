package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var embeddedFS embed.FS

func main() {
	svc := &UsageService{}

	err := wails.Run(&options.App{
		Title:  "llmut",
		Width:  1100,
		Height: 720,
		AssetServer: &assetserver.Options{
			Assets: embeddedFS,
		},
		OnStartup:  svc.OnStartup,
		OnShutdown: svc.OnShutdown,
		Bind:       []interface{}{svc},
	})
	if err != nil {
		println("error:", err.Error())
	}
}

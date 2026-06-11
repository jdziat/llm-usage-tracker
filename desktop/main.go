package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var embeddedFS embed.FS

func main() {
	app := application.New(application.Options{
		Name: "llmut",
		Services: []application.Service{
			application.NewService(&UsageService{}),
		},
		Assets: application.AssetOptions{
			Handler: application.BundledAssetFileServer(embeddedFS),
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "llmut",
		Width:  1100,
		Height: 720,
		URL:    "/",
	})

	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		startUsageWatch(app)
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

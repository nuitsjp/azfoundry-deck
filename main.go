package main

import (
	"embed"
	"log"
	"os"

	"github.com/nuitsjp/azfoundry-deck/internal/service"
	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if os.Getenv("AZFOUNDRY_MOCK") != "1" {
		log.Fatal("現在は互換性確認用のモックのみです。mise run mock で起動してください。")
	}

	app := application.New(application.Options{
		Name: "AzFoundry Deck",
		Services: []application.Service{
			application.NewService(&service.CompatibilityService{}),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
	})
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "AzFoundry Deck — 互換性確認用モック",
		Width:  960,
		Height: 600,
		URL:    "/",
	})
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

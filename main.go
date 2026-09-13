package main

import (
	"embed"
	"log"
	"os"
	"time"

	"github.com/nuitsjp/azfoundry-deck/internal/mock"
	"github.com/nuitsjp/azfoundry-deck/internal/service"
	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func init() {
	application.RegisterEvent[service.DeploymentProgress](service.DeploymentProgressEvent)
}

func main() {
	if os.Getenv("AZFOUNDRY_MOCK") != "1" {
		log.Fatal("F1モックのみ利用可能で、実接続は未実装です。mise run mock で起動してください。")
	}

	provider := mock.NewProvider()
	emitProgress := func(progress service.DeploymentProgress) {
		application.Get().Event.Emit(service.DeploymentProgressEvent, progress)
	}
	app := application.New(application.Options{
		Name: "AzFoundry Deck",
		// Allow the 30-second loading scenario to finish in browser mode.
		Server: application.ServerOptions{WriteTimeout: 45 * time.Second},
		Services: []application.Service{
			application.NewService(service.NewDeploymentService(provider.Fetch, true, emitProgress)),
			application.NewService(mock.NewMockService(provider)),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
	})
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "AzFoundry Deck — F1モック",
		Width:     1440,
		Height:    960,
		MinWidth:  1080,
		MinHeight: 720,
		URL:       "/",
	})
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

package main

import (
	"embed"
	"log"
	"os"
	"time"

	"github.com/nuitsjp/azfoundry-deck/internal/azurego"
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
	emitProgress := func(progress service.DeploymentProgress) {
		application.Get().Event.Emit(service.DeploymentProgressEvent, progress)
	}
	services := make([]application.Service, 0, 5)
	title := "AzFoundry Deck"
	if os.Getenv("AZFOUNDRY_MOCK") == "1" {
		provider := mock.NewProvider()
		services = append(services,
			application.NewService(service.NewDeploymentService(provider.Fetch, true, emitProgress)),
			application.NewService(service.NewModelService(provider.FetchModels, provider.FetchRegionModels, true)),
			application.NewService(service.NewPlacementService(provider.FetchPlacements, provider.FetchDeploymentNames, true)),
			application.NewService(service.NewCreationService(provider.CreateModelDeployment, provider.CheckCreateOperation, provider.PendingCreateOperations, true)),
			application.NewService(mock.NewMockService(provider)),
		)
		title = "AzFoundry Deck — F1/F2モック"
	} else {
		provider := azurego.NewProvider()
		services = append(services,
			application.NewService(service.NewDeploymentService(provider.Fetch, false, emitProgress)),
			application.NewService(service.NewModelService(provider.FetchModels, provider.FetchRegionModels, false)),
			application.NewService(service.NewPlacementService(provider.FetchPlacements, provider.FetchDeploymentNames, false)),
			application.NewService(service.NewCreationService(provider.CreateModelDeployment, provider.CheckCreateOperation, provider.PendingCreateOperations, false)),
		)
	}

	app := application.New(application.Options{
		Name: "AzFoundry Deck",
		// Allow the 30-second loading scenario to finish in browser mode.
		Server:   application.ServerOptions{WriteTimeout: 45 * time.Second},
		Services: services,
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
	})
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     title,
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

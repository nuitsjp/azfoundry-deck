package main

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"azfoundrydeck/internal/azauth"
	"azfoundrydeck/internal/desktop"
	"azfoundrydeck/internal/diagnostics"
	"azfoundrydeck/internal/fault"
	"azfoundrydeck/internal/foundry"
	"azfoundrydeck/internal/updates"
)

//go:embed all:frontend/dist
var webAssets embed.FS

//go:embed build/app.json
var configJSON []byte

// Set only through -ldflags -X (see scripts/build.mjs) to build an older version or a
// local update source for checking the update. Empty keeps build/app.json.
var buildVersion, buildUpdateSource, buildUpdatePublicKey string

type appConfig struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Executable string `json:"executable"`
	Version    string `json:"version"`
	// The GitHub Releases URL of update.json and the base64 Ed25519 key that verifies it.
	UpdateSource    string `json:"updateSource"`
	UpdatePublicKey string `json:"updatePublicKey"`
}

// updateBoundary is what main hands to the update service besides build/app.json:
// how to start the installer, and, for screen review only, a delay before the
// check and an action after staging.
type updateBoundary struct {
	launch func(string) error
	delay  time.Duration
	staged func() error
}

func init() {
	application.RegisterEvent[foundry.Progress](foundry.ProgressEvent)
	application.RegisterEvent[string](foundry.CapacityReadyEvent)
	application.RegisterEvent[string](foundry.ConnectionReadyEvent)
	application.RegisterEvent[string](foundry.CostReadyEvent)
	application.RegisterEvent[foundry.FoundryCreateProgress](foundry.FoundryCreateProgressEvent)
	application.RegisterEvent[foundry.FoundryDeleteProgress](foundry.FoundryDeleteProgressEvent)
	application.RegisterEvent[updates.Status](updates.ProgressEvent)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "起動できません:", err)
		showStartupFailure()
		os.Exit(1)
	}
}
func run() error {
	var cfg appConfig
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return err
	}
	for target, value := range map[*string]string{&cfg.Version: buildVersion, &cfg.UpdateSource: buildUpdateSource, &cfg.UpdatePublicKey: buildUpdatePublicKey} {
		if value != "" {
			*target = value
		}
	}
	if cfg.ID == "" || cfg.Name == "" {
		return fmt.Errorf("build/app.json: id and name are required")
	}
	dir := os.Getenv("WAILS_DATA_DIR")
	if dir == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return err
		}
		dir = filepath.Join(base, cfg.ID)
	}
	if !filepath.IsAbs(dir) {
		return fmt.Errorf("WAILS_DATA_DIR must be absolute")
	}
	logger, logs, err := diagnostics.Open(filepath.Join(dir, "logs"), production)
	diagnosticsAvailable := err == nil
	if err != nil {
		// Diagnostics must not prevent useful operation. This fallback is
		// explicit and visible; it never substitutes business data.
		logger = slog.Default()
		logger.Warn("診断ログを保存できません。標準エラー出力を使用します。", "cause", err)
	} else {
		defer logs.Close() //nolint:errcheck // Shutdown cannot report a closing failure through the same diagnostic log.
	}
	logger.Info("starting", "version", cfg.Version, "os", runtime.GOOS, "arch", runtime.GOARCH, "server", serverMode)
	root, err := fs.Sub(webAssets, "frontend/dist")
	if err != nil {
		return err
	}
	port, err := serverPort()
	if err != nil {
		return err
	}
	var app *application.App
	var window *application.WebviewWindow
	emit := func(name string, data any) {
		if app != nil {
			app.Event.Emit(name, data)
		}
	}
	authStore := recordStore(dir)
	clearViews := func() error {
		for _, name := range []string{"azure-views", "foundry-state.json", "foundry-models"} {
			target := filepath.Join(dir, name)
			if filepath.Dir(target) != filepath.Clean(dir) {
				return fmt.Errorf("invalid saved data path")
			}
			if err := os.RemoveAll(target); err != nil {
				return err
			}
		}
		return nil
	}
	operations := &sync.Mutex{}
	var foundryService *foundry.Service
	authService := azauth.New(authStore, logger, clearViews, operations, func(discard bool) { foundry.StopView(foundryService, discard) })
	foundryService = foundry.New(operations, func() (foundry.Source, error) { return foundrySource(authStore) }, func(ctx context.Context) error {
		status, err := authService.GetStatus(ctx)
		if err != nil {
			return err
		}
		if status.Phase != azauth.SignedIn || status.Account == nil || status.Account.SelectedTenantID == "" {
			return fault.New("NOT_SIGNED_IN", "ログインしていません。")
		}
		return nil
	}, func() (string, error) {
		record, found, err := authStore.Load()
		if err != nil {
			return "", err
		}
		if !found || record.SelectedTenantID == "" {
			return "", fmt.Errorf("selected tenant is missing")
		}
		identity, err := json.Marshal([]string{record.Record.HomeAccountID, record.SelectedTenantID})
		if err != nil {
			return "", err
		}
		key := sha256.Sum256(identity)
		return filepath.Join(dir, "azure-views", fmt.Sprintf("%x", key), "foundry-state.json"), nil
	}, logger, emit)
	info := desktop.Info{Name: cfg.Name, Version: cfg.Version, AppID: cfg.ID, Server: serverMode, DiagnosticsAvailable: diagnosticsAvailable}
	appService := desktop.New(info, logger)
	updateConfig := updates.Config{AppID: cfg.ID, Version: cfg.Version, Arch: runtime.GOARCH, Source: cfg.UpdateSource, PublicKey: cfg.UpdatePublicKey, CacheDir: filepath.Join(dir, "updates")}
	boundary, err := updateSource(dir, &updateConfig, logger)
	if err != nil {
		return err
	}
	updateService := updates.New(updateConfig, logger, emit, boundary.launch, func() { app.Quit() })
	options := application.Options{
		Name: cfg.Name, Description: "Azure Foundry 管理用デスクトップアプリ", Logger: logger,
		Assets:       application.AssetOptions{Handler: application.BundledAssetFileServer(root), DisableLogging: true},
		Services:     []application.Service{application.NewService(authService), application.NewService(appService), application.NewService(foundryService), application.NewService(updateService)},
		MarshalError: fault.Marshal,
		// ARM creation can outlast Wails' default 30-second response deadline.
		Server:  application.ServerOptions{Host: "127.0.0.1", Port: port, WriteTimeout: -1},
		Windows: application.WindowsOptions{WebviewUserDataPath: filepath.Join(dir, "webview")},
	}
	if !serverMode {
		// A deterministic per-product key, not a secret or an updater key.
		key := sha256.Sum256([]byte(cfg.ID + ":single-instance"))
		options.SingleInstance = &application.SingleInstanceOptions{UniqueID: cfg.ID, EncryptionKey: key, OnSecondInstanceLaunch: func(application.SecondInstanceData) {
			if window != nil {
				window.Show()
				window.Restore()
				window.Focus()
			}
		}}
	}
	app = application.New(options)
	if !serverMode {
		window = app.Window.NewWithOptions(application.WebviewWindowOptions{Title: cfg.Name, Width: 1160, Height: 800, Frameless: true, URL: "/"})
	}
	if updateConfig.Enabled {
		// Apart from startup, so neither a slow network nor a failure delays the window.
		go func() {
			time.Sleep(boundary.delay)
			updates.Run(context.Background(), updateService)
			if boundary.staged != nil {
				if err := boundary.staged(); err != nil {
					logger.Warn("update_review_staged_failed", "cause", err)
				}
			}
		}()
	}
	return app.Run()
}
func serverPort() (int, error) {
	if s := os.Getenv("WAILS_SERVER_PORT"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 && n < 65536 {
			return n, nil
		}
		return 0, fmt.Errorf("invalid WAILS_SERVER_PORT")
	}
	return 34115, nil
}

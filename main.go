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

	"github.com/wailsapp/wails/v3/pkg/application"

	"azfoundrydeck/internal/azauth"
	"azfoundrydeck/internal/desktop"
	"azfoundrydeck/internal/diagnostics"
	"azfoundrydeck/internal/fault"
	"azfoundrydeck/internal/foundry"
)

//go:embed all:frontend/dist
var webAssets embed.FS

//go:embed build/app.json
var configJSON []byte

type appConfig struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Executable string `json:"executable"`
	Version    string `json:"version"`
}

func init() {
	application.RegisterEvent[foundry.Progress](foundry.ProgressEvent)
	application.RegisterEvent[foundry.FoundryCreateProgress](foundry.FoundryCreateProgressEvent)
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
		defer logs.Close()
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
	authService := azauth.New(authStore, logger, clearViews, operations)
	foundryService := foundry.New(operations, func() (foundry.Source, error) { return foundrySource(authStore) }, func(ctx context.Context) error {
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
	options := application.Options{
		Name: cfg.Name, Description: "Azure Foundry 管理用デスクトップアプリ", Logger: logger,
		Assets:       application.AssetOptions{Handler: application.BundledAssetFileServer(root), DisableLogging: true},
		Services:     []application.Service{application.NewService(authService), application.NewService(appService), application.NewService(foundryService)},
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

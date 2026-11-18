// Package settings persists application-wide display preferences.
package settings

import (
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"azfoundrydeck/internal/fault"
)

type ThemeMode string

const (
	ThemeDark  ThemeMode = "dark"
	ThemeLight ThemeMode = "light"
)

type settingsFile struct {
	Theme ThemeMode `json:"theme"`
}

type Service struct {
	mu     sync.Mutex
	path   string
	logger *slog.Logger
}

func New(path string, logger *slog.Logger) *Service {
	return &Service{path: path, logger: logger}
}

// GetTheme returns the saved theme, or ThemeDark when settings.json does not exist yet.
func (s *Service) GetTheme() (ThemeMode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ThemeDark, nil
		}
		s.logger.Error("settings_load_failed", "cause", err)
		return "", fault.New("THEME_LOAD_FAILED", "設定ファイルを読み込めませんでした。")
	}

	var file settingsFile
	if err := json.Unmarshal(data, &file); err != nil {
		s.logger.Error("settings_load_failed", "cause", err)
		return "", fault.New("THEME_LOAD_FAILED", "設定ファイルの形式が不正です。")
	}
	if file.Theme != ThemeDark && file.Theme != ThemeLight {
		s.logger.Error("settings_load_failed", "theme", file.Theme)
		return "", fault.New("THEME_LOAD_FAILED", "設定ファイルのテーマ値が不正です。")
	}
	return file.Theme, nil
}

// SetTheme validates and atomically writes the selected theme to settings.json.
func (s *Service) SetTheme(theme ThemeMode) (ThemeMode, error) {
	if theme != ThemeDark && theme != ThemeLight {
		return "", fault.New("THEME_SAVE_FAILED", "不正なテーマが指定されました。")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := saveJSON(s.path, settingsFile{Theme: theme}); err != nil {
		s.logger.Error("settings_save_failed", "cause", err)
		return "", fault.New("THEME_SAVE_FAILED", "設定ファイルを保存できませんでした。")
	}
	return theme, nil
}

func saveJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".settings-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name()) //nolint:errcheck // After rename the temporary file is absent; on failure preserve the original error.
	if _, err := file.Write(data); err != nil {
		file.Close() //nolint:errcheck // Preserve the Write failure while releasing the temporary file.
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close() //nolint:errcheck // Preserve the Sync failure while releasing the temporary file.
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

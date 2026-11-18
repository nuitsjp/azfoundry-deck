package settings

import (
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"azfoundrydeck/internal/fault"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func assertFaultCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error %s, got nil", code)
	}
	var target *fault.Error
	if !errors.As(err, &target) {
		t.Fatalf("expected *fault.Error, got %T: %v", err, err)
	}
	if target.Code != code {
		t.Fatalf("expected fault code %q, got %q (%s)", code, target.Code, target.Message)
	}
}

func TestGetThemeDefaultsToDarkWithoutCreatingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	svc := New(path, testLogger())

	got, err := svc.GetTheme()
	if err != nil {
		t.Fatalf("GetTheme returned unexpected error: %v", err)
	}
	if got != ThemeDark {
		t.Fatalf("expected %q, got %q", ThemeDark, got)
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected settings.json not to be created, stat err=%v", statErr)
	}
}

func TestSetThemeAndGetThemeRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	svc := New(path, testLogger())

	saved, err := svc.SetTheme(ThemeLight)
	if err != nil {
		t.Fatalf("SetTheme(light) failed: %v", err)
	}
	if saved != ThemeLight {
		t.Fatalf("expected %q, got %q", ThemeLight, saved)
	}

	got, err := svc.GetTheme()
	if err != nil {
		t.Fatalf("GetTheme failed: %v", err)
	}
	if got != ThemeLight {
		t.Fatalf("expected %q, got %q", ThemeLight, got)
	}
}

func TestGetThemeReturnsLoadFailedOnInvalidContent(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
	}{
		{name: "broken json", content: "{broken"},
		{name: "invalid theme value", content: `{"theme":"solarized"}`},
		{name: "empty theme value", content: `{}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")
			if err := os.WriteFile(path, []byte(tc.content), 0600); err != nil {
				t.Fatalf("write failed: %v", err)
			}
			svc := New(path, testLogger())

			_, err := svc.GetTheme()
			assertFaultCode(t, err, "THEME_LOAD_FAILED")
		})
	}
}

func TestSetThemeRejectsInvalidThemeAndPreservesExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	svc := New(path, testLogger())

	if _, err := svc.SetTheme(ThemeLight); err != nil {
		t.Fatalf("initial SetTheme failed: %v", err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	_, err = svc.SetTheme(ThemeMode("blue"))
	assertFaultCode(t, err, "THEME_SAVE_FAILED")

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(after) != string(before) {
		t.Fatalf("expected file content to stay %q, got %q", string(before), string(after))
	}
}

func TestSetThemeReturnsSaveFailedWhenTargetIsDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.MkdirAll(filepath.Join(path, "nested"), 0700); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	svc := New(path, testLogger())

	_, err := svc.SetTheme(ThemeLight)
	assertFaultCode(t, err, "THEME_SAVE_FAILED")
}

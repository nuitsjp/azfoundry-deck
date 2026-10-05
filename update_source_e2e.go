//go:build e2e

package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"azfoundrydeck/internal/updates"
)

// updateReviewDelay leaves time to look at Home before the update section appears.
const updateReviewDelay = 5 * time.Second

// updateSource stands in for GitHub Releases when AZFOUNDRYDECK_E2E_UPDATE is set:
// a signed v0.2.0 in a local folder, whose installer is never executed.
// =untrusted alters the staged installer so that the re-verification fails.
func updateSource(dir string, cfg *updates.Config, logger *slog.Logger) (updateBoundary, error) {
	mode := os.Getenv("AZFOUNDRYDECK_E2E_UPDATE")
	if mode == "" {
		return updateBoundary{launch: updates.LaunchInstaller}, nil
	}
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return updateBoundary{}, err
	}
	source := filepath.Join(dir, "e2e-release")
	if err = os.MkdirAll(source, 0o700); err != nil {
		return updateBoundary{}, err
	}
	installer := []byte("e2e installer - never executed")
	hash := sha256.Sum256(installer)
	m := updates.Manifest{AppID: cfg.AppID, Version: "0.2.0", OS: "windows", Arch: cfg.Arch, Filename: "azfoundrydeck-0.2.0-amd64-setup.exe", Size: int64(len(installer)), SHA256: hex.EncodeToString(hash[:])}
	if err = os.WriteFile(filepath.Join(source, m.Filename), installer, 0o600); err != nil {
		return updateBoundary{}, err
	}
	signed, err := updates.Sign(m, key)
	if err != nil {
		return updateBoundary{}, err
	}
	if err = os.WriteFile(filepath.Join(source, updates.ManifestName), signed, 0o600); err != nil {
		return updateBoundary{}, err
	}
	cfg.Source, cfg.PublicKey, cfg.Enabled = source, base64.StdEncoding.EncodeToString(pub), true
	boundary := updateBoundary{delay: updateReviewDelay, launch: func(path string) error {
		logger.Info("e2e_installer_not_executed", "file", filepath.Base(path))
		return nil
	}}
	if mode == "untrusted" {
		boundary.staged = func() error {
			staged, err := filepath.Glob(filepath.Join(cfg.CacheDir, "update-*", m.Filename))
			if err != nil || len(staged) != 1 {
				return fmt.Errorf("staged installer not found: %v", err)
			}
			return os.WriteFile(staged[0], []byte("altered"), 0o600)
		}
	}
	return boundary, nil
}

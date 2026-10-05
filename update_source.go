//go:build !e2e

package main

import (
	"log/slog"

	"azfoundrydeck/internal/updates"
)

// updateSource keeps the source and key of build/app.json. Only the installed
// desktop build checks; dev builds and the browser server never do.
func updateSource(_ string, cfg *updates.Config, _ *slog.Logger) (updateBoundary, error) {
	cfg.Enabled = production && !serverMode
	return updateBoundary{launch: updates.LaunchInstaller}, nil
}

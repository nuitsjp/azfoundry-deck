//go:build e2e

package main

import (
	"path/filepath"

	"azfoundrydeck/internal/azauth"
)

// E2E builds save the record in the per-test data directory instead of the Credential Manager.
func recordStore(dataDir string) azauth.RecordStore {
	return azauth.FileStore{Path: filepath.Join(dataDir, "e2e-authentication-record.json")}
}

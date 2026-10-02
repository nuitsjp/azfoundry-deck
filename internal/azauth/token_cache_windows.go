//go:build !e2e

package azauth

import (
	"errors"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// deleteTokenCache removes the persistent token cache with the given name. azidentity/cache
// v0.4.0 has no delete API; on Windows it stores each cache as one DPAPI file at
// <LocalAppData>\.IdentityService\<name>, and CAE tokens under <name>.cae
// (cache.go cacheFilePath, cache_windows.go cacheDir and storage). Files that
// are already gone are not an error.
func deleteTokenCache(name string) error {
	dir, err := windows.KnownFolderPath(windows.FOLDERID_LocalAppData, 0)
	if err != nil {
		return err
	}
	for _, file := range []string{name, name + ".cae"} {
		err := os.Remove(filepath.Join(dir, ".IdentityService", file))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

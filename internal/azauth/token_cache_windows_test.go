//go:build !e2e

package azauth

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// deleteTokenCache works on real files under LocalAppData, which E2E cannot
// reach. It uses a test-only cache name and never touches tokenCacheName.
func TestDeleteTokenCacheRemovesFilesAndIsIdempotent(t *testing.T) {
	dir, err := windows.KnownFolderPath(windows.FOLDERID_LocalAppData, 0)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("azfoundrydeck-test-%d", time.Now().UnixNano())
	files := []string{
		filepath.Join(dir, ".IdentityService", name),
		filepath.Join(dir, ".IdentityService", name+".cae"),
	}
	t.Cleanup(func() {
		for _, f := range files {
			_ = os.Remove(f)
		}
	})
	if err := os.MkdirAll(filepath.Dir(files[0]), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if err := os.WriteFile(f, []byte("test"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if err := deleteTokenCache(name); err != nil {
		t.Fatalf("deleteTokenCache: %v", err)
	}
	for _, f := range files {
		if _, err := os.Stat(f); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s remains: %v", f, err)
		}
	}
	if err := deleteTokenCache(name); err != nil {
		t.Errorf("deleteTokenCache on missing files: %v", err)
	}
}

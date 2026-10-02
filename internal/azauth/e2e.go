//go:build e2e

package azauth

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

// E2E builds replace only the external boundaries (Entra ID / ARM and the
// Credential Manager). AZFOUNDRYDECK_E2E_FAIL=signin, =save or =restore injects
// a failure. A record file placed before start makes the startup restore sign in silently.
// Every sign-in leaves the e2e-signin-called file in WAILS_DATA_DIR, and
// AZFOUNDRYDECK_E2E_HOLD_RESTORE=1 holds the restore until the e2e-restore-release
// file appears there. Production builds never compile this file.

func signIn(context.Context) (Account, azidentity.AuthenticationRecord, error) {
	if err := os.WriteFile(filepath.Join(os.Getenv("WAILS_DATA_DIR"), "e2e-signin-called"), nil, 0o600); err != nil {
		return Account{}, azidentity.AuthenticationRecord{}, err
	}
	if os.Getenv("AZFOUNDRYDECK_E2E_FAIL") == "signin" {
		return Account{}, azidentity.AuthenticationRecord{}, errors.New("e2e: sign-in failure injected")
	}
	record := azidentity.AuthenticationRecord{
		Authority:     "login.microsoftonline.com",
		ClientID:      "e2e-client",
		HomeAccountID: "e2e-object.e2e-tenant",
		TenantID:      "e2e-tenant",
		Username:      "operator@contoso.onmicrosoft.com",
		Version:       "1.0",
	}
	return Account{Username: record.Username, TenantName: "Contoso"}, record, nil
}

func restoreAccount(ctx context.Context, record azidentity.AuthenticationRecord) (Account, error) {
	if os.Getenv("AZFOUNDRYDECK_E2E_HOLD_RESTORE") == "1" {
		release := filepath.Join(os.Getenv("WAILS_DATA_DIR"), "e2e-restore-release")
		for {
			if _, err := os.Stat(release); err == nil {
				break
			}
			select {
			case <-ctx.Done():
				return Account{}, ctx.Err()
			case <-time.After(50 * time.Millisecond):
			}
		}
	}
	if os.Getenv("AZFOUNDRYDECK_E2E_FAIL") == "restore" {
		return Account{}, errors.New("e2e: restore failure injected")
	}
	return Account{Username: record.Username, TenantName: "Contoso"}, nil
}

// FileStore writes the record as the same JSON the Credential Manager store uses.
type FileStore struct {
	Path string
}

func (f FileStore) Save(record azidentity.AuthenticationRecord) error {
	if os.Getenv("AZFOUNDRYDECK_E2E_FAIL") == "save" {
		return errors.New("e2e: save failure injected")
	}
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return os.WriteFile(f.Path, data, 0o600)
}

func (f FileStore) Load() (azidentity.AuthenticationRecord, bool, error) {
	var record azidentity.AuthenticationRecord
	data, err := os.ReadFile(f.Path)
	if errors.Is(err, os.ErrNotExist) {
		return record, false, nil
	}
	if err != nil {
		return record, false, err
	}
	return record, true, json.Unmarshal(data, &record)
}

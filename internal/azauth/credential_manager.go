package azauth

import (
	"encoding/json"
	"errors"

	"github.com/zalando/go-keyring"
)

// The record is stored as one generic credential whose target name is
// "AzFoundryDeck:AuthenticationRecord" and whose blob is the record's JSON.
// It holds account identifiers only, no password or token.
const (
	credentialService = "AzFoundryDeck"
	credentialUser    = "AuthenticationRecord"
)

// CredentialManager stores the AuthenticationRecord in the Windows Credential Manager.
type CredentialManager struct{}

func (CredentialManager) Save(record LoginRecord) error {
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return keyring.Set(credentialService, credentialUser, string(data))
}

func (CredentialManager) Load() (LoginRecord, bool, error) {
	var record LoginRecord
	data, err := keyring.Get(credentialService, credentialUser)
	if errors.Is(err, keyring.ErrNotFound) {
		return record, false, nil
	}
	if err != nil {
		return record, false, err
	}
	return record, true, json.Unmarshal([]byte(data), &record)
}

// Delete removes the saved record. A record that is already gone is not an error.
func (CredentialManager) Delete() error {
	err := keyring.Delete(credentialService, credentialUser)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}

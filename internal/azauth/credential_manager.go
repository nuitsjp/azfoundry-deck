package azauth

import (
	"encoding/json"
	"errors"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
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

func (CredentialManager) Save(record azidentity.AuthenticationRecord) error {
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return keyring.Set(credentialService, credentialUser, string(data))
}

func (CredentialManager) Load() (azidentity.AuthenticationRecord, bool, error) {
	var record azidentity.AuthenticationRecord
	data, err := keyring.Get(credentialService, credentialUser)
	if errors.Is(err, keyring.ErrNotFound) {
		return record, false, nil
	}
	if err != nil {
		return record, false, err
	}
	return record, true, json.Unmarshal([]byte(data), &record)
}

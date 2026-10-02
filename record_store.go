//go:build !e2e

package main

import "azfoundrydeck/internal/azauth"

func recordStore(string) azauth.RecordStore {
	return azauth.CredentialManager{}
}

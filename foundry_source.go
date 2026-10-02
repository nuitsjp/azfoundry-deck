//go:build !e2e

package main

import (
	"azfoundrydeck/internal/azauth"
	"azfoundrydeck/internal/foundry"
	"fmt"
)

func foundrySource(store azauth.RecordStore) (foundry.Source, error) {
	record, found, err := store.Load()
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("signed-in authentication record is missing")
	}
	cred, err := azauth.NewSilentCredential(record)
	if err != nil {
		return nil, err
	}
	return foundry.NewAzure(cred)
}

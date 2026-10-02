//go:build e2e

package main

import (
	"azfoundrydeck/internal/azauth"
	"azfoundrydeck/internal/foundry"
)

func foundrySource(_ azauth.RecordStore) (foundry.Source, error) {
	return foundry.NewFixedSource(), nil
}

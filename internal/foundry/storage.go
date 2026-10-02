package foundry

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func read(path string) (InitialFoundryView, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return InitialFoundryView{}, err
	}
	var view InitialFoundryView
	err = json.Unmarshal(data, &view)
	return view, err
}

func save(path string, view InitialFoundryView) error {
	return saveJSON(path, view)
}

func modelsPath(statePath, foundryID string) string {
	id := sha256.Sum256([]byte(foundryID))
	return filepath.Join(filepath.Dir(statePath), "foundry-models", fmt.Sprintf("%x.json", id))
}

// savedModels is one Foundry's model file.
type savedModels struct {
	FetchedAt   string       `json:"fetchedAt"`
	Deployments []Deployment `json:"deployments"`
}

func readModels(path string) (savedModels, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return savedModels{}, err
	}
	var models savedModels
	if err := json.Unmarshal(data, &models); err != nil {
		return savedModels{}, err
	}
	if models.Deployments == nil {
		return savedModels{}, fmt.Errorf("saved deployments must be an array")
	}
	return models, nil
}

func saveModels(path string, models savedModels) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return saveJSON(path, models)
}

// removeModelsExcept deletes the model files of Foundries outside the list.
func removeModelsExcept(statePath string, foundries []Foundry) error {
	keep := map[string]bool{}
	for _, foundry := range foundries {
		keep[modelsPath(statePath, foundry.ID)] = true
	}
	dir := filepath.Join(filepath.Dir(statePath), "foundry-models")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if !keep[path] {
			if err := os.Remove(path); err != nil {
				return err
			}
		}
	}
	return nil
}

func saveJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".foundry-state-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

package foundry

import (
	"crypto/sha256"
	"encoding/json"
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

func readModels(path string) ([]Deployment, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var models []Deployment
	if err := json.Unmarshal(data, &models); err != nil {
		return nil, err
	}
	if models == nil {
		return nil, fmt.Errorf("saved deployments must be an array")
	}
	return models, nil
}

func saveModels(path string, models []Deployment) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return saveJSON(path, models)
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

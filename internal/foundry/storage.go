package foundry

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// savedState is the file: the Foundry list and the selection. Deployments are
// always fetched from Azure and never saved.
type savedState struct {
	Foundries          []Foundry `json:"foundries"`
	SelectedFoundryID  string    `json:"selectedFoundryId"`
	FoundriesFetchedAt string    `json:"foundriesFetchedAt"`
}

// read returns the saved Foundry list and selection with empty deployments.
func read(path string) (InitialFoundryView, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return InitialFoundryView{}, err
	}
	var state savedState
	if err := json.Unmarshal(data, &state); err != nil {
		return InitialFoundryView{}, err
	}
	return InitialFoundryView{
		Foundries: state.Foundries, SelectedFoundryID: state.SelectedFoundryID,
		FoundriesFetchedAt: state.FoundriesFetchedAt, Deployments: []Deployment{},
	}, nil
}

func save(path string, view InitialFoundryView) error {
	return saveJSON(path, savedState{Foundries: view.Foundries, SelectedFoundryID: view.SelectedFoundryID, FoundriesFetchedAt: view.FoundriesFetchedAt})
}

func saveJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
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

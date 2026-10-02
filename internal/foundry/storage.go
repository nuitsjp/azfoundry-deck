package foundry

import (
	"encoding/json"
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
	data, err := json.MarshalIndent(view, "", "  ")
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

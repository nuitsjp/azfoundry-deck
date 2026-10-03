package foundry

const ProgressEvent = "foundry:progress"

type Progress struct {
	FoundryPhase        string `json:"foundryPhase"`
	FoundryCount        int    `json:"foundryCount"`
	SelectedFoundryName string `json:"selectedFoundryName"`
	ModelPhase          string `json:"modelPhase"`
	ModelCount          int    `json:"modelCount"`
}

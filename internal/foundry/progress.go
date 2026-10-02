package foundry

const ProgressEvent = "foundry:progress"

type SubscriptionProgress struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Phase        string `json:"phase"`
	FoundryCount int    `json:"foundryCount"`
}

type DiscoveryProgress struct {
	SubscriptionSearch string
	Subscriptions      []SubscriptionProgress
}

type Progress struct {
	SubscriptionSearch  string                 `json:"subscriptionSearch"`
	Subscriptions       []SubscriptionProgress `json:"subscriptions"`
	SelectedFoundryName string                 `json:"selectedFoundryName"`
	ModelPhase          string                 `json:"modelPhase"`
	ModelCount          int                    `json:"modelCount"`
	SavePhase           string                 `json:"savePhase"`
}

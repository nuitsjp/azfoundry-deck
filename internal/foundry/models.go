package foundry

type Foundry struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	SubscriptionName  string `json:"subscriptionName"`
	ResourceGroupName string `json:"resourceGroupName"`
}

type Deployment struct {
	ID             string `json:"id"`
	DeploymentName string `json:"deploymentName"`
	ModelName      string `json:"modelName"`
	Version        string `json:"version"`
}

type InitialFoundryView struct {
	Foundries         []Foundry    `json:"foundries"`
	SelectedFoundryID string       `json:"selectedFoundryId"`
	Deployments       []Deployment `json:"deployments"`
	// RFC 3339 times when the list and the displayed models were fetched from Azure.
	FoundriesFetchedAt   string `json:"foundriesFetchedAt"`
	DeploymentsFetchedAt string `json:"deploymentsFetchedAt"`
}

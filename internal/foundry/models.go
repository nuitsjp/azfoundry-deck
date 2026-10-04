package foundry

type Foundry struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	SubscriptionName  string `json:"subscriptionName"`
	ResourceGroupName string `json:"resourceGroupName"`
}

// Deployment is one deployed model as listed by Azure. The optional fields are
// absent when Azure does not return them. The unexported fields are the inputs of
// the capacity limit and exist in memory only.
type Deployment struct {
	ID                   string   `json:"id"`
	DeploymentName       string   `json:"deploymentName"`
	ModelName            string   `json:"modelName"`
	Version              string   `json:"version"`
	SKUName              *string  `json:"skuName"`
	Capacity             *float64 `json:"capacity"`
	CapacityUnit         *string  `json:"capacityUnit"`
	ProvisioningState    *string  `json:"provisioningState"`
	VersionUpgradePolicy *string  `json:"versionUpgradePolicy"`

	format      string
	skuCapacity float64
	multiplier  float64
}

type InitialFoundryView struct {
	Foundries         []Foundry    `json:"foundries"`
	SelectedFoundryID string       `json:"selectedFoundryId"`
	Deployments       []Deployment `json:"deployments"`
	// RFC 3339 times when the Foundry list was fetched and when the displayed
	// deployments were last fetched from Azure.
	FoundriesFetchedAt   string `json:"foundriesFetchedAt"`
	DeploymentsFetchedAt string `json:"deploymentsFetchedAt"`
}

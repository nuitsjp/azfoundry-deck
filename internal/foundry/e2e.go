//go:build e2e

package foundry

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

// Opt-in gates hold the external source until the E2E test releases each stage.
// Ordinary E2E tests and screen review retain immediate fixed responses.
func waitForRelease(ctx context.Context, stage string) error {
	if os.Getenv("AZFOUNDRYDECK_E2E_HOLD_FOUNDRY") != "1" {
		return ctx.Err()
	}
	path := filepath.Join(os.Getenv("WAILS_DATA_DIR"), "e2e-foundry-"+stage+"-release")
	for {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}

type fixedSource struct{}

func NewFixedSource() Source { return fixedSource{} }

func (fixedSource) Foundries(ctx context.Context) ([]Foundry, error) {
	if err := waitForRelease(ctx, "discovery"); err != nil {
		return nil, err
	}
	// AZFOUNDRYDECK_E2E_FOUNDRIES=none: no Foundry is readable.
	if os.Getenv("AZFOUNDRYDECK_E2E_FOUNDRIES") == "none" {
		return withoutDeletedFoundries(fixedCreatedFoundries()), ctx.Err()
	}
	return withoutDeletedFoundries(append([]Foundry{
		{ID: "/subscriptions/review-production/resourceGroups/rg-ai-production-japaneast/providers/Microsoft.CognitiveServices/accounts/contoso-foundry-production-japaneast", Name: "contoso-foundry-production-japaneast", SubscriptionName: "Contoso AI Production Subscription", ResourceGroupName: "rg-ai-production-japaneast"},
		{ID: "/subscriptions/review-development/resourceGroups/rg-ai-development/providers/Microsoft.CognitiveServices/accounts/contoso-foundry-development", Name: "contoso-foundry-development", SubscriptionName: "Contoso Development", ResourceGroupName: "rg-ai-development"},
		{ID: "/subscriptions/review-research/resourceGroups/rg-ai-research/providers/Microsoft.CognitiveServices/accounts/contoso-foundry-research", Name: "contoso-foundry-research", SubscriptionName: "Contoso Research", ResourceGroupName: "rg-ai-research"},
	}, fixedCreatedFoundries()...)), ctx.Err()
}

func (fixedSource) Deployments(ctx context.Context, foundry Foundry, report func(int)) ([]Deployment, error) {
	newFoundry := false
	for _, created := range fixedCreatedFoundries() {
		if created.ID == foundry.ID {
			if err := fixedCreationWait(ctx, time.Second); err != nil {
				return nil, err
			}
			newFoundry = true
			break
		}
	}
	deployments := []Deployment{
		{ID: foundry.ID + "/deployments/chat-production", DeploymentName: "chat-production", ModelName: "gpt-4.1", Version: "2025-04-14"},
		{ID: foundry.ID + "/deployments/chat-mini", DeploymentName: "chat-mini", ModelName: "gpt-4.1-mini", Version: "2025-04-14"},
		{ID: foundry.ID + "/deployments/embeddings", DeploymentName: "embeddings", ModelName: "text-embedding-3-large", Version: "1"},
	}
	switch foundry.Name {
	case "contoso-foundry-development":
		deployments = []Deployment{
			{ID: foundry.ID + "/deployments/development-chat", DeploymentName: "development-chat", ModelName: "gpt-4.1", Version: "2025-04-14"},
			{ID: foundry.ID + "/deployments/development-mini", DeploymentName: "development-mini", ModelName: "gpt-4.1-mini", Version: "2025-04-14"},
			{ID: foundry.ID + "/deployments/development-embedding", DeploymentName: "development-embedding", ModelName: "text-embedding-3-large", Version: "1"},
		}
	case "contoso-foundry-research":
		deployments = []Deployment{
			{ID: foundry.ID + "/deployments/research-chat", DeploymentName: "research-chat", ModelName: "gpt-4.1", Version: "2025-04-14"},
		}
	}
	if newFoundry {
		deployments = []Deployment{}
	}
	if err := waitForRelease(ctx, "models"); err != nil {
		return nil, err
	}
	deployments = slices.DeleteFunc(deployments, func(deployment Deployment) bool { return deleted.has(deployment.ID) })
	deployments = append(deployments, created.get(foundry.ID)...)
	for i := range deployments {
		deployments[i] = fixedDetails(deployments[i])
	}
	deployments = patched.apply(deployments)
	report(len(deployments))
	return deployments, ctx.Err()
}

// deleted remembers deployments removed through the fixed source, so later fetches omit them.
var deleted = deletedSet{ids: map[string]bool{}}

type deletedSet struct {
	mu  sync.Mutex
	ids map[string]bool
}

func (d *deletedSet) has(id string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.ids[id]
}

func (d *deletedSet) add(id string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.ids[id] = true
}

func (fixedSource) DeleteDeployment(ctx context.Context, foundry Foundry, deployment Deployment) error {
	if err := waitForRelease(ctx, "delete"); err != nil {
		return err
	}
	// AZFOUNDRYDECK_E2E_FAIL=delete: Azure rejects the deletion (for example, insufficient permission).
	if os.Getenv("AZFOUNDRYDECK_E2E_FAIL") == "delete" {
		return fmt.Errorf("simulated deployment deletion failure")
	}
	deleted.add(deployment.ID)
	return ctx.Err()
}

// fixedDetails adds the fixed SKU, capacity and policy of the deployment's model.
func fixedDetails(d Deployment) Deployment {
	sku, capacity, unit := "GlobalStandard", float64(50000), "TPM"
	state, policy := "Succeeded", "OnceNewDefaultVersionAvailable"
	switch d.ModelName {
	case "gpt-4.1-mini":
		capacity = 100000
		policy = "OnceCurrentVersionExpired"
	case "text-embedding-3-large":
		sku = "Standard"
		capacity = 20000
		policy = "NoAutoUpgrade"
	}
	d.SKUName, d.Capacity, d.CapacityUnit = &sku, &capacity, &unit
	d.ProvisioningState, d.VersionUpgradePolicy = &state, &policy
	return d
}

type fixedLimits struct{}

// CapacityLimits is the held first fetch of the limits; AZFOUNDRYDECK_E2E_FAIL=detail fails it.
func (fixedSource) CapacityLimits(ctx context.Context, foundry Foundry) (CapacityLimits, error) {
	if err := waitForRelease(ctx, "detail"); err != nil {
		return nil, err
	}
	if os.Getenv("AZFOUNDRYDECK_E2E_FAIL") == "detail" {
		return nil, fmt.Errorf("simulated capacity limit retrieval failure")
	}
	return fixedLimits{}, ctx.Err()
}

func (fixedLimits) Maximum(d Deployment) *float64 {
	maximum := float64(160000)
	switch d.ModelName {
	case "gpt-4.1-mini":
		maximum = 250000
	case "text-embedding-3-large":
		maximum = 80000
	}
	return &maximum
}

func (fixedLimits) Versions(d Deployment) []string {
	versions := []string{"2025-04-14", "2024-11-20"}
	switch d.ModelName {
	case "gpt-4.1-mini":
		versions = []string{"2025-04-14", "2024-07-18"}
	case "text-embedding-3-large":
		versions = []string{"1", "2"}
	}
	if !slices.Contains(versions, d.Version) {
		versions = append([]string{d.Version}, versions...)
	}
	return versions
}

// RefreshQuota is the held quota fetch of the edit dialog; AZFOUNDRYDECK_E2E_FAIL=update-settings fails it.
func (l fixedLimits) RefreshQuota(ctx context.Context) (CapacityLimits, error) {
	if err := waitForRelease(ctx, "update-settings"); err != nil {
		return nil, err
	}
	if os.Getenv("AZFOUNDRYDECK_E2E_FAIL") == "update-settings" {
		return nil, fmt.Errorf("simulated deployment settings retrieval failure")
	}
	return l, ctx.Err()
}

// created remembers deployments created through the fixed source.
var created = createdStore{deployments: []Deployment{}}

type createdStore struct {
	mu          sync.Mutex
	deployments []Deployment
}

func (c *createdStore) get(foundryID string) []Deployment {
	c.mu.Lock()
	defer c.mu.Unlock()
	var res []Deployment
	prefix := foundryID + "/deployments/"
	for _, d := range c.deployments {
		if len(d.ID) > len(prefix) && d.ID[:len(prefix)] == prefix {
			res = append(res, d)
		}
	}
	return res
}

func (c *createdStore) add(d Deployment) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deployments = append(c.deployments, d)
}

func (fixedSource) ListModels(ctx context.Context, foundry Foundry) ([]ModelCatalogItem, error) {
	if err := waitForRelease(ctx, "catalog"); err != nil {
		return nil, err
	}
	if os.Getenv("AZFOUNDRYDECK_E2E_FAIL") == "catalog" {
		return nil, fmt.Errorf("simulated model catalog retrieval failure")
	}

	cap160 := int64(160000)
	cap250 := int64(250000)
	cap100 := int64(100000)
	cap80 := int64(80000)

	inSonnet := "$3.00 / 1M"
	outSonnet := "$15.00 / 1M"
	inHaiku := "$0.80 / 1M"
	outHaiku := "$4.00 / 1M"
	inLlama := "$0.70 / 1M"
	outLlama := "$0.90 / 1M"
	inMistral := "$2.00 / 1M"
	outMistral := "$6.00 / 1M"

	cap66 := int64(66700)

	return []ModelCatalogItem{
		{
			Name:        "gpt-4o",
			Publisher:   "OpenAI",
			Option:      "Standard",
			Tasks:       []string{"Chat", "Multimodal"},
			Sub:         "128k context · GlobalStandard",
			MaxCapacity: &cap160,
			SKUs: []ModelSKUItem{
				{Name: "GlobalStandard", MaxCapacity: &cap160},
				{Name: "DataZoneStandard", MaxCapacity: &cap66},
			},
			Versions: []string{"2024-11-20 (Default)", "2024-08-06", "2024-05-13"},
		},
		{
			Name:        "gpt-4o-mini",
			Publisher:   "OpenAI",
			Option:      "Standard",
			Tasks:       []string{"Chat"},
			Sub:         "128k context · GlobalStandard",
			MaxCapacity: &cap250,
			SKUs: []ModelSKUItem{
				{Name: "GlobalStandard", MaxCapacity: &cap250},
			},
			Versions: []string{"2024-07-18 (Default)"},
		},
		{
			Name:       "claude-3-5-sonnet",
			Publisher:  "Anthropic",
			Option:     "Pay-as-you-go",
			Tasks:      []string{"Chat", "Multimodal", "Reasoning"},
			Sub:        "200k context · Serverless API",
			SKUs:       []ModelSKUItem{{Name: "GlobalProvisioned"}},
			Versions:   []string{"20241022 (Default)", "20240620"},
			InputRate:  &inSonnet,
			OutputRate: &outSonnet,
		},
		{
			Name:       "claude-3-5-haiku",
			Publisher:  "Anthropic",
			Option:     "Pay-as-you-go",
			Tasks:      []string{"Chat", "Reasoning"},
			Sub:        "200k context · Fast & Intelligent",
			SKUs:       []ModelSKUItem{{Name: "GlobalProvisioned"}},
			Versions:   []string{"20241022 (Default)"},
			InputRate:  &inHaiku,
			OutputRate: &outHaiku,
		},
		{
			Name:        "gpt-4.1",
			Publisher:   "OpenAI",
			Option:      "Standard",
			Tasks:       []string{"Reasoning", "Multimodal"},
			Sub:         "Next-gen reasoning model",
			MaxCapacity: &cap160,
			SKUs: []ModelSKUItem{
				{Name: "GlobalStandard", MaxCapacity: &cap160},
				{Name: "DataZoneStandard", MaxCapacity: &cap66},
			},
			Versions: []string{"2024-11-20 (Default)"},
		},
		{
			Name:       "llama-3.3-70b-instruct",
			Publisher:  "Meta",
			Option:     "Pay-as-you-go",
			Tasks:      []string{"Chat", "Reasoning"},
			Sub:        "128k context · Open-weight flagship",
			Versions:   []string{"1 (Default)"},
			InputRate:  &inLlama,
			OutputRate: &outLlama,
		},
		{
			Name:       "mistral-large-2411",
			Publisher:  "Mistral AI",
			Option:     "Pay-as-you-go",
			Tasks:      []string{"Chat", "Reasoning", "Multimodal"},
			Sub:        "128k context · Top-tier reasoning",
			Versions:   []string{"2411 (Default)"},
			InputRate:  &inMistral,
			OutputRate: &outMistral,
		},
		{
			Name:        "phi-4",
			Publisher:   "Microsoft",
			Option:      "Standard",
			Tasks:       []string{"Reasoning"},
			Sub:         "14B parameters · SOTA math & code",
			MaxCapacity: &cap100,
			Versions:    []string{"1 (Default)"},
		},
		{
			Name:        "text-embedding-3-large",
			Publisher:   "OpenAI",
			Option:      "Standard",
			Tasks:       []string{"Embeddings"},
			Sub:         "3,072 dimensions · High accuracy",
			MaxCapacity: &cap80,
			Versions:    []string{"1 (Default)"},
		},
		{
			Name:        "text-embedding-3-small",
			Publisher:   "OpenAI",
			Option:      "Standard",
			Tasks:       []string{"Embeddings"},
			Sub:         "1,536 dimensions · Efficient embeddings",
			MaxCapacity: &cap80,
			Versions:    []string{"1 (Default)"},
		},
	}, ctx.Err()
}

func (fixedSource) CreateDeployment(ctx context.Context, foundry Foundry, spec DeploymentCreateSpec) error {
	if err := waitForRelease(ctx, "create"); err != nil {
		return err
	}
	if os.Getenv("AZFOUNDRYDECK_E2E_FAIL") == "create" || spec.DeploymentName == "fail-deploy" || spec.DeploymentName == "chat-production" {
		return fmt.Errorf("The deployment name '%s' already exists in this Foundry.", spec.DeploymentName)
	}
	created.add(Deployment{
		ID:             foundry.ID + "/deployments/" + spec.DeploymentName,
		DeploymentName: spec.DeploymentName,
		ModelName:      spec.ModelName,
		Version:        spec.Version,
	})
	return ctx.Err()
}

// patched remembers deployment setting changes so later list fetches return them.
var patched = patchStore{items: map[string]deploymentPatch{}}

type deploymentPatch struct {
	Version  string
	Capacity *float64
	Policy   string
}

type patchStore struct {
	mu    sync.Mutex
	items map[string]deploymentPatch
}

func (p *patchStore) get(id string) (deploymentPatch, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	patch, ok := p.items[id]
	return patch, ok
}

func (p *patchStore) set(id string, patch deploymentPatch) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.items[id] = patch
}

func (p *patchStore) apply(deployments []Deployment) []Deployment {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range deployments {
		patch, ok := p.items[deployments[i].ID]
		if !ok {
			continue
		}
		if patch.Version != "" {
			deployments[i].Version = patch.Version
		}
		if patch.Capacity != nil {
			deployments[i].Capacity = patch.Capacity
		}
		if patch.Policy != "" {
			policy := patch.Policy
			deployments[i].VersionUpgradePolicy = &policy
		}
	}
	return deployments
}

func (fixedSource) UpdateDeployment(ctx context.Context, foundry Foundry, deployment Deployment, spec DeploymentUpdateSpec) error {
	if err := waitForRelease(ctx, "update"); err != nil {
		return err
	}
	if os.Getenv("AZFOUNDRYDECK_E2E_FAIL") == "update" {
		return fmt.Errorf("simulated deployment update failure")
	}
	var capacity *float64
	if spec.Capacity != nil {
		value := float64(*spec.Capacity)
		capacity = &value
	}
	patched.set(deployment.ID, deploymentPatch{Version: spec.Version, Capacity: capacity, Policy: spec.UpgradePolicy})
	return ctx.Err()
}

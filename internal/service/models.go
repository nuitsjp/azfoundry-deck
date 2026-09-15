package service

import "context"

// CapacityContract is the deployment capacity constraint of one model SKU. Every
// field is optional because Azure omits the ones it does not define; the screen
// refuses to build a request rather than filling a missing constraint in.
type CapacityContract struct {
	Default       *int32  `json:"default"`
	Minimum       *int32  `json:"minimum"`
	Maximum       *int32  `json:"maximum"`
	Step          *int32  `json:"step"`
	AllowedValues []int32 `json:"allowedValues"`
}

// ModelSKU is one deployment SKU of a model version together with the capacity
// constraint Azure reports for that exact combination.
type ModelSKU struct {
	Name     string            `json:"name"`
	Capacity *CapacityContract `json:"capacity"`
}

// ModelCandidate is one model-version row returned for a Foundry account.
// Optional values remain empty when Azure does not provide them; the screen
// decides how to present that unknown value.
type ModelCandidate struct {
	Name             string     `json:"name"`
	Format           string     `json:"format"`
	Version          string     `json:"version"`
	Lifecycle        string     `json:"lifecycle"`
	IsDefaultVersion bool       `json:"isDefaultVersion"`
	SKUs             []ModelSKU `json:"skus"`
}

// ModelResult is one complete read of the model catalog for one account.
type ModelResult struct {
	Models    []ModelCandidate `json:"models"`
	Failures  []FetchFailure   `json:"failures"`
	FetchedAt string           `json:"fetchedAt"`
}

// ModelService is the screen-facing service for model candidates, read by the
// model addition screen. The
// fetch function is the boundary to Azure (or the deterministic mock).
type ModelService struct {
	fetch       func(context.Context, string) (ModelResult, error)
	fetchRegion func(context.Context, string, string) (ModelResult, error)
	mock        bool
}

// NewModelService creates the common service used by both the mock and the
// eventual Azure implementation.
func NewModelService(
	fetch func(context.Context, string) (ModelResult, error),
	fetchRegion func(context.Context, string, string) (ModelResult, error),
	mock bool,
) *ModelService {
	return &ModelService{fetch: fetch, fetchRegion: fetchRegion, mock: mock}
}

// GetModels obtains the model candidates for one F1 account row.
func (s *ModelService) GetModels(ctx context.Context, accountID string) (ModelResult, error) {
	result, err := s.fetch(ctx, accountID)
	return normalizeModelResult(result), err
}

// GetRegionModels obtains the candidates a subscription can deploy in one
// region. It serves a Foundry that does not exist yet, so the result is
// provisional and must be confirmed against the created account before use.
func (s *ModelService) GetRegionModels(ctx context.Context, subscriptionID, region string) (ModelResult, error) {
	result, err := s.fetchRegion(ctx, subscriptionID, region)
	return normalizeModelResult(result), err
}

func normalizeModelResult(result ModelResult) ModelResult {
	if result.Models == nil {
		result.Models = make([]ModelCandidate, 0)
	}
	if result.Failures == nil {
		result.Failures = make([]FetchFailure, 0)
	}
	for index := range result.Models {
		if result.Models[index].SKUs == nil {
			result.Models[index].SKUs = make([]ModelSKU, 0)
		}
	}
	return result
}

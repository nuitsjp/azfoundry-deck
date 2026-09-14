package service

import "context"

// ModelCandidate is one model-version row returned for a Foundry account.
// Optional values remain empty when Azure does not provide them; the screen
// decides how to present that unknown value.
type ModelCandidate struct {
	Name             string   `json:"name"`
	Format           string   `json:"format"`
	Version          string   `json:"version"`
	Lifecycle        string   `json:"lifecycle"`
	IsDefaultVersion bool     `json:"isDefaultVersion"`
	SKUs             []string `json:"skus"`
}

// ModelResult is one complete read of the model catalog for one account.
type ModelResult struct {
	Models    []ModelCandidate `json:"models"`
	Failures  []FetchFailure   `json:"failures"`
	FetchedAt string           `json:"fetchedAt"`
}

// ModelService is the screen-facing service for F2 model candidates. The
// fetch function is the boundary to Azure (or the deterministic mock).
type ModelService struct {
	fetch func(context.Context, string) (ModelResult, error)
	mock  bool
}

// NewModelService creates the common service used by both the mock and the
// eventual Azure implementation.
func NewModelService(fetch func(context.Context, string) (ModelResult, error), mock bool) *ModelService {
	return &ModelService{fetch: fetch, mock: mock}
}

// GetModels obtains the model candidates for one F1 account row.
func (s *ModelService) GetModels(ctx context.Context, accountID string) (ModelResult, error) {
	result, err := s.fetch(ctx, accountID)
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
			result.Models[index].SKUs = make([]string, 0)
		}
	}
	return result
}

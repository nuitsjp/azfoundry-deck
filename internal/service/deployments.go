package service

import (
	"context"
	"sync"
)

const DeploymentProgressEvent = "deployments:progress"

const (
	DeploymentProgressStageDiscovering = "discovering"
	DeploymentProgressStageFetching    = "fetching"
)

// DeploymentService is the screen-facing service for the F1 deployment list.
// The fetch function is the boundary to Azure (or the deterministic mock).
type DeploymentService struct {
	fetch func(context.Context, func(DeploymentProgress)) (DeploymentResult, error)
	mock  bool
	emit  func(DeploymentProgress)
}

// Environment describes the source of the data returned by the service.
type Environment struct {
	Mock bool `json:"mock"`
}

// DeploymentResult is one complete read result. TotalAccounts is nil when
// account discovery did not complete, so a discovery failure is not confused
// with a successful empty result.
type DeploymentResult struct {
	Deployments        []Deployment   `json:"deployments"`
	Failures           []FetchFailure `json:"failures"`
	SuccessfulAccounts int            `json:"successfulAccounts"`
	TotalAccounts      *int           `json:"totalAccounts"`
	FetchedAt          string         `json:"fetchedAt"`
}

// DeploymentProgress is one immutable snapshot of an in-flight deployment
// read. TotalSubscriptions and TotalAccounts are nil while their respective
// totals are not known yet.
type DeploymentProgress struct {
	RequestID              string           `json:"requestId"`
	Sequence               uint64           `json:"sequence"`
	Stage                  string           `json:"stage"`
	CompletedSubscriptions int              `json:"completedSubscriptions"`
	TotalSubscriptions     *int             `json:"totalSubscriptions"`
	CompletedAccounts      int              `json:"completedAccounts"`
	Result                 DeploymentResult `json:"result"`
}

// Deployment is a row in the cross-account deployment list.
type Deployment struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	TenantID         string `json:"tenantId"`
	TenantName       string `json:"tenantName"`
	SubscriptionID   string `json:"subscriptionId"`
	SubscriptionName string `json:"subscriptionName"`
	AccountID        string `json:"accountId"`
	AccountName      string `json:"accountName"`
	Region           string `json:"region"`
	Model            string `json:"model"`
	ModelVersion     string `json:"modelVersion"`
	SKU              string `json:"sku"`
	TPM              *int64 `json:"tpm"`
	RPM              *int64 `json:"rpm"`
}

// FetchFailure describes an account or discovery scope that could not be
// read. A scope may be a tenant, subscription, account, or discovery scope.
type FetchFailure struct {
	Scope            string `json:"scope"`
	TenantName       string `json:"tenantName"`
	SubscriptionName string `json:"subscriptionName"`
	AccountName      string `json:"accountName"`
	Code             string `json:"code"`
	Message          string `json:"message"`
	Action           string `json:"action"`
}

// NewDeploymentService creates the common service used by both the mock and
// the eventual Azure implementation.
func NewDeploymentService(fetch func(context.Context, func(DeploymentProgress)) (DeploymentResult, error), mock bool, emit func(DeploymentProgress)) *DeploymentService {
	return &DeploymentService{fetch: fetch, mock: mock, emit: emit}
}

// GetDeployments obtains one snapshot from the configured boundary.
func (s *DeploymentService) GetDeployments(ctx context.Context, requestID string) (DeploymentResult, error) {
	var reportMu sync.Mutex
	var sequence uint64 = 1
	report := func(progress DeploymentProgress) {
		reportMu.Lock()
		defer reportMu.Unlock()

		progress.RequestID = requestID
		progress.Sequence = sequence
		sequence++
		progress.Result = normalizeDeploymentResult(progress.Result)
		if s.emit != nil {
			s.emit(progress)
		}
	}

	result, err := s.fetch(ctx, report)
	result = normalizeDeploymentResult(result)
	return result, err
}

// GetEnvironment tells the screen whether this service is backed by the
// deterministic F1 data source.
func (s *DeploymentService) GetEnvironment() Environment {
	return Environment{Mock: s.mock}
}

func normalizeDeploymentResult(result DeploymentResult) DeploymentResult {
	if result.Deployments == nil {
		result.Deployments = make([]Deployment, 0)
	}
	if result.Failures == nil {
		result.Failures = make([]FetchFailure, 0)
	}
	return result
}

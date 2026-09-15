package service

import "context"

// Creation outcomes. They match the result screens the user agreed on, so the
// screen keeps one wording per outcome instead of rendering raw Azure errors.
const (
	OutcomeSuccess           = "success"
	OutcomeGroupFailure      = "group-failure"
	OutcomeCandidateMismatch = "foundry-candidate-mismatch"
	OutcomeDeploymentFailure = "deployment-failure"
	OutcomeUnknown           = "unknown"
	OutcomeConflict          = "conflict"
	OutcomeListFailure       = "list-failure"
	OutcomeInterrupted       = "interrupted"
)

// CreateRequest is the confirmed content of one model addition. The screen sends
// the exact combination it displayed; the service never rebuilds it.
type CreateRequest struct {
	SubscriptionID string `json:"subscriptionId"`
	ResourceGroup  string `json:"resourceGroup"`
	GroupIsNew     bool   `json:"groupIsNew"`
	GroupRegion    string `json:"groupRegion"`
	FoundryName    string `json:"foundryName"`
	FoundryIsNew   bool   `json:"foundryIsNew"`
	FoundryRegion  string `json:"foundryRegion"`
	Format         string `json:"format"`
	Model          string `json:"model"`
	Version        string `json:"version"`
	SKU            string `json:"sku"`
	Capacity       int32  `json:"capacity"`
	DeploymentName string `json:"deploymentName"`
}

// CreateResult is what the screen shows after one addition attempt. A stage that
// succeeded stays succeeded: a later failure never reports it as rolled back.
type CreateResult struct {
	Outcome        string `json:"outcome"`
	OperationID    string `json:"operationId"`
	CreatedGroup   bool   `json:"createdGroup"`
	CreatedFoundry bool   `json:"createdFoundry"`
	FoundryID      string `json:"foundryId"`
	DeploymentID   string `json:"deploymentId"`
	Detail         string `json:"detail"`
}

// PendingOperation is one addition whose result is not settled. It survives an
// application restart so the same deployment is never sent twice.
type PendingOperation struct {
	ID             string `json:"id"`
	StartedAt      string `json:"startedAt"`
	SubscriptionID string `json:"subscriptionId"`
	DeploymentID   string `json:"deploymentId"`
	Stage          string `json:"stage"`
	State          string `json:"state"`
}

// CreationService is the screen-facing service for adding a model. Writing is
// deliberately separate from the read services: it records an operation before
// every request and never retries a request on its own.
type CreationService struct {
	create  func(context.Context, CreateRequest) (CreateResult, error)
	check   func(context.Context, string) (CreateResult, error)
	pending func(context.Context) ([]PendingOperation, error)
	mock    bool
}

// NewCreationService creates the common service used by both the mock and the
// Azure implementation.
func NewCreationService(
	create func(context.Context, CreateRequest) (CreateResult, error),
	check func(context.Context, string) (CreateResult, error),
	pending func(context.Context) ([]PendingOperation, error),
	mock bool,
) *CreationService {
	return &CreationService{create: create, check: check, pending: pending, mock: mock}
}

// AddModel performs one confirmed addition. It returns the observed outcome;
// it does not resend a request whose result is unknown.
func (s *CreationService) AddModel(ctx context.Context, request CreateRequest) (CreateResult, error) {
	return s.create(ctx, request)
}

// CheckOperation re-reads the state of one recorded operation. It only reads:
// nothing is created, resent or deleted.
func (s *CreationService) CheckOperation(ctx context.Context, operationID string) (CreateResult, error) {
	return s.check(ctx, operationID)
}

// PendingOperations lists the operations whose result is still unsettled, so the
// screen can show them after a restart instead of silently continuing.
func (s *CreationService) PendingOperations(ctx context.Context) ([]PendingOperation, error) {
	operations, err := s.pending(ctx)
	if operations == nil {
		operations = make([]PendingOperation, 0)
	}
	return operations, err
}

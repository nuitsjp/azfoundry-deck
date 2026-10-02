// Package azauth establishes and holds the Azure sign-in state.
package azauth

import (
	"context"
	"log/slog"
	"sync"

	"azfoundrydeck/internal/fault"
)

type Phase string

const (
	SignedOut Phase = "signedOut"
	SigningIn Phase = "signingIn"
	SignedIn  Phase = "signedIn"
)

type Account struct {
	Username string `json:"username"`
	TenantID string `json:"tenantID"`
}

type Status struct {
	Phase   Phase    `json:"phase"`
	Account *Account `json:"account,omitempty"`
}

// Authenticator is the only boundary to Azure SDK and the mock composition point.
type Authenticator interface {
	Authenticate(ctx context.Context) (Account, error)
}

type Service struct {
	mu     sync.Mutex
	status Status
	auth   Authenticator
	logger *slog.Logger
}

func New(auth Authenticator, logger *slog.Logger) *Service {
	return &Service{status: Status{Phase: SignedOut}, auth: auth, logger: logger}
}

func (s *Service) GetStatus() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// Login succeeds only when the authenticator has acquired a token. On failure
// the state returns to SignedOut and never becomes a substituted success.
func (s *Service) Login(ctx context.Context) (Status, error) {
	s.mu.Lock()
	if s.status.Phase == SigningIn {
		s.mu.Unlock()
		return Status{}, fault.New("BUSY", "サインインの完了を待っています。")
	}
	s.status = Status{Phase: SigningIn}
	s.mu.Unlock()

	account, err := s.auth.Authenticate(ctx)

	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		s.status = Status{Phase: SignedOut}
		s.logger.Error("operation_failed", "operation", "azauth.Login", "cause", err)
		if ctx.Err() != nil {
			return Status{}, fault.Public(ctx.Err())
		}
		return Status{}, fault.New("LOGIN_FAILED", "Azureにログインできませんでした。ブラウザーでのサインインを完了したか確認し、もう一度ログインしてください。")
	}
	s.status = Status{Phase: SignedIn, Account: &account}
	return s.status, nil
}

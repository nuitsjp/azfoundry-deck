// Package azauth establishes and holds the Azure sign-in state.
package azauth

import (
	"context"
	"log/slog"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"

	"azfoundrydeck/internal/fault"
)

type Phase string

const (
	SignedOut Phase = "signedOut"
	SigningIn Phase = "signingIn"
	SignedIn  Phase = "signedIn"
)

type Account struct {
	Username   string `json:"username"`
	TenantName string `json:"tenantName"`
}

type Status struct {
	Phase   Phase    `json:"phase"`
	Account *Account `json:"account,omitempty"`
}

// RecordStore saves the account identity that a later start uses to restore sign-in.
type RecordStore interface {
	Save(record azidentity.AuthenticationRecord) error
}

type Service struct {
	mu     sync.Mutex
	status Status
	store  RecordStore
	logger *slog.Logger
}

func New(store RecordStore, logger *slog.Logger) *Service {
	return &Service{status: Status{Phase: SignedOut}, store: store, logger: logger}
}

func (s *Service) GetStatus() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// Login succeeds only when the token, the tenant name and the saved record are
// all in place. On failure the state returns to SignedOut and never becomes a
// substituted success.
func (s *Service) Login(ctx context.Context) (Status, error) {
	s.mu.Lock()
	if s.status.Phase == SigningIn {
		s.mu.Unlock()
		return Status{}, fault.New("BUSY", "サインインの完了を待っています。")
	}
	s.status = Status{Phase: SigningIn}
	s.mu.Unlock()

	account, record, err := signIn(ctx)
	if err == nil {
		err = s.store.Save(record)
	}

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

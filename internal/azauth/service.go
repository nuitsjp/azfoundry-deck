// Package azauth establishes and holds the Azure sign-in state.
package azauth

import (
	"context"
	"log/slog"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/wailsapp/wails/v3/pkg/application"

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

// RecordStore keeps the account identity that a later start uses to restore sign-in.
type RecordStore interface {
	Save(record azidentity.AuthenticationRecord) error
	// Load reports found=false when nothing is saved.
	Load() (record azidentity.AuthenticationRecord, found bool, err error)
}

type Service struct {
	mu         sync.Mutex
	status     Status
	restoreErr error
	restored   chan struct{}
	store      RecordStore
	logger     *slog.Logger
}

// New returns a service whose GetStatus waits until the startup restore has finished.
func New(store RecordStore, logger *slog.Logger) *Service {
	return &Service{status: Status{Phase: SigningIn}, restored: make(chan struct{}), store: store, logger: logger}
}

// ServiceStartup is called by Wails when the app starts and runs the restore in
// the background; it is not exposed to the frontend.
func (s *Service) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	go s.restore(ctx)
	return nil
}

// restore signs in silently with the saved record and the persistent token
// cache. It never opens a browser and never deletes the saved record. Without a
// saved record the state becomes SignedOut; on failure GetStatus returns the
// failure until the next Login starts.
func (s *Service) restore(ctx context.Context) {
	defer close(s.restored)
	record, found, err := s.store.Load()
	if err == nil && !found {
		s.setStatus(Status{Phase: SignedOut})
		return
	}
	var account Account
	if err == nil {
		account, err = restoreAccount(ctx, record)
	}
	if err != nil {
		s.logger.Error("operation_failed", "operation", "azauth.Restore", "cause", err)
		s.mu.Lock()
		s.status = Status{Phase: SignedOut}
		s.restoreErr = fault.New("LOGIN_FAILED", "保存済みのログイン情報でAzureにログインできませんでした。もう一度ログインしてください。")
		s.mu.Unlock()
		return
	}
	s.setStatus(Status{Phase: SignedIn, Account: &account})
}

func (s *Service) setStatus(status Status) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = status
}

// GetStatus waits for the startup restore so the screen shows neither the
// login modal nor a signed-in header before the outcome is known.
func (s *Service) GetStatus(ctx context.Context) (Status, error) {
	select {
	case <-s.restored:
	case <-ctx.Done():
		return Status{}, fault.Public(ctx.Err())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.restoreErr != nil {
		return Status{}, s.restoreErr
	}
	return s.status, nil
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
	s.restoreErr = nil
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

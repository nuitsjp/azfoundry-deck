// Package azauth establishes and holds the Azure sign-in state.
package azauth

import (
	"context"
	"log/slog"
	"slices"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"

	"azfoundrydeck/internal/fault"
)

// tokenCacheName isolates this app's persistent token cache. On Windows the
// SDK stores it DPAPI-encrypted at %LOCALAPPDATA%\.IdentityService\<name>.
const tokenCacheName = "azfoundrydeck"

type Phase string

const (
	SignedOut       Phase = "signedOut"
	SigningIn       Phase = "signingIn"
	SelectingTenant Phase = "selectingTenant"
	SignedIn        Phase = "signedIn"
)

type Account struct {
	Username         string   `json:"username"`
	Tenants          []Tenant `json:"tenants"`
	SelectedTenantID string   `json:"selectedTenantId"`
}

type Tenant struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
}

type Status struct {
	Phase   Phase    `json:"phase"`
	Account *Account `json:"account,omitempty"`
}

// RecordStore keeps the account identity that a later start uses to restore sign-in.
type RecordStore interface {
	Save(record LoginRecord) error
	// Load reports found=false when nothing is saved.
	Load() (record LoginRecord, found bool, err error)
	// Delete removes the saved record; nothing saved is not an error.
	Delete() error
}

type Service struct {
	mu         sync.Mutex
	status     Status
	restoreErr error
	restored   chan struct{}
	store      RecordStore
	logger     *slog.Logger
	clearViews func() error
	operations *sync.Mutex
}

// New returns a service whose GetStatus waits until the startup restore has finished.
func New(store RecordStore, logger *slog.Logger, clearViews func() error, operations *sync.Mutex) *Service {
	return &Service{status: Status{Phase: SigningIn}, restored: make(chan struct{}), store: store, logger: logger, clearViews: clearViews, operations: operations}
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
		account, err = record.account()
		if err == nil {
			err = restoreToken(ctx, record)
		}
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

// Login succeeds only when the token, tenant list and saved selection are
// all in place. On failure the state returns to SignedOut and never becomes a
// substituted success.
func (s *Service) Login(ctx context.Context) (Status, error) {
	s.operations.Lock()
	defer s.operations.Unlock()
	s.mu.Lock()
	if s.status.Phase == SigningIn {
		s.mu.Unlock()
		return Status{}, fault.New("BUSY", "サインインの完了を待っています。")
	}
	s.status = Status{Phase: SigningIn}
	s.restoreErr = nil
	s.mu.Unlock()

	record, err := signIn(ctx)
	saved := LoginRecord{Record: record}
	if err == nil {
		err = s.store.Save(saved)
	}
	if err == nil {
		saved.Tenants, err = listTenants(ctx, record)
	}
	if err == nil {
		err = s.store.Save(saved)
	}
	var account Account
	phase := SignedIn
	if err == nil {
		switch len(saved.Tenants) {
		case 0:
			err = fault.New("NO_TENANTS", "利用可能なテナントがありません。")
		case 1:
			saved.SelectedTenantID = saved.Tenants[0].ID
			err = acquireTenantToken(ctx, saved)
			if err == nil {
				err = s.store.Save(saved)
			}
			if err == nil {
				account, err = saved.account()
			}
		default:
			phase = SelectingTenant
			account = Account{Username: record.Username, Tenants: saved.Tenants}
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		s.status = Status{Phase: SignedOut}
		s.logger.Error("operation_failed", "operation", "azauth.Login", "cause", err)
		if ctx.Err() != nil {
			return Status{}, fault.Public(ctx.Err())
		}
		return Status{}, fault.New("LOGIN_FAILED", "Azureにログインできませんでした。"+err.Error())
	}
	s.status = Status{Phase: phase, Account: &account}
	return s.status, nil
}

// SelectTenant completes sign-in after the user chooses from the saved list.
func (s *Service) SelectTenant(ctx context.Context, tenantID string) (Status, error) {
	s.operations.Lock()
	defer s.operations.Unlock()
	s.mu.Lock()
	selecting := s.status.Phase == SelectingTenant
	s.mu.Unlock()
	if !selecting {
		return Status{}, fault.New("NOT_SELECTING_TENANT", "テナント選択待ちではありません。")
	}
	saved, found, err := s.store.Load()
	if err == nil && !found {
		err = fault.New("LOGIN_REQUIRED", "サインインし直してください。")
	}
	valid := false
	if err == nil {
		for _, tenant := range saved.Tenants {
			if tenant.ID == tenantID && tenantID != "" {
				valid = true
				break
			}
		}
		if !valid {
			return Status{}, fault.New("INVALID_TENANT", "一覧からテナントを選択してください。")
		}
		saved.SelectedTenantID = tenantID
		err = acquireTenantToken(ctx, saved)
	}
	if err == nil {
		err = s.store.Save(saved)
	}
	if err != nil {
		s.logger.Error("operation_failed", "operation", "azauth.SelectTenant", "cause", err)
		if ctx.Err() != nil {
			return Status{}, fault.Public(ctx.Err())
		}
		return Status{}, fault.New("SELECT_TENANT_FAILED", "テナントを設定できませんでした。もう一度確定してください。")
	}
	account, err := saved.account()
	if err != nil {
		return Status{}, fault.Public(err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = Status{Phase: SignedIn, Account: &account}
	return s.status, nil
}

// ChangeTenant switches the target tenant of a signed-in account. The previous
// selection and status stay in place unless the token and the saved selection both succeed.
func (s *Service) ChangeTenant(ctx context.Context, tenantID string) (Status, error) {
	s.operations.Lock()
	defer s.operations.Unlock()
	s.mu.Lock()
	status := s.status
	s.mu.Unlock()
	if status.Phase != SignedIn {
		return Status{}, fault.New("NOT_SIGNED_IN", "ログインしていません。")
	}
	if status.Account != nil && status.Account.SelectedTenantID == tenantID {
		return status, nil
	}
	saved, found, err := s.store.Load()
	if err == nil && !found {
		err = fault.New("LOGIN_REQUIRED", "サインインし直してください。")
	}
	if err == nil {
		if !slices.ContainsFunc(saved.Tenants, func(tenant Tenant) bool { return tenant.ID == tenantID && tenantID != "" }) {
			return Status{}, fault.New("INVALID_TENANT", "一覧からテナントを選択してください。")
		}
		saved.SelectedTenantID = tenantID
		err = acquireTenantToken(ctx, saved)
	}
	if err == nil {
		err = s.store.Save(saved)
	}
	if err != nil {
		s.logger.Error("operation_failed", "operation", "azauth.ChangeTenant", "cause", err)
		if ctx.Err() != nil {
			return Status{}, fault.Public(ctx.Err())
		}
		return Status{}, fault.New("SELECT_TENANT_FAILED", "テナントを変更できませんでした。もう一度選択してください。")
	}
	account, err := saved.account()
	if err != nil {
		return Status{}, fault.Public(err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = Status{Phase: SignedIn, Account: &account}
	return s.status, nil
}

// Logout deletes the persistent token cache and saved views, then the record,
// then drops the in-memory sign-in. Tokens go first so that a partial failure
// leaves only the record, which holds no secret; the next start then fails the
// restore and shows the login modal. If any deletion fails the state stays
// SignedIn, and a retry is safe because both deletions are idempotent.
func (s *Service) Logout() (Status, error) {
	s.operations.Lock()
	defer s.operations.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.status.Phase != SignedIn {
		return Status{}, fault.New("NOT_SIGNED_IN", "ログインしていません。")
	}
	err := deleteTokenCache(tokenCacheName)
	if err == nil {
		err = s.clearViews()
	}
	if err == nil {
		err = s.store.Delete()
	}
	if err != nil {
		s.logger.Error("operation_failed", "operation", "azauth.Logout", "cause", err)
		return Status{}, fault.New("LOGOUT_FAILED", "Azureからログアウトできませんでした。もう一度ログアウトしてください。")
	}
	s.status = Status{Phase: SignedOut}
	return s.status, nil
}

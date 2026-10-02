package azauth

import (
	"context"
	"errors"
	"time"
)

// Fixed is the specification mock: a fixed account after a short wait.
// It is composed only by non-production builds started in mock mode.
type Fixed struct {
	Fail bool
}

func (f Fixed) Authenticate(ctx context.Context) (Account, error) {
	select {
	case <-ctx.Done():
		return Account{}, ctx.Err()
	case <-time.After(3 * time.Second):
	}
	if f.Fail {
		return Account{}, errors.New("mock: AZFOUNDRYDECK_MOCK_LOGIN_FAIL=1")
	}
	return Account{Username: "operator@contoso.onmicrosoft.com", TenantID: "00000000-0000-0000-0000-000000000001"}, nil
}

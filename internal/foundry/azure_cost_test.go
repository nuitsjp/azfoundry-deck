//go:build !e2e

package foundry

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
)

func TestThrottledReadsCostManagementRetryAfter(t *testing.T) {
	response := func(status int, retryAfter string) error {
		header := http.Header{}
		if retryAfter != "" {
			header.Set(costRetryAfter, retryAfter)
		}
		return &azcore.ResponseError{StatusCode: status, RawResponse: &http.Response{StatusCode: status, Header: header}}
	}
	for _, c := range []struct {
		name string
		err  error
		wait time.Duration
		ok   bool
	}{
		{"429 with the entity header", response(http.StatusTooManyRequests, "56"), 56 * time.Second, true},
		{"429 without the header", response(http.StatusTooManyRequests, ""), 0, false},
		{"403", response(http.StatusForbidden, "56"), 0, false},
		{"not a response error", errors.New("network"), 0, false},
		{"success", nil, 0, false},
	} {
		wait, ok := throttled(c.err)
		if wait != c.wait || ok != c.ok {
			t.Errorf("%s: throttled = %v, %v; want %v, %v", c.name, wait, ok, c.wait, c.ok)
		}
	}
}

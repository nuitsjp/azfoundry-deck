package azurego

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
)

type testCredential func(context.Context, policy.TokenRequestOptions) (azcore.AccessToken, error)

func (f testCredential) GetToken(ctx context.Context, options policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return f(ctx, options)
}

type testTransport func(*http.Request) (*http.Response, error)

func (f testTransport) Do(request *http.Request) (*http.Response, error) { return f(request) }

func testSDKClient(t *testing.T, credential azcore.TokenCredential, transport testTransport) *sdkSubscriptionClient {
	t.Helper()
	client, err := arm.NewClient("azfoundry-test", "v0.0.0", credential, &arm.ClientOptions{
		DisableRPRegistration: true,
		ClientOptions: policy.ClientOptions{
			Transport: transport,
			Retry:     policy.RetryOptions{MaxRetries: 1, RetryDelay: time.Millisecond, MaxRetryDelay: time.Millisecond},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return &sdkSubscriptionClient{client: client, subscriptionID: "sub"}
}

func testToken(calls *atomic.Int32) testCredential {
	return func(context.Context, policy.TokenRequestOptions) (azcore.AccessToken, error) {
		calls.Add(1)
		return azcore.AccessToken{Token: "test-token", ExpiresOn: time.Now().Add(time.Hour)}, nil
	}
}

func testJSONResponse(request *http.Request, status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    request,
	}
}

func TestSDKListsAllPagesAndReusesTokenWithoutCachingResourceData(t *testing.T) {
	const accountID = "/subscriptions/sub/resourceGroups/rg one/providers/Microsoft.CognitiveServices/accounts/account"
	var tokens, accountReads, deploymentReads atomic.Int32
	var refresh atomic.Int32
	client := testSDKClient(t, testToken(&tokens), func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodGet || request.URL.Host != "management.azure.com" || request.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("expected an authenticated, read-only ARM request")
		}
		query := request.URL.Query()
		var body string
		switch request.URL.EscapedPath() {
		case "/subscriptions/sub/resources":
			accountReads.Add(1)
			if query.Get("$skiptoken") == "" {
				if query.Get("api-version") != "2021-04-01" || query.Get("$filter") != "resourceType eq 'Microsoft.CognitiveServices/accounts'" || query.Has("$top") {
					t.Errorf("unexpected Resources List query: %v", query)
				}
				body = `{"value":[null,{"kind":"TextAnalytics"}],"nextLink":"https://management.azure.com/subscriptions/sub/resources?$skiptoken=page%2B2%2F%3D&api-version=2021-04-01"}`
			} else {
				if query.Get("$skiptoken") != "page+2/=" {
					t.Errorf("continuation token changed: %v", query)
				}
				body = `{"value":[{"id":"` + accountID + `","name":"account","kind":"OpenAI","location":"eastus2"}]}`
			}
		case "/subscriptions/sub/resourceGroups/rg%20one/providers/Microsoft.CognitiveServices/accounts/account/deployments":
			deploymentReads.Add(1)
			if query.Get("api-version") != "2025-09-01" {
				t.Errorf("deployment API changed: %v", query)
			}
			if query.Get("$skiptoken") == "" {
				body = `{"value":[],"nextLink":"https://management.azure.com/subscriptions/sub/resourceGroups/rg%20one/providers/Microsoft.CognitiveServices/accounts/account/deployments?api-version=2025-09-01&$skiptoken=next"}`
			} else {
				body = fmt.Sprintf(`{"value":[{"id":"%s/deployments/chat","name":"chat","sku":{"name":"GlobalStandard","capacity":999},"properties":{"model":{"name":"test-model","version":"1"},"rateLimits":[{"key":"token","count":%d,"renewalPeriod":60},{"key":"request","count":10,"renewalPeriod":30}]}},{"name":"unknown","properties":{}}]}`, accountID, refresh.Load()*1000)
			}
		default:
			t.Errorf("unexpected request path: %s", request.URL.EscapedPath())
		}
		return testJSONResponse(request, http.StatusOK, body), nil
	})
	p := newProvider(providerOptions{
		runCommand: func(context.Context, ...string) ([]byte, []byte, error) {
			return []byte(`[{"id":"sub","tenantId":"tenant","cloudName":"AzureCloud","state":"Enabled","user":{"name":"test","type":"user"}}]`), nil, nil
		},
		newClient: func(subscriptionInfo) (subscriptionClient, error) { return client, nil },
	})
	for run := int32(1); run <= 2; run++ {
		refresh.Store(run)
		result, err := p.Fetch(context.Background(), nil)
		if err != nil || len(result.Failures) != 0 || len(result.Deployments) != 2 {
			t.Fatalf("Fetch failed: err=%v, failures=%v, deployments=%d", err, result.Failures, len(result.Deployments))
		}
		row := result.Deployments[0]
		if row.TPM == nil || *row.TPM != int64(run)*1000 || row.RPM == nil || *row.RPM != 20 || row.Model != "test-model" || row.SKU != "GlobalStandard" || row.AccountID != accountID || row.Region != "eastus2" {
			t.Fatalf("incorrect or stale row: %+v", row)
		}
		if result.Deployments[1].TPM != nil || result.Deployments[1].RPM != nil || result.TotalAccounts == nil || *result.TotalAccounts != 1 || result.SuccessfulAccounts != 1 {
			t.Fatalf("quantities, kind filtering or totals changed: %+v", result)
		}
	}
	if tokens.Load() != 1 || accountReads.Load() != 4 || deploymentReads.Load() != 4 {
		t.Fatalf("tokens/accounts/deployments = %d/%d/%d, want 1/4/4", tokens.Load(), accountReads.Load(), deploymentReads.Load())
	}
}

func TestSDKPageFailureDiscardsIncompleteAccountOrDeploymentList(t *testing.T) {
	for _, deployments := range []bool{false, true} {
		for _, status := range []int{401, 403, 404, 429, 500} {
			t.Run(fmt.Sprintf("deployments=%t/status=%d", deployments, status), func(t *testing.T) {
				var tokens, calls atomic.Int32
				client := testSDKClient(t, testToken(&tokens), func(request *http.Request) (*http.Response, error) {
					calls.Add(1)
					if request.URL.Query().Get("page") == "" {
						body := fmt.Sprintf(`{"value":[{"name":"first"}],"nextLink":%q}`, request.URL.String()+"&page=2")
						return testJSONResponse(request, 200, body), nil
					}
					return testJSONResponse(request, status, `{"error":{"code":"TestError","message":"test"}}`), nil
				})
				var err error
				var count int
				if deployments {
					rows, e := client.listDeployments(context.Background(), "rg", "account")
					err, count = e, len(rows)
				} else {
					rows, e := client.listAccounts(context.Background())
					err, count = e, len(rows)
				}
				var responseErr *azcore.ResponseError
				if !errors.As(err, &responseErr) || responseErr.StatusCode != status || responseErr.ErrorCode != "TestError" || count != 0 {
					t.Fatalf("incomplete list/error not preserved: rows=%d, err=%v", count, err)
				}
				wantCalls := int32(2)
				if status == 429 || status == 500 {
					wantCalls++ // Standard SDK retry remains enabled.
				}
				if calls.Load() != wantCalls {
					t.Fatalf("HTTP calls=%d, want %d", calls.Load(), wantCalls)
				}
			})
		}
	}
}

func TestSDKRejectsNextLinkOutsideSubscription(t *testing.T) {
	for _, next := range []string{
		"https://other.example/subscriptions/sub/resources",
		"http://management.azure.com/subscriptions/sub/resources",
		"https://management.azure.com/subscriptions/other/resources",
		"https://management.azure.com/subscriptions/sub/../other/resources",
		"https://user@management.azure.com/subscriptions/sub/resources",
	} {
		t.Run(next, func(t *testing.T) {
			var tokens, calls atomic.Int32
			client := testSDKClient(t, testToken(&tokens), func(request *http.Request) (*http.Response, error) {
				calls.Add(1)
				return testJSONResponse(request, 200, fmt.Sprintf(`{"value":[],"nextLink":%q}`, next)), nil
			})
			if _, err := client.listAccounts(context.Background()); err == nil || calls.Load() != 1 {
				t.Fatalf("nextLink accepted: error=%v, calls=%d", err, calls.Load())
			}
		})
	}
}

func TestSDKCancellationAndMalformedJSONFailTheRead(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		t.Run(fmt.Sprint(canceled), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var tokens, calls atomic.Int32
			client := testSDKClient(t, testToken(&tokens), func(request *http.Request) (*http.Response, error) {
				calls.Add(1)
				if canceled {
					cancel()
					return testJSONResponse(request, 200, `{"value":[],"nextLink":"https://management.azure.com/subscriptions/sub/resources?page=2"}`), nil
				}
				return testJSONResponse(request, 200, `{"value":`), nil
			})
			if _, err := client.listAccounts(ctx); err == nil || (canceled && !errors.Is(err, context.Canceled)) || calls.Load() != 1 {
				t.Fatalf("read did not fail correctly: err=%v, calls=%d", err, calls.Load())
			}
		})
	}
}

func TestSDKRefreshesExpiredToken(t *testing.T) {
	var tokens atomic.Int32
	client := testSDKClient(t, testCredential(func(context.Context, policy.TokenRequestOptions) (azcore.AccessToken, error) {
		expiry := time.Now().Add(time.Hour)
		if tokens.Add(1) == 1 {
			expiry = time.Now().Add(-time.Minute)
		}
		return azcore.AccessToken{Token: "test-token", ExpiresOn: expiry}, nil
	}), func(request *http.Request) (*http.Response, error) {
		return testJSONResponse(request, 200, `{"value":[]}`), nil
	})
	for i := 0; i < 3; i++ {
		if _, err := client.listAccounts(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if tokens.Load() != 2 {
		t.Fatalf("token calls=%d, want expired token replaced once", tokens.Load())
	}
}

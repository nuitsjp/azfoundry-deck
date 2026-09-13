package azurego

import (
	"bytes"
	"context"
	"testing"

	armcognitiveservices "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/cognitiveservices/armcognitiveservices/v3"
)

func TestDecodeCLIOutput(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		codePage uint32
		want     []byte
		wantErr  bool
	}{
		{
			name:     "nil",
			data:     nil,
			codePage: 932,
			want:     nil,
		},
		{
			name:     "empty",
			data:     []byte{},
			codePage: 65001,
			want:     []byte{},
		},
		{
			name:     "ASCII and newlines",
			data:     []byte("az account list\r\nnext line\n"),
			codePage: 932,
			want:     []byte("az account list\r\nnext line\n"),
		},
		{
			name:     "CP932 Japanese halfwidth kana and extension",
			data:     []byte{0x93, 0xfa, 0x96, 0x7b, 0x8c, 0xea, 0x0a, 0x94, 0xbc, 0x8a, 0x70, 0xb6, 0xc5, 0x20, 0x87, 0x40},
			codePage: 932,
			want:     []byte("日本語\n半角ｶﾅ ①"),
		},
		{
			name:     "UTF-8 Japanese and emoji",
			data:     []byte("日本語😀\r\n"),
			codePage: 65001,
			want:     []byte("日本語😀\r\n"),
		},
		{
			name:     "embedded NUL is preserved",
			data:     []byte{'A', 0x00, 'B', '\r', '\n'},
			codePage: 65001,
			want:     []byte{'A', 0x00, 'B', '\r', '\n'},
		},
		{
			name:     "CP932 trailing lead byte",
			data:     []byte{0x93},
			codePage: 932,
			wantErr:  true,
		},
		{
			name:     "UTF-8 incomplete sequence",
			data:     []byte{0xe3, 0x81},
			codePage: 65001,
			wantErr:  true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := decodeCLIOutput(test.data, test.codePage)
			if test.wantErr {
				if err == nil {
					t.Fatal("decodeCLIOutput returned nil error")
				}
				if got != nil {
					t.Fatalf("decoded result = %v, want nil", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("decodeCLIOutput returned error: %v", err)
			}
			if !bytes.Equal(got, test.want) {
				t.Fatalf("decoded result = %x (%q), want %x (%q)", got, got, test.want, test.want)
			}
		})
	}
}

func TestCLIJapaneseNamesReachDeployment(t *testing.T) {
	const tenantName = "テナント日本語"
	const subscriptionName = "サブスクリプション日本語"

	account := &armcognitiveservices.Account{
		ID:       stringPointer("/subscriptions/sub-jp/resourceGroups/rg/providers/Microsoft.CognitiveServices/accounts/account-jp"),
		Name:     stringPointer("account-jp"),
		Kind:     stringPointer(accountKindAIServices),
		Location: stringPointer("japaneast"),
	}
	client := &fakeSubscriptionClient{
		accounts:         []*armcognitiveservices.Account{account},
		deployments:      map[string][]*armcognitiveservices.Deployment{"account-jp": {{Name: stringPointer("deployment-jp")}}},
		deploymentErrors: map[string]error{},
	}
	p := testProvider(t, []cliSubscription{{
		ID:                "sub-jp",
		Name:              subscriptionName,
		TenantID:          "tenant-jp",
		TenantDisplayName: tenantName,
		CloudName:         azureCloudName,
		State:             "Enabled",
	}}, map[string]subscriptionClient{"sub-jp": client})

	subscriptions, err := p.listSubscriptions(context.Background())
	if err != nil {
		t.Fatalf("listSubscriptions returned error: %v", err)
	}
	if len(subscriptions) != 1 || subscriptions[0].tenantName != tenantName || subscriptions[0].name != subscriptionName {
		t.Fatalf("subscriptions = %#v, want Japanese tenant and subscription names", subscriptions)
	}

	result, err := p.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if len(result.Deployments) != 1 {
		t.Fatalf("deployments = %#v, want one deployment", result.Deployments)
	}
	deployment := result.Deployments[0]
	if deployment.TenantName != tenantName || deployment.SubscriptionName != subscriptionName {
		t.Fatalf("deployment names = %q, %q; want %q, %q", deployment.TenantName, deployment.SubscriptionName, tenantName, subscriptionName)
	}
}

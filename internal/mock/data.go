package mock

import (
	"strings"

	"github.com/nuitsjp/azfoundry-deck/internal/service"
)

type mockAccount struct {
	tenantID         string
	tenantName       string
	subscriptionID   string
	subscriptionName string
	accountID        string
	accountName      string
	region           string
}

var mockAccounts = []mockAccount{
	{
		tenantID: "tenant-001", tenantName: "Contoso Engineering",
		subscriptionID: "subscription-001", subscriptionName: "Contoso Production",
		accountID: "account-001", accountName: "contoso-chat-prod", region: "eastus",
	},
	{
		tenantID: "tenant-001", tenantName: "Contoso Engineering",
		subscriptionID: "subscription-001", subscriptionName: "Contoso Production",
		accountID: "account-002", accountName: "contoso-chat-dev", region: "japaneast",
	},
	{
		tenantID: "tenant-001", tenantName: "Contoso Engineering",
		subscriptionID: "subscription-002", subscriptionName: "Contoso Sandbox",
		accountID: "account-003", accountName: "contoso-research", region: "westus2",
	},
	{
		tenantID: "tenant-002", tenantName: "Fabrikam Research",
		subscriptionID: "subscription-003", subscriptionName: "Fabrikam Production",
		accountID: "account-004", accountName: "fabrikam-chat-prod", region: "eastus2",
	},
	{
		tenantID: "tenant-002", tenantName: "Fabrikam Research",
		subscriptionID: "subscription-003", subscriptionName: "Fabrikam Production",
		accountID: "account-005", accountName: "fabrikam-ml-prod", region: "swedencentral",
	},
	{
		tenantID: "tenant-002", tenantName: "Fabrikam Research",
		subscriptionID: "subscription-004", subscriptionName: "Fabrikam Development",
		accountID: "account-006", accountName: "fabrikam-lab", region: "australiaeast",
	},
	{
		tenantID: "tenant-002", tenantName: "Fabrikam Research",
		subscriptionID: "subscription-004", subscriptionName: "Fabrikam Development",
		accountID: "account-007", accountName: "fabrikam-chat-dev", region: "eastus",
	},
	{
		tenantID: "tenant-003", tenantName: "Northwind Labs",
		subscriptionID: "subscription-005", subscriptionName: "Northwind Research",
		accountID: "account-008", accountName: "northwind-ai-prod", region: "uksouth",
	},
	{
		tenantID: "tenant-003", tenantName: "Northwind Labs",
		subscriptionID: "subscription-005", subscriptionName: "Northwind Research",
		accountID: "account-009", accountName: "northwind-chat-prod", region: "canadacentral",
	},
	{
		tenantID: "tenant-003", tenantName: "Northwind Labs",
		subscriptionID: "subscription-005", subscriptionName: "Northwind Research",
		accountID: "account-010", accountName: "northwind-evaluation", region: "koreacentral",
	},
}

// resultForScenario builds fresh values on every call. The service may hand a
// result to a UI caller, so scenarios must not share mutable quantity pointers.
func resultForScenario(scenario string) service.DeploymentResult {
	if _, ok := validScenarios[scenario]; !ok {
		panic("invalid mock scenario: " + scenario)
	}
	result := emptyResult()
	totalAccounts := len(mockAccounts)
	result.TotalAccounts = &totalAccounts

	switch scenario {
	case ScenarioSuccess, ScenarioDelayed:
		result.Deployments = allDeployments()
		result.SuccessfulAccounts = totalAccounts
	case ScenarioEmpty:
		result.SuccessfulAccounts = totalAccounts
	case ScenarioPartial:
		result.Deployments = deploymentsForAccounts(0, 6)
		result.SuccessfulAccounts = 6
		result.Failures = partialFailures()
	case ScenarioFailure:
		result.TotalAccounts = nil
		result.Failures = []service.FetchFailure{
			discoveryFailure("Azure CLIセッション", "", "", "not-logged-in", "Azure CLIにサインインしていません。", "az loginを実行してから再試行してください。"),
		}
	case ScenarioCLIMissing:
		result.TotalAccounts = nil
		result.Failures = []service.FetchFailure{
			discoveryFailure("Azure CLI", "", "", "cli-missing", "Azure CLIがインストールされていません。", "Azure CLIをインストールしてから再試行してください。"),
		}
	case ScenarioAllAccountsFailed:
		result.SuccessfulAccounts = 0
		result.Failures = allAccountFailures()
	case ScenarioLoading:
		result.Deployments = allDeployments()
		result.SuccessfulAccounts = totalAccounts
	}
	return result
}

func allDeployments() []service.Deployment {
	return deploymentsForAccounts(0, len(mockAccounts))
}

func deploymentsForAccounts(start, end int) []service.Deployment {
	counts := [...]int{3, 2, 3, 2, 3, 2, 3, 2, 2, 2}
	deployments := make([]service.Deployment, 0, 24)
	deploymentNumber := 1
	for accountIndex := start; accountIndex < end; accountIndex++ {
		account := mockAccounts[accountIndex]
		for deploymentIndex := 0; deploymentIndex < counts[accountIndex]; deploymentIndex++ {
			deployments = append(deployments, makeDeployment(account, deploymentIndex, deploymentNumber))
			deploymentNumber++
		}
	}
	return deployments
}

func makeDeployment(account mockAccount, index, number int) service.Deployment {
	deploymentName := []string{"chat-prod", "embed-prod", "reasoning-prod"}[index%3]
	model := []string{"gpt-4o", "text-embedding-3-large", "o3-mini"}[index%3]
	version := []string{"2024-11-20", "1", "2025-01-31"}[index%3]
	sku := []string{"GlobalStandard", "Standard", "GlobalStandard"}[index%3]
	// TPM/RPM は SKU の Capacity から換算せず、画面確認用に独立して設定した架空値です。
	tpm := int64(120000 - index*20000)
	rpm := int64(1200 - index*200)
	var tpmValue *int64 = &tpm
	var rpmValue *int64 = &rpm
	// Keep all three missing-value combinations in the fixture. A missing
	// quantity is represented by nil all the way to the JSON response.
	switch number % 5 {
	case 0:
		tpmValue = nil
	case 1:
		rpmValue = nil
	case 2:
		tpmValue = nil
		rpmValue = nil
	}
	return service.Deployment{
		ID:               accountResourceID(account) + "/deployments/" + deploymentName,
		Name:             deploymentName,
		TenantID:         account.tenantID,
		TenantName:       account.tenantName,
		SubscriptionID:   account.subscriptionID,
		SubscriptionName: account.subscriptionName,
		AccountID:        accountResourceID(account),
		AccountName:      account.accountName,
		Region:           account.region,
		Model:            model,
		ModelVersion:     version,
		SKU:              sku,
		TPM:              tpmValue,
		RPM:              rpmValue,
	}
}

func accountResourceID(account mockAccount) string {
	return "/subscriptions/" + account.subscriptionID + "/resourceGroups/rg-" + account.accountID + "/providers/Microsoft.CognitiveServices/accounts/" + account.accountName
}

func accountByResourceID(accountID string) (mockAccount, bool) {
	for _, account := range mockAccounts {
		if strings.EqualFold(accountResourceID(account), accountID) {
			return account, true
		}
	}
	return mockAccount{}, false
}

func modelResultForScenario(account mockAccount, scenario string) service.ModelResult {
	if _, ok := validModelScenarios[scenario]; !ok {
		panic("invalid mock model scenario: " + scenario)
	}
	result := service.ModelResult{
		Models:   make([]service.ModelCandidate, 0),
		Failures: make([]service.FetchFailure, 0),
	}
	switch scenario {
	case ModelScenarioSuccess, ModelScenarioDelayed, ModelScenarioLoading:
		result.Models = modelCandidates()
	case ModelScenarioFailure:
		result.Failures = []service.FetchFailure{{
			Scope:            "account",
			TenantName:       account.tenantName,
			SubscriptionName: account.subscriptionName,
			AccountName:      account.accountName,
			Code:             "model-catalog-forbidden",
			Message:          "現在の権限ではモデル候補を読み取れません。",
			Action:           "対象アカウントのモデル一覧読み取り権限を確認して再試行してください。",
		}}
	}
	return result
}

func regionModelResultForScenario(subscriptionID, region, scenario string) service.ModelResult {
	if _, ok := validModelScenarios[scenario]; !ok {
		panic("invalid mock model scenario: " + scenario)
	}
	result := service.ModelResult{
		Models:   make([]service.ModelCandidate, 0),
		Failures: make([]service.FetchFailure, 0),
	}
	switch scenario {
	case ModelScenarioSuccess, ModelScenarioDelayed, ModelScenarioLoading:
		result.Models = modelCandidates()
	case ModelScenarioFailure:
		result.Failures = []service.FetchFailure{{
			Scope:            "region",
			SubscriptionName: subscriptionID,
			AccountName:      region,
			Code:             "region-catalog-forbidden",
			Message:          "現在の権限ではリージョンのモデル候補を読み取れません。",
			Action:           "サブスクリプションの読み取り権限を確認して再試行してください。",
		}}
	}
	return result
}

func modelCandidates() []service.ModelCandidate {
	return []service.ModelCandidate{
		{
			Name:             "gpt-4o",
			Format:           "OpenAI",
			Version:          "2024-05-13",
			Lifecycle:        "GenerallyAvailable",
			IsDefaultVersion: false,
			SKUs:             []service.ModelSKU{modelSKU("GlobalStandard", 10, 1, 100, 1), modelSKU("Standard", 0, 1, 100, 1)},
		},
		{
			Name:             "gpt-4o",
			Format:           "OpenAI",
			Version:          "2024-11-20",
			Lifecycle:        "Deprecating",
			IsDefaultVersion: true,
			SKUs:             []service.ModelSKU{modelSKU("GlobalStandard", 20, 1, 100, 1)},
		},
		{
			Name:             "gpt-5.4-mini",
			Format:           "OpenAI",
			Version:          "2025-04-14",
			Lifecycle:        "GenerallyAvailable",
			IsDefaultVersion: true,
			SKUs:             []service.ModelSKU{modelSKU("GlobalStandard", 10, 1, 100, 1), modelSKU("DataZoneStandard", 10, 1, 50, 1)},
		},
		{
			Name:             "text-embedding-3-large",
			Format:           "OpenAI",
			Version:          "1",
			Lifecycle:        "GenerallyAvailable",
			IsDefaultVersion: true,
			SKUs:             []service.ModelSKU{allowedValuesSKU("Standard", 1, 1, 5, 10)},
		},
		{
			Name:             "o3-mini",
			Format:           "OpenAI",
			Version:          "2025-01-31",
			Lifecycle:        "Preview",
			IsDefaultVersion: false,
			SKUs:             []service.ModelSKU{{Name: "GlobalStandard"}},
		},
		{
			Name:             "custom-model",
			Format:           "",
			Version:          "",
			Lifecycle:        "",
			IsDefaultVersion: false,
			SKUs:             []service.ModelSKU{},
		},
	}
}

// modelSKU builds one SKU whose capacity is expressed as a range. A zero default
// means Azure did not report one for that SKU.
func modelSKU(name string, defaultCapacity, minimum, maximum, step int32) service.ModelSKU {
	capacity := service.CapacityContract{
		Minimum:       &minimum,
		Maximum:       &maximum,
		Step:          &step,
		AllowedValues: []int32{},
	}
	if defaultCapacity > 0 {
		capacity.Default = &defaultCapacity
	}
	return service.ModelSKU{Name: name, Capacity: &capacity}
}

// allowedValuesSKU builds one SKU whose capacity is an explicit value set.
func allowedValuesSKU(name string, defaultCapacity int32, allowed ...int32) service.ModelSKU {
	capacity := service.CapacityContract{Default: &defaultCapacity, AllowedValues: allowed}
	return service.ModelSKU{Name: name, Capacity: &capacity}
}

func partialFailures() []service.FetchFailure {
	return []service.FetchFailure{
		accountFailure(mockAccounts[6], "forbidden", "現在の権限ではアカウントを読み取れません。", "アカウントの読み取り権限を付与して再試行してください。"),
		accountFailure(mockAccounts[7], "communication", "Azureに接続できませんでした。", "接続を確認して再試行してください。"),
		accountFailure(mockAccounts[8], "api-error", "デプロイ一覧取得中にAzureがエラーを返しました。", "Azureのエラーを確認して再試行してください。"),
		accountFailure(mockAccounts[9], "timeout", "アカウントの取得がタイムアウトしました。", "アカウントの取得を再試行してください。"),
	}
}

func allAccountFailures() []service.FetchFailure {
	failures := make([]service.FetchFailure, 0, len(mockAccounts))
	for index, account := range mockAccounts {
		failures = append(failures, accountFailure(account, "account-fetch-failed", "アカウントを読み取れませんでした。", "アカウントの取得を再試行してください。"))
		if index == 0 {
			failures[index].Code = "forbidden"
		}
	}
	return failures
}

func discoveryFailure(scope, tenantName, subscriptionName, code, message, action string) service.FetchFailure {
	return service.FetchFailure{
		Scope:            scope,
		TenantName:       tenantName,
		SubscriptionName: subscriptionName,
		Code:             code,
		Message:          message,
		Action:           action,
	}
}

func accountFailure(account mockAccount, code, message, action string) service.FetchFailure {
	return service.FetchFailure{
		Scope:            account.accountName,
		TenantName:       account.tenantName,
		SubscriptionName: account.subscriptionName,
		AccountName:      account.accountName,
		Code:             code,
		Message:          message,
		Action:           action,
	}
}

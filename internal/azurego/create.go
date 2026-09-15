package azurego

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	armcognitiveservices "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/cognitiveservices/armcognitiveservices/v3"
	"github.com/nuitsjp/azfoundry-deck/internal/service"
)

const resourceGroupAPIVersion = "2021-04-01"

// noWriteRetry disables the SDK's own retries for a write. A negative MaxRetries
// is what turns retrying off; zero would mean the default of three attempts, so
// a dropped connection could resend the same PUT.
var noWriteRetry = policy.RetryOptions{MaxRetries: -1}

// CreateModelDeployment performs one confirmed addition: the resource group and
// Foundry when they are new, then the deployment. Every request is recorded
// before it is sent, and no request is ever resent automatically.
func (p *Provider) CreateModelDeployment(ctx context.Context, request service.CreateRequest) (service.CreateResult, error) {
	result := service.CreateResult{Outcome: service.OutcomeUnknown}
	store, err := newOperationStore()
	if err != nil {
		return stopped(result, service.OutcomeInterrupted, err.Error()), nil
	}
	unlock, err := store.lock()
	if err != nil {
		return stopped(result, service.OutcomeInterrupted, err.Error()), nil
	}
	defer unlock()

	deploymentID := deploymentResourceID(request)
	pending, err := store.unresolved()
	if err != nil {
		return stopped(result, service.OutcomeInterrupted, err.Error()), nil
	}
	for _, record := range pending {
		if strings.EqualFold(record.DeploymentID, deploymentID) {
			return stopped(result, service.OutcomeUnknown,
				"同じデプロイに対する結果不明の操作が残っています。先に状態を確認してください。"), nil
		}
	}

	record := &operationRecord{
		Version:        operationRecordVersion,
		ID:             time.Now().UTC().Format("20060102T150405Z") + "-" + strings.ToLower(request.DeploymentName),
		StartedAt:      time.Now().UTC().Format(time.RFC3339),
		SubscriptionID: request.SubscriptionID,
		Request:        request,
		GroupID:        groupResourceID(request),
		FoundryID:      foundryResourceID(request),
		DeploymentID:   deploymentID,
	}
	result.OperationID = record.ID
	result.FoundryID = record.FoundryID
	result.DeploymentID = record.DeploymentID

	subscription, client, failure, err := p.resolveSubscriptionClient(ctx, request.SubscriptionID, discoveredAccount{})
	if err != nil {
		return result, err
	}
	if failure != nil {
		return stopped(result, service.OutcomeInterrupted, failure.Message+" "+failure.Action), nil
	}
	writer, err := newWriteClient(subscription)
	if err != nil {
		return stopped(result, service.OutcomeInterrupted, err.Error()), nil
	}

	if request.GroupIsNew {
		stage := record.stage(stageGroup)
		stage.State = stateSent
		stage.TargetID = record.GroupID
		if err := store.save(record); err != nil {
			return stopped(result, service.OutcomeInterrupted, err.Error()), nil
		}
		if err := writer.createResourceGroup(ctx, request.ResourceGroup, request.GroupRegion); err != nil {
			stage.State = stateFailed
			stage.Detail = err.Error()
			record.Outcome = service.OutcomeGroupFailure
			_ = store.save(record)
			return stopped(result, service.OutcomeGroupFailure, err.Error()), nil
		}
		stage.State = stateSucceeded
		stage.ObservedAt = time.Now().UTC().Format(time.RFC3339)
		result.CreatedGroup = true
		if err := store.save(record); err != nil {
			return stopped(result, service.OutcomeInterrupted, err.Error()), nil
		}
	}

	if request.FoundryIsNew {
		stage := record.stage(stageFoundry)
		stage.State = stateSent
		stage.TargetID = record.FoundryID
		if err := store.save(record); err != nil {
			return stopped(result, service.OutcomeInterrupted, err.Error()), nil
		}
		if err := writer.createAccount(ctx, request); err != nil {
			stage.State = stateFailed
			stage.Detail = err.Error()
			record.Outcome = service.OutcomeGroupFailure
			_ = store.save(record)
			return stopped(result, service.OutcomeGroupFailure, err.Error()), nil
		}
		stage.State = stateSucceeded
		stage.ObservedAt = time.Now().UTC().Format(time.RFC3339)
		result.CreatedFoundry = true
		if err := store.save(record); err != nil {
			return stopped(result, service.OutcomeInterrupted, err.Error()), nil
		}
	}

	// The confirmed combination is re-checked against the Foundry that now
	// exists. A candidate that is gone stops the operation instead of being
	// replaced by a different model, SKU or capacity.
	if mismatch, err := p.confirmCandidate(ctx, client, request); err != nil {
		record.Outcome = service.OutcomeCandidateMismatch
		_ = store.save(record)
		return stopped(result, service.OutcomeCandidateMismatch, err.Error()), nil
	} else if mismatch != "" {
		record.Outcome = service.OutcomeCandidateMismatch
		_ = store.save(record)
		return stopped(result, service.OutcomeCandidateMismatch, mismatch), nil
	}

	// The deployment names are re-read immediately before the request. An
	// incomplete read stops the operation; it is not treated as a free name.
	existing, err := client.listDeployments(ctx, request.ResourceGroup, request.FoundryName)
	if err != nil {
		record.Outcome = service.OutcomeCandidateMismatch
		_ = store.save(record)
		return stopped(result, service.OutcomeCandidateMismatch,
			"既存のデプロイ名を確認できませんでした。"+describeCause(err)), nil
	}
	for _, deployment := range existing {
		if deployment != nil && strings.EqualFold(value(deployment.Name), request.DeploymentName) {
			record.Outcome = service.OutcomeConflict
			_ = store.save(record)
			return stopped(result, service.OutcomeConflict, ""), nil
		}
	}

	stage := record.stage(stageDeployment)
	stage.State = stateSent
	stage.TargetID = record.DeploymentID
	if err := store.save(record); err != nil {
		return stopped(result, service.OutcomeInterrupted, err.Error()), nil
	}
	requestID, err := writer.createDeployment(ctx, request)
	stage.RequestID = requestID
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			stage.State = stateUnknown
			stage.Detail = err.Error()
			record.Outcome = service.OutcomeUnknown
			_ = store.save(record)
			return stopped(result, service.OutcomeUnknown, ""), nil
		}
		stage.State = stateFailed
		stage.Detail = err.Error()
		record.Outcome = service.OutcomeDeploymentFailure
		_ = store.save(record)
		return stopped(result, service.OutcomeDeploymentFailure, err.Error()), nil
	}
	stage.State = stateSucceeded
	stage.ObservedAt = time.Now().UTC().Format(time.RFC3339)
	record.Outcome = service.OutcomeSuccess
	if err := store.save(record); err != nil {
		// The deployment exists; only the record failed. The screen must not
		// report this as a failed creation.
		result.Outcome = service.OutcomeListFailure
		result.Detail = err.Error()
		return result, nil
	}
	result.Outcome = service.OutcomeSuccess
	return result, nil
}

// CheckCreateOperation re-reads one recorded operation. It only reads: no
// request is resent and nothing is deleted.
func (p *Provider) CheckCreateOperation(ctx context.Context, operationID string) (service.CreateResult, error) {
	result := service.CreateResult{Outcome: service.OutcomeUnknown, OperationID: operationID}
	store, err := newOperationStore()
	if err != nil {
		return stopped(result, service.OutcomeUnknown, err.Error()), nil
	}
	record, err := store.load(operationID)
	if err != nil {
		return stopped(result, service.OutcomeUnknown, err.Error()), nil
	}
	result.FoundryID = record.FoundryID
	result.DeploymentID = record.DeploymentID

	subscription, client, failure, err := p.resolveSubscriptionClient(ctx, record.SubscriptionID, discoveredAccount{})
	_ = subscription
	if err != nil {
		return result, err
	}
	if failure != nil {
		return stopped(result, service.OutcomeUnknown, failure.Message+" "+failure.Action), nil
	}
	deployments, err := client.listDeployments(ctx, record.Request.ResourceGroup, record.Request.FoundryName)
	if err != nil {
		return stopped(result, service.OutcomeUnknown,
			"対象のデプロイ状態を読み取れませんでした。"+describeCause(err)), nil
	}
	for _, deployment := range deployments {
		if deployment == nil || !strings.EqualFold(value(deployment.Name), record.Request.DeploymentName) {
			continue
		}
		state := ""
		if deployment.Properties != nil && deployment.Properties.ProvisioningState != nil {
			state = string(*deployment.Properties.ProvisioningState)
		}
		record.stage(stageDeployment).State = stateSucceeded
		record.stage(stageDeployment).Detail = state
		record.Outcome = service.OutcomeSuccess
		_ = store.save(record)
		result.Outcome = service.OutcomeSuccess
		result.Detail = "Azure上の状態: " + state
		return result, nil
	}
	result.Detail = "対象のデプロイはまだ見つかりません。作成が続いている可能性があります。"
	return result, nil
}

// PendingCreateOperations lists the operations whose result is unsettled.
func (p *Provider) PendingCreateOperations(context.Context) ([]service.PendingOperation, error) {
	store, err := newOperationStore()
	if err != nil {
		return nil, err
	}
	records, err := store.unresolved()
	if err != nil {
		return nil, err
	}
	pending := make([]service.PendingOperation, 0, len(records))
	for _, record := range records {
		pending = append(pending, record.pending())
	}
	return pending, nil
}

// confirmCandidate re-reads the Foundry's candidates and checks that the exact
// confirmed combination is still offered.
func (p *Provider) confirmCandidate(ctx context.Context, client subscriptionClient, request service.CreateRequest) (string, error) {
	models, err := client.listModels(ctx, request.ResourceGroup, request.FoundryName)
	if err != nil {
		return "", fmt.Errorf("作成後のモデル候補を確認できませんでした。%s", describeCause(err))
	}
	for _, candidate := range modelCandidates(models) {
		if !strings.EqualFold(candidate.Format, request.Format) ||
			!strings.EqualFold(candidate.Name, request.Model) ||
			!strings.EqualFold(candidate.Version, request.Version) {
			continue
		}
		for _, sku := range candidate.SKUs {
			if !strings.EqualFold(sku.Name, request.SKU) {
				continue
			}
			if message := capacityMismatch(request.Capacity, sku.Capacity); message != "" {
				return message, nil
			}
			return "", nil
		}
	}
	return "選択した形式・モデル・バージョン・SKUの組み合わせが、このFoundryの候補にありません。", nil
}

func capacityMismatch(capacity int32, contract *service.CapacityContract) string {
	if contract == nil {
		return "選択したSKUの容量条件を取得できません。"
	}
	if len(contract.AllowedValues) > 0 {
		for _, allowed := range contract.AllowedValues {
			if allowed == capacity {
				return ""
			}
		}
		return "選択した容量は、このSKUの許可値に含まれていません。"
	}
	if contract.Minimum != nil && capacity < *contract.Minimum {
		return "選択した容量が、このSKUの最小値を下回っています。"
	}
	if contract.Minimum == nil && capacity < 1 {
		return "容量は 1 以上である必要があります。"
	}
	if contract.Maximum != nil && capacity > *contract.Maximum {
		return "選択した容量が、このSKUの最大値を超えています。"
	}
	if contract.Maximum == nil {
		return "選択したSKUの容量条件を取得できません。"
	}
	return ""
}

// writeClient sends the create requests. Its retry policy differs from the read
// clients: the SDK must never resend a write on its own.
type writeClient struct {
	subscriptionID string
	accounts       *armcognitiveservices.AccountsClient
	deployments    *armcognitiveservices.DeploymentsClient
	raw            *arm.Client
}

func newWriteClient(subscription subscriptionInfo) (*writeClient, error) {
	credential, err := azidentity.NewAzureCLICredential(&azidentity.AzureCLICredentialOptions{
		Subscription: subscription.id,
	})
	if err != nil {
		return nil, fmt.Errorf("Azure CLI 資格情報を初期化できません: %w", err)
	}
	options := &arm.ClientOptions{DisableRPRegistration: true}
	options.Retry = noWriteRetry
	factory, err := armcognitiveservices.NewClientFactory(subscription.id, credential, options)
	if err != nil {
		return nil, fmt.Errorf("Azure ARM SDK を初期化できません: %w", err)
	}
	raw, err := arm.NewClient("github.com/nuitsjp/azfoundry-deck", "v0.0.0", credential, options)
	if err != nil {
		return nil, fmt.Errorf("Azure ARM SDK を初期化できません: %w", err)
	}
	return &writeClient{
		subscriptionID: subscription.id,
		accounts:       factory.NewAccountsClient(),
		deployments:    factory.NewDeploymentsClient(),
		raw:            raw,
	}, nil
}

func stopped(result service.CreateResult, outcome, detail string) service.CreateResult {
	result.Outcome = outcome
	if detail != "" {
		result.Detail = detail
	}
	return result
}

func groupResourceID(request service.CreateRequest) string {
	return "/subscriptions/" + request.SubscriptionID + "/resourceGroups/" + request.ResourceGroup
}

func foundryResourceID(request service.CreateRequest) string {
	return groupResourceID(request) + "/providers/Microsoft.CognitiveServices/accounts/" + request.FoundryName
}

func deploymentResourceID(request service.CreateRequest) string {
	return foundryResourceID(request) + "/deployments/" + request.DeploymentName
}

func describeCause(err error) string {
	_, message, action := describeError(err)
	return strings.TrimSpace(message + " " + action)
}

func (c *writeClient) createResourceGroup(ctx context.Context, name, region string) error {
	body := map[string]string{"location": region}
	requestURL := c.raw.Endpoint() + "/subscriptions/" + url.PathEscape(c.subscriptionID) +
		"/resourcegroups/" + url.PathEscape(name) + "?api-version=" + resourceGroupAPIVersion
	request, err := runtime.NewRequest(ctx, http.MethodPut, requestURL)
	if err != nil {
		return err
	}
	if err := runtime.MarshalAsJSON(request, body); err != nil {
		return err
	}
	response, err := c.raw.Pipeline().Do(request)
	if err != nil {
		return fmt.Errorf("リソースグループを作成できませんでした: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusCreated {
		return fmt.Errorf("リソースグループを作成できませんでした: %s", runtime.NewResponseError(response))
	}
	return nil
}

func (c *writeClient) createAccount(ctx context.Context, request service.CreateRequest) error {
	kind := accountKindAIServices
	skuName := "S0"
	identity := armcognitiveservices.ResourceIdentityTypeSystemAssigned
	publicAccess := armcognitiveservices.PublicNetworkAccessEnabled
	defaultAction := armcognitiveservices.NetworkRuleActionAllow
	disableLocalAuth := true
	account := armcognitiveservices.Account{
		Kind:     &kind,
		Location: &request.FoundryRegion,
		SKU:      &armcognitiveservices.SKU{Name: &skuName},
		Identity: &armcognitiveservices.Identity{Type: &identity},
		Properties: &armcognitiveservices.AccountProperties{
			CustomSubDomainName: &request.FoundryName,
			PublicNetworkAccess: &publicAccess,
			DisableLocalAuth:    &disableLocalAuth,
			NetworkACLs:         &armcognitiveservices.NetworkRuleSet{DefaultAction: &defaultAction},
		},
	}
	poller, err := c.accounts.BeginCreate(ctx, request.ResourceGroup, request.FoundryName, account, nil)
	if err != nil {
		return fmt.Errorf("Foundryを作成できませんでした: %w", err)
	}
	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("Foundryの作成が完了しませんでした: %w", err)
	}
	return nil
}

func (c *writeClient) createDeployment(ctx context.Context, request service.CreateRequest) (string, error) {
	upgrade := armcognitiveservices.DeploymentModelVersionUpgradeOptionNoAutoUpgrade
	capacity := request.Capacity
	deployment := armcognitiveservices.Deployment{
		SKU: &armcognitiveservices.SKU{Name: &request.SKU, Capacity: &capacity},
		Properties: &armcognitiveservices.DeploymentProperties{
			Model: &armcognitiveservices.DeploymentModel{
				Format:  &request.Format,
				Name:    &request.Model,
				Version: &request.Version,
			},
			VersionUpgradeOption: &upgrade,
		},
	}
	var response *http.Response
	sendCtx := policy.WithCaptureResponse(ctx, &response)
	poller, err := c.deployments.BeginCreateOrUpdate(sendCtx, request.ResourceGroup, request.FoundryName, request.DeploymentName, deployment, nil)
	requestID := ""
	if response != nil {
		requestID = response.Header.Get("x-ms-request-id")
	}
	if err != nil {
		return requestID, fmt.Errorf("デプロイを作成できませんでした: %w", err)
	}
	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		return requestID, fmt.Errorf("デプロイの作成が完了しませんでした: %w", err)
	}
	return requestID, nil
}

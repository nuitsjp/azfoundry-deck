package azurego

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	armcognitiveservices "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/cognitiveservices/armcognitiveservices/v3"
	"github.com/nuitsjp/azfoundry-deck/internal/service"
)

// U4 covers what happens when one addition does not finish cleanly: the process
// dies between the record and the response, a later run has to read that record,
// the long-running monitor stops answering, or somebody else creates the same
// name first. These tests drive the same code path the screen uses, with the
// Azure reads and writes replaced and the record directory redirected.

// redirectOperationStore points the user configuration directory at a temporary
// one, so a test never reads or writes the records of the real installation.
func redirectOperationStore(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("AppData", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	return filepath.Join(dir, "AzFoundryDeck", "operations")
}

type fakeWriter struct {
	deploymentCalls int
	deploymentError error
	requestID       string
}

func (w *fakeWriter) createResourceGroup(context.Context, string, string) error { return nil }

func (w *fakeWriter) createAccount(context.Context, service.CreateRequest) error { return nil }

func (w *fakeWriter) createDeployment(context.Context, service.CreateRequest) (string, error) {
	w.deploymentCalls++
	return w.requestID, w.deploymentError
}

// offeredCandidate is the combination the test request confirms, so the
// re-check immediately before the send passes and the test reaches the write.
func offeredCandidate() map[string][]*armcognitiveservices.AccountModel {
	return map[string][]*armcognitiveservices.AccountModel{
		"foundry": {{
			Name:    stringPointer("gpt-5.4-nano"),
			Format:  stringPointer("OpenAI"),
			Version: stringPointer("2026-03-17"),
			SKUs: []*armcognitiveservices.ModelSKU{{
				Name:     stringPointer("GlobalStandard"),
				Capacity: &armcognitiveservices.CapacityConfig{Maximum: int32Pointer(100)},
			}},
		}},
	}
}

func testCreateRequest() service.CreateRequest {
	return service.CreateRequest{
		SubscriptionID: "sub",
		ResourceGroup:  "rg",
		FoundryName:    "foundry",
		Format:         "OpenAI",
		Model:          "gpt-5.4-nano",
		Version:        "2026-03-17",
		SKU:            "GlobalStandard",
		Capacity:       10,
		DeploymentName: "chat",
	}
}

func testCreateProvider(t *testing.T, client subscriptionClient, writer deploymentWriter) *Provider {
	t.Helper()
	provider := testProvider(t,
		[]cliSubscription{{ID: "sub", Name: "Subscription", TenantID: "tenant", CloudName: azureCloudName, State: "Enabled"}},
		map[string]subscriptionClient{"sub": client})
	provider.newWriter = func(subscriptionInfo) (deploymentWriter, error) { return writer, nil }
	return provider
}

func readRecords(t *testing.T, dir string) []operationRecord {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("操作記録を一覧できません: %v", err)
	}
	records := make([]operationRecord, 0, len(entries))
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatalf("操作記録を読み取れません: %v", err)
		}
		var record operationRecord
		if err := json.Unmarshal(content, &record); err != nil {
			t.Fatalf("操作記録を解釈できません: %v", err)
		}
		records = append(records, record)
	}
	return records
}

// U4(4). Somebody else created the same deployment name after the screen
// checked it. The re-query immediately before the send has to stop the
// operation without sending the PUT, because the PUT would update theirs.
func TestExternalSameNameStopsBeforeTheDeploymentIsSent(t *testing.T) {
	dir := redirectOperationStore(t)
	writer := &fakeWriter{}
	client := &fakeSubscriptionClient{
		models:      offeredCandidate(),
		deployments: map[string][]*armcognitiveservices.Deployment{"foundry": {{Name: stringPointer("CHAT")}}},
	}
	result, err := testCreateProvider(t, client, writer).CreateModelDeployment(context.Background(), testCreateRequest())
	if err != nil {
		t.Fatalf("CreateModelDeployment = %v", err)
	}
	if result.Outcome != service.OutcomeConflict {
		t.Fatalf("outcome = %q, want %q", result.Outcome, service.OutcomeConflict)
	}
	if writer.deploymentCalls != 0 {
		t.Fatalf("送信回数 = %d, want 0", writer.deploymentCalls)
	}
	records := readRecords(t, dir)
	if len(records) != 1 || records[0].Outcome != service.OutcomeConflict {
		t.Fatalf("記録された結果が競合停止ではありません: %+v", records)
	}
	for _, stage := range records[0].Stages {
		if stage.Name == stageDeployment {
			t.Fatalf("送っていないデプロイ段階が記録されています: %+v", stage)
		}
	}
}

// U4(2). A record left unresolved by an earlier run is found by the next run,
// and the same deployment is refused instead of being sent a second time.
func TestAnUnresolvedRecordRefusesTheSameDeploymentAfterARestart(t *testing.T) {
	redirectOperationStore(t)
	request := testCreateRequest()
	store, err := newOperationStore()
	if err != nil {
		t.Fatal(err)
	}
	crashed := &operationRecord{
		Version:      operationRecordVersion,
		ID:           "20260915T000000Z-chat",
		StartedAt:    "2026-09-15T00:00:00Z",
		Request:      request,
		DeploymentID: deploymentResourceID(request),
	}
	crashed.stage(stageDeployment).State = stateSent
	if err := store.save(crashed); err != nil {
		t.Fatal(err)
	}

	writer := &fakeWriter{}
	provider := testCreateProvider(t, &fakeSubscriptionClient{models: offeredCandidate()}, writer)

	pending, err := provider.PendingCreateOperations(context.Background())
	if err != nil {
		t.Fatalf("PendingCreateOperations = %v", err)
	}
	if len(pending) != 1 || pending[0].ID != crashed.ID || pending[0].Stage != stageDeployment || pending[0].State != stateSent {
		t.Fatalf("再起動後の未解決記録が読めていません: %+v", pending)
	}

	result, err := provider.CreateModelDeployment(context.Background(), request)
	if err != nil {
		t.Fatalf("CreateModelDeployment = %v", err)
	}
	if result.Outcome != service.OutcomeUnknown || writer.deploymentCalls != 0 {
		t.Fatalf("未解決記録があるのに送信しました: outcome=%q sends=%d", result.Outcome, writer.deploymentCalls)
	}
}

// crashingWriter ends the process inside the deployment send, which is where a
// power loss or a killed process leaves the operation: the request may already
// be on its way to Azure, and nothing has been written since the record.
type crashingWriter struct{ fakeWriter }

func (w *crashingWriter) createDeployment(context.Context, service.CreateRequest) (string, error) {
	os.Exit(7)
	return "", nil
}

// TestCrashHelperProcess is not a test. It is the child process of U4(1): it
// starts one addition against the operation directory the parent prepared and
// dies inside the send.
func TestCrashHelperProcess(t *testing.T) {
	if os.Getenv("AZFOUNDRY_CRASH_HELPER") != "1" {
		t.Skip("親プロセスから起動されたときだけ実行します")
	}
	provider := testCreateProvider(t, &fakeSubscriptionClient{models: offeredCandidate()}, &crashingWriter{})
	_, _ = provider.CreateModelDeployment(context.Background(), testCreateRequest())
	t.Fatal("送信中に終了しませんでした")
}

// U4(1). The process dies after the record is written and while the request is
// being sent. The record has to survive as an unresolved "sent" deployment, and
// the next run must be able to start work again.
func TestProcessDeathDuringTheSendLeavesAResumableRecord(t *testing.T) {
	dir := redirectOperationStore(t)
	command := exec.Command(os.Args[0], "-test.run=TestCrashHelperProcess", "-test.v")
	command.Env = append(os.Environ(), "AZFOUNDRY_CRASH_HELPER=1")
	output, err := command.CombinedOutput()
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) || exitError.ExitCode() != 7 {
		t.Fatalf("子プロセスが送信中に終了していません: err=%v, 出力=%s", err, output)
	}

	records := readRecords(t, dir)
	if len(records) != 1 {
		t.Fatalf("操作記録は1件であるべきです: %+v", records)
	}
	record := records[0]
	if record.Outcome != "" {
		t.Fatalf("結果が確定していないのに %q が記録されています", record.Outcome)
	}
	stage := record.stage(stageDeployment)
	if stage.State != stateSent || stage.TargetID != deploymentResourceID(testCreateRequest()) {
		t.Fatalf("送信済み段階が記録されていません: %+v", stage)
	}

	// The next run reads the record instead of sending the same PUT again.
	writer := &fakeWriter{}
	provider := testCreateProvider(t, &fakeSubscriptionClient{models: offeredCandidate()}, writer)
	pending, err := provider.PendingCreateOperations(context.Background())
	if err != nil {
		t.Fatalf("PendingCreateOperations = %v", err)
	}
	if len(pending) != 1 || pending[0].State != stateSent {
		t.Fatalf("異常終了した操作が未解決として読めていません: %+v", pending)
	}

	// A different deployment must still be possible. A lock left behind by the
	// dead process would block every later addition.
	other := testCreateRequest()
	other.DeploymentName = "another"
	result, err := provider.CreateModelDeployment(context.Background(), other)
	if err != nil {
		t.Fatalf("CreateModelDeployment = %v", err)
	}
	if result.Outcome != service.OutcomeSuccess {
		t.Fatalf("異常終了後に別の追加ができません: outcome=%q detail=%q", result.Outcome, result.Detail)
	}
}

// TestLockHelperProcess is not a test. It is the child process that holds the
// write lock while the parent checks that a second addition is refused.
func TestLockHelperProcess(t *testing.T) {
	if os.Getenv("AZFOUNDRY_LOCK_HELPER") != "1" {
		t.Skip("親プロセスから起動されたときだけ実行します")
	}
	store, err := newOperationStore()
	if err != nil {
		t.Fatal(err)
	}
	unlock, err := store.lock()
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	fmt.Println("locked")
	_, _ = io.ReadAll(os.Stdin)
}

// A lock whose owner is still running must keep its hold. The takeover added
// for U4(1) must not turn the lock into no protection at all.
func TestALiveOwnerKeepsTheWriteLock(t *testing.T) {
	redirectOperationStore(t)
	command := exec.Command(os.Args[0], "-test.run=TestLockHelperProcess")
	command.Env = append(os.Environ(), "AZFOUNDRY_LOCK_HELPER=1")
	input, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = input.Close()
		_ = command.Wait()
	}()
	reader := bufio.NewReader(output)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("子プロセスがロックを取得できませんでした: %v", err)
		}
		if strings.TrimSpace(line) == "locked" {
			break
		}
	}

	store, err := newOperationStore()
	if err != nil {
		t.Fatal(err)
	}
	if unlock, err := store.lock(); err == nil {
		unlock()
		t.Fatal("実行中の追加があるのにロックを奪いました")
	}
}

// testWriteClient builds the real write client over a test transport, so the
// SDK's own long-running-operation polling is exercised.
func testWriteClient(t *testing.T, transport testTransport) deploymentWriter {
	t.Helper()
	options := &arm.ClientOptions{DisableRPRegistration: true}
	options.Transport = transport
	options.Retry = noWriteRetry
	var tokens atomic.Int32
	factory, err := armcognitiveservices.NewClientFactory("sub", testToken(&tokens), options)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := arm.NewClient("azfoundry-test", "v0.0.0", testToken(&tokens), options)
	if err != nil {
		t.Fatal(err)
	}
	return &writeClient{
		subscriptionID: "sub",
		accounts:       factory.NewAccountsClient(),
		deployments:    factory.NewDeploymentsClient(),
		raw:            raw,
	}
}

// U4(3). Azure accepted the deployment request and returned a monitor URL that
// later stops answering. The request is on Azure's side, so the result is
// unknown: reporting a failed creation would be untrue, and resending the PUT
// is forbidden.
func TestAnExpiredMonitorURLIsReportedAsAnUnknownResultNotAFailure(t *testing.T) {
	dir := redirectOperationStore(t)
	var sends, polls atomic.Int32
	writer := testWriteClient(t, func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodPut {
			sends.Add(1)
			response := testJSONResponse(request, http.StatusCreated, `{"name":"chat","properties":{"provisioningState":"Accepted"}}`)
			response.Header.Set("Azure-AsyncOperation", "https://management.azure.com/subscriptions/sub/providers/Microsoft.CognitiveServices/locations/japaneast/operations/gone?api-version=2025-09-01")
			response.Header.Set("x-ms-request-id", "request-1")
			return response, nil
		}
		polls.Add(1)
		return testJSONResponse(request, http.StatusNotFound, `{"error":{"code":"OperationNotFound","message":"The operation was not found."}}`), nil
	})
	provider := testCreateProvider(t, &fakeSubscriptionClient{models: offeredCandidate()}, writer)

	result, err := provider.CreateModelDeployment(context.Background(), testCreateRequest())
	if err != nil {
		t.Fatalf("CreateModelDeployment = %v", err)
	}
	if sends.Load() != 1 {
		t.Fatalf("PUT送信回数 = %d, want 1", sends.Load())
	}
	if result.Outcome != service.OutcomeUnknown {
		t.Fatalf("outcome = %q, want %q (受理済みの要求を失敗と報告してはいけません)", result.Outcome, service.OutcomeUnknown)
	}
	records := readRecords(t, dir)
	if len(records) != 1 {
		t.Fatalf("操作記録は1件であるべきです: %+v", records)
	}
	stage := records[0].stage(stageDeployment)
	if stage.State != stateUnknown || stage.RequestID != "request-1" {
		t.Fatalf("段階が結果不明として記録されていません: %+v", stage)
	}
	if records[0].Outcome != service.OutcomeUnknown {
		t.Fatalf("記録された結果 = %q, want %q", records[0].Outcome, service.OutcomeUnknown)
	}
}

// The state check is the only way out of an unknown result, so it has to report
// what Azure actually says. A deployment that exists is not by itself a
// successful creation.
func TestTheStateCheckReportsTheProvisioningStateAzureReports(t *testing.T) {
	for _, test := range []struct {
		name        string
		state       string
		wantOutcome string
		wantSettled bool
	}{
		{name: "succeeded", state: "Succeeded", wantOutcome: service.OutcomeSuccess, wantSettled: true},
		{name: "failed", state: "Failed", wantOutcome: service.OutcomeDeploymentFailure, wantSettled: true},
		{name: "canceled", state: "Canceled", wantOutcome: service.OutcomeDeploymentFailure, wantSettled: true},
		{name: "still creating", state: "Creating", wantOutcome: service.OutcomeUnknown},
		{name: "not found yet", state: "", wantOutcome: service.OutcomeUnknown},
	} {
		t.Run(test.name, func(t *testing.T) {
			redirectOperationStore(t)
			request := testCreateRequest()
			store, err := newOperationStore()
			if err != nil {
				t.Fatal(err)
			}
			record := &operationRecord{
				Version:        operationRecordVersion,
				ID:             "20260915T000000Z-chat",
				StartedAt:      "2026-09-15T00:00:00Z",
				SubscriptionID: "sub",
				Request:        request,
				DeploymentID:   deploymentResourceID(request),
				Outcome:        service.OutcomeUnknown,
			}
			record.stage(stageDeployment).State = stateUnknown
			if err := store.save(record); err != nil {
				t.Fatal(err)
			}

			deployments := []*armcognitiveservices.Deployment{}
			if test.state != "" {
				state := armcognitiveservices.DeploymentProvisioningState(test.state)
				deployments = append(deployments, &armcognitiveservices.Deployment{
					Name:       stringPointer("chat"),
					Properties: &armcognitiveservices.DeploymentProperties{ProvisioningState: &state},
				})
			}
			client := &fakeSubscriptionClient{deployments: map[string][]*armcognitiveservices.Deployment{"foundry": deployments}}
			provider := testCreateProvider(t, client, &fakeWriter{})

			result, err := provider.CheckCreateOperation(context.Background(), record.ID)
			if err != nil {
				t.Fatalf("CheckCreateOperation = %v", err)
			}
			if result.Outcome != test.wantOutcome {
				t.Fatalf("outcome = %q, want %q (detail=%q)", result.Outcome, test.wantOutcome, result.Detail)
			}
			stored, err := store.load(record.ID)
			if err != nil {
				t.Fatal(err)
			}
			settled := stored.Outcome != service.OutcomeUnknown && stored.Outcome != "" && stored.Outcome != service.OutcomeInterrupted
			if settled != test.wantSettled {
				t.Fatalf("記録された結果 = %q, 確定済み=%v want=%v", stored.Outcome, settled, test.wantSettled)
			}
		})
	}
}

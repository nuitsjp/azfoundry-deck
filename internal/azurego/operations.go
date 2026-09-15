package azurego

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/nuitsjp/azfoundry-deck/internal/service"
)

// The operation record exists so a write is never sent twice. It is written
// before every request and read back after a restart. It holds no token, key or
// response body.

const operationRecordVersion = 1

type operationStage struct {
	Name         string `json:"name"`
	State        string `json:"state"`
	TargetID     string `json:"targetId"`
	RequestID    string `json:"requestId,omitempty"`
	OperationURL string `json:"operationUrl,omitempty"`
	ObservedAt   string `json:"observedAt,omitempty"`
	Detail       string `json:"detail,omitempty"`
}

type operationRecord struct {
	Version        int                   `json:"version"`
	ID             string                `json:"id"`
	StartedAt      string                `json:"startedAt"`
	UpdatedAt      string                `json:"updatedAt"`
	SubscriptionID string                `json:"subscriptionId"`
	Request        service.CreateRequest `json:"request"`
	GroupID        string                `json:"groupId"`
	FoundryID      string                `json:"foundryId"`
	DeploymentID   string                `json:"deploymentId"`
	Stages         []operationStage      `json:"stages"`
	Outcome        string                `json:"outcome"`
}

// Stage names and states of one addition.
const (
	stageGroup      = "resource-group"
	stageFoundry    = "foundry"
	stageDeployment = "deployment"

	stateSent      = "sent"
	stateSucceeded = "succeeded"
	stateFailed    = "failed"
	stateUnknown   = "unknown"
)

type operationStore struct {
	dir string
}

func newOperationStore() (*operationStore, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("操作記録の保存先を決定できません: %w", err)
	}
	dir := filepath.Join(base, "AzFoundryDeck", "operations")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("操作記録の保存先を作成できません: %w", err)
	}
	return &operationStore{dir: dir}, nil
}

func (s *operationStore) path(id string) string {
	return filepath.Join(s.dir, id+".json")
}

// lock keeps a single Go process writing operation records for this Windows
// user. It does not prevent a write made from the Azure Portal or another
// machine.
func (s *operationStore) lock() (func(), error) {
	path := filepath.Join(s.dir, "write.lock")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil, errors.New("別のモデル追加が進行中です。完了を待つか、状態を確認してから再試行してください。")
		}
		return nil, fmt.Errorf("操作記録の排他制御に失敗しました: %w", err)
	}
	_, _ = file.WriteString(time.Now().UTC().Format(time.RFC3339))
	_ = file.Close()
	return func() { _ = os.Remove(path) }, nil
}

// save writes the record and flushes it before the caller sends a request. A
// failed save must stop the send.
func (s *operationStore) save(record *operationRecord) error {
	record.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	content, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("操作記録を作成できません: %w", err)
	}
	temporary := s.path(record.ID) + ".tmp"
	file, err := os.OpenFile(temporary, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("操作記録を書き込めません: %w", err)
	}
	if _, err := file.Write(content); err != nil {
		_ = file.Close()
		return fmt.Errorf("操作記録を書き込めません: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("操作記録を保存できません: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("操作記録を保存できません: %w", err)
	}
	if err := os.Rename(temporary, s.path(record.ID)); err != nil {
		return fmt.Errorf("操作記録を確定できません: %w", err)
	}
	return nil
}

func (s *operationStore) load(id string) (*operationRecord, error) {
	content, err := os.ReadFile(s.path(id))
	if err != nil {
		return nil, fmt.Errorf("操作記録を読み取れません: %w", err)
	}
	var record operationRecord
	if err := json.Unmarshal(content, &record); err != nil {
		return nil, fmt.Errorf("操作記録が壊れています: %w", err)
	}
	if record.Version != operationRecordVersion {
		return nil, fmt.Errorf("操作記録の形式が想定と異なります: version=%d", record.Version)
	}
	return &record, nil
}

// unresolved lists the records whose outcome is not settled. A record that
// cannot be read is reported as an error, never skipped as an empty history.
func (s *operationStore) unresolved() ([]*operationRecord, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("操作記録を一覧できません: %w", err)
	}
	records := make([]*operationRecord, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		record, err := s.load(strings.TrimSuffix(name, ".json"))
		if err != nil {
			return nil, err
		}
		if record.Outcome == "" || record.Outcome == service.OutcomeUnknown || record.Outcome == service.OutcomeInterrupted {
			records = append(records, record)
		}
	}
	sort.Slice(records, func(i, j int) bool { return records[i].StartedAt < records[j].StartedAt })
	return records, nil
}

func (record *operationRecord) stage(name string) *operationStage {
	for index := range record.Stages {
		if record.Stages[index].Name == name {
			return &record.Stages[index]
		}
	}
	record.Stages = append(record.Stages, operationStage{Name: name})
	return &record.Stages[len(record.Stages)-1]
}

func (record *operationRecord) pending() service.PendingOperation {
	stage := ""
	state := ""
	for _, item := range record.Stages {
		if item.State != stateSucceeded {
			stage = item.Name
			state = item.State
			break
		}
	}
	return service.PendingOperation{
		ID:             record.ID,
		StartedAt:      record.StartedAt,
		SubscriptionID: record.SubscriptionID,
		DeploymentID:   record.DeploymentID,
		Stage:          stage,
		State:          state,
	}
}

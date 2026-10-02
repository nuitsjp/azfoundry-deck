package azauth

import (
	"encoding/json"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

// LoginRecord stores account identity and the tenant list from the last sign-in.
type LoginRecord struct {
	Record           azidentity.AuthenticationRecord
	Tenants          []Tenant `json:"tenants"`
	SelectedTenantID string   `json:"selectedTenantId"`
}

func (r LoginRecord) MarshalJSON() ([]byte, error) {
	data, err := json.Marshal(r.Record)
	if err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	fields["tenants"], err = json.Marshal(r.Tenants)
	if err != nil {
		return nil, err
	}
	fields["selectedTenantId"], err = json.Marshal(r.SelectedTenantID)
	if err != nil {
		return nil, err
	}
	return json.Marshal(fields)
}

func (r *LoginRecord) UnmarshalJSON(data []byte) error {
	if err := json.Unmarshal(data, &r.Record); err != nil {
		return err
	}
	var selection struct {
		Tenants          []Tenant `json:"tenants"`
		SelectedTenantID string   `json:"selectedTenantId"`
	}
	if err := json.Unmarshal(data, &selection); err != nil {
		return err
	}
	r.Tenants, r.SelectedTenantID = selection.Tenants, selection.SelectedTenantID
	return nil
}

func (r LoginRecord) account() (Account, error) {
	for _, tenant := range r.Tenants {
		if tenant.ID != "" && tenant.DisplayName != "" && tenant.ID == r.SelectedTenantID {
			return Account{Username: r.Record.Username, Tenants: r.Tenants, SelectedTenantID: r.SelectedTenantID}, nil
		}
	}
	return Account{}, fmt.Errorf("保存済みのテナント選択がありません。サインインし直してください。")
}

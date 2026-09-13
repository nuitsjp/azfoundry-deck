package service

// CompatibilityService は U2 の Go / TypeScript 間の接続確認に使用する。
// F1 のサービス契約はモック作成時に確定する。
type CompatibilityService struct{}

type DeploymentSample struct {
	Name         string `json:"name"`
	Model        string `json:"model"`
	ModelVersion string `json:"modelVersion"`
	SKU          string `json:"sku"`
	Capacity     *int32 `json:"capacity"`
	State        string `json:"state"`
}

func (*CompatibilityService) GetDeploymentSample() DeploymentSample {
	return DeploymentSample{
		Name:         "example-deployment",
		Model:        "example-model",
		ModelVersion: "example-version",
		SKU:          "GlobalStandard",
		Capacity:     nil,
		State:        "Succeeded",
	}
}

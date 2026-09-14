// Fixed UI-agreement data. Only injected when the application is in mock mode.
export const addModelMock = {
  subscriptions: [
    { id: "subscription-001", name: "Contoso Production", tenant: "Contoso Engineering" },
    { id: "subscription-002", name: "Contoso Sandbox", tenant: "Contoso Engineering" },
  ],
  groups: [
    { name: "rg-production", subscription: "subscription-001" },
    { name: "rg-empty", subscription: "subscription-001" },
    { name: "rg-sandbox", subscription: "subscription-002" },
  ],
  foundries: [
    { name: "contoso-chat-prod", subscription: "subscription-001", group: "rg-production", region: "eastus", empty: false },
    { name: "contoso-first-model", subscription: "subscription-001", group: "rg-production", region: "japaneast", empty: true },
  ],
  models: [
    { name: "gpt-4o", defaultVersion: "2024-08-06", defaultSku: "GlobalStandard", versions: ["2024-08-06", "2024-11-20"], skus: ["GlobalStandard", "Standard"] },
    { name: "gpt-5.4-mini", defaultVersion: "2026-03-17", defaultSku: "GlobalStandard", versions: ["2026-03-17"], skus: ["GlobalStandard"] },
    { name: "text-embedding-3-large", defaultVersion: "1", defaultSku: "Standard", versions: ["1"], skus: ["Standard"] },
  ],
};

export type ModelLoadScenario = "success" | "slow" | "empty" | "failure";

// The UI receives an asynchronous boundary; no Azure resources are created here.
export async function loadMockModels(_foundry: string, scenario: ModelLoadScenario) {
  await new Promise(resolve => setTimeout(resolve, scenario === "slow" ? 8000 : 800));
  if (scenario === "failure") throw new Error("モデル候補を取得できませんでした。再試行してください。");
  return scenario === "empty" ? [] : addModelMock.models;
}

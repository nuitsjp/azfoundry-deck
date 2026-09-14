// Fixed UI-agreement data. Only injected when the application is in mock mode.
export type CapacityContract = {
  default?: number;
  min?: number;
  max?: number;
  step?: number;
  allowedValues?: number[];
};

export type ModelSku = { name: string; capacity?: CapacityContract };
export type ModelVersion = { version: string; isDefault?: boolean; skus: ModelSku[] };
export type ModelCandidate = { format: string; name: string; versions: ModelVersion[] };

export type ModelLoadScenario = "success" | "slow" | "empty" | "failure" | "no-default" | "missing-capacity";
export type CreateResultScenario =
  | "success"
  | "group-failure"
  | "foundry-candidate-mismatch"
  | "deployment-failure"
  | "unknown"
  | "conflict"
  | "list-failure"
  | "interrupted";

const standardGptVersion: ModelVersion = {
  version: "2024-08-06",
  isDefault: true,
  skus: [
    { name: "Standard", capacity: { min: 1, max: 100, step: 1 } },
    { name: "GlobalStandard", capacity: { default: 10, min: 1, max: 100, step: 1 } },
  ],
};

const newerGptVersion: ModelVersion = {
  version: "2024-11-20",
  skus: [
    { name: "GlobalStandard", capacity: { default: 20, min: 1, max: 100, step: 1 } },
    { name: "Standard", capacity: { min: 1, max: 100, step: 1 } },
  ],
};

const normalModels: ModelCandidate[] = [
  { format: "OpenAI", name: "gpt-4o", versions: [standardGptVersion, newerGptVersion] },
  {
    format: "OpenAI",
    name: "gpt-5.4-mini",
    versions: [{ version: "2026-03-17", isDefault: true, skus: [{ name: "GlobalStandard", capacity: { default: 10, min: 1, max: 100, step: 1 } }] }],
  },
  {
    format: "OpenAI",
    name: "text-embedding-3-large",
    versions: [{ version: "1", isDefault: true, skus: [{ name: "Standard", capacity: { default: 1, allowedValues: [1, 5, 10] } }] }],
  },
];

const modelsWithoutDefault: ModelCandidate[] = [
  {
    format: "OpenAI",
    name: "gpt-no-default",
    versions: [
      { version: "2026-01-01", skus: [{ name: "GlobalStandard", capacity: { default: 5, min: 1, max: 20, step: 1 } }] },
      { version: "2026-02-01", skus: [{ name: "Standard", capacity: { min: 1, max: 20, step: 1 } }, { name: "DataZoneStandard", capacity: { default: 5, min: 1, max: 20, step: 1 } }] },
    ],
  },
];

const modelsWithMissingCapacity: ModelCandidate[] = [
  {
    format: "OpenAI",
    name: "gpt-capacity-unknown",
    versions: [{ version: "2026-03-01", isDefault: true, skus: [{ name: "GlobalStandard" }] }],
  },
];

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
    {
      name: "contoso-chat-prod",
      subscription: "subscription-001",
      group: "rg-production",
      region: "eastus",
      empty: false,
      deployments: ["chat-production"],
    },
    {
      name: "contoso-first-model",
      subscription: "subscription-001",
      group: "rg-production",
      region: "japaneast",
      empty: true,
      deployments: [],
    },
  ],
  models: normalModels,
};

// The UI receives an asynchronous boundary; no Azure resources are created here.
export async function loadMockModels(_foundry: string, scenario: ModelLoadScenario): Promise<ModelCandidate[]> {
  await new Promise(resolve => setTimeout(resolve, scenario === "slow" ? 8000 : 800));
  if (scenario === "failure") throw new Error("モデル候補を取得できませんでした。再試行してください。");
  if (scenario === "empty") return [];
  if (scenario === "no-default") return modelsWithoutDefault;
  if (scenario === "missing-capacity") return modelsWithMissingCapacity;
  return normalModels;
}

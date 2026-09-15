// Screen-facing shapes for the model addition dialog, and the conversion from
// the Go service results. The Go services return one flat row per model version;
// the dialog selects a model first, then a version, then a SKU, so the rows are
// grouped here without losing any reported combination.
import type { ModelResult, PlacementResult } from "../bindings/github.com/nuitsjp/azfoundry-deck/internal/service/models";

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

export type CatalogSubscription = { id: string; name: string; tenant: string };
export type CatalogGroup = { subscription: string; name: string };
export type CatalogFoundry = {
  id: string;
  subscription: string;
  group: string;
  name: string;
  region: string;
  empty: boolean;
};
export type AddModelCatalog = {
  subscriptions: CatalogSubscription[];
  groups: CatalogGroup[];
  foundries: CatalogFoundry[];
};

export const emptyCatalog: AddModelCatalog = { subscriptions: [], groups: [], foundries: [] };

const RETIRED_LIFECYCLES = new Set(["deprecating", "deprecated"]);

function optional(value: number | null): number | undefined {
  return value === null ? undefined : value;
}

// toCapacity keeps a missing constraint missing. The dialog refuses the
// combination instead of substituting a value Azure did not report.
function toCapacity(capacity: ModelResult["models"][number]["skus"][number]["capacity"]): CapacityContract | undefined {
  if (!capacity) return undefined;
  const contract: CapacityContract = {
    default: optional(capacity.default),
    min: optional(capacity.minimum),
    max: optional(capacity.maximum),
    step: optional(capacity.step),
  };
  if (capacity.allowedValues.length > 0) contract.allowedValues = capacity.allowedValues;
  return contract;
}

// toCandidates groups the flat rows by format and model name, keeping the order
// the service returned them in.
export function toCandidates(result: ModelResult): ModelCandidate[] {
  const candidates: ModelCandidate[] = [];
  for (const row of result.models) {
    if (!row.format || !row.name || !row.version) continue;
    // A retiring model is still listed, but Azure refuses a new deployment of it
    // with ServiceModelDeprecating. Offering it would only produce a failure.
    if (RETIRED_LIFECYCLES.has(row.lifecycle.toLowerCase())) continue;
    let candidate = candidates.find(item => item.format === row.format && item.name === row.name);
    if (!candidate) {
      candidate = { format: row.format, name: row.name, versions: [] };
      candidates.push(candidate);
    }
    let version = candidate.versions.find(item => item.version === row.version);
    if (!version) {
      version = { version: row.version, isDefault: row.isDefaultVersion, skus: [] };
      candidate.versions.push(version);
    }
    for (const sku of row.skus) {
      if (!sku.name || version.skus.some(item => item.name === sku.name)) continue;
      version.skus.push({ name: sku.name, capacity: toCapacity(sku.capacity) });
    }
  }
  return candidates;
}

// modelFailureMessage turns a reported read failure into the screen's error
// text. An empty result without a failure is "no candidates", not an error.
export function modelFailureMessage(result: ModelResult): string {
  const failure = result.failures[0];
  if (!failure) return "";
  return [failure.message, failure.action].filter(Boolean).join(" ");
}

// toCatalog builds the placement choices. A Foundry counts as empty when the
// deployment list holds no row for it, so a Foundry that has never been
// deployed to stays selectable and is marked as such.
export function toCatalog(result: PlacementResult, accountsWithDeployments: Set<string>): AddModelCatalog {
  const catalog: AddModelCatalog = { subscriptions: [], groups: [], foundries: [] };
  for (const subscription of result.subscriptions) {
    catalog.subscriptions.push({ id: subscription.id, name: subscription.name, tenant: subscription.tenantName });
    for (const group of subscription.groups) {
      catalog.groups.push({ subscription: subscription.id, name: group.name });
    }
    for (const foundry of subscription.foundries) {
      catalog.foundries.push({
        id: foundry.id,
        subscription: subscription.id,
        group: foundry.resourceGroup,
        name: foundry.name,
        region: foundry.region,
        empty: !accountsWithDeployments.has(foundry.id.toLowerCase()),
      });
    }
  }
  return catalog;
}

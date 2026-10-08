import { GetCostState, RefreshCost } from '@bindings/azfoundrydeck/internal/foundry/service';

export async function getCostState() {
  return GetCostState();
}

export async function refreshCost() {
  return RefreshCost();
}

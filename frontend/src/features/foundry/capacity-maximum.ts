import { GetCapacityState } from '@bindings/azfoundrydeck/internal/foundry/service';

export async function getCapacityState(retry = false) {
  return GetCapacityState(retry);
}

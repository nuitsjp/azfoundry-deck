import { GetConnectionState } from '@bindings/azfoundrydeck/internal/foundry/service';

export async function getConnectionState() {
  return GetConnectionState();
}

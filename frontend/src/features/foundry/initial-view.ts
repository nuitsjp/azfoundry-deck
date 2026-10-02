import { GetInitialView } from '@bindings/azfoundrydeck/internal/foundry/service';
import type { InitialFoundryView } from './models';

export const loadInitialView = () => GetInitialView() as Promise<InitialFoundryView>;

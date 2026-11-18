import { useMutation, useMutationState, useQuery, useQueryClient } from '@tanstack/react-query';
import { ThemeMode as BindingThemeMode } from '@bindings/azfoundrydeck/internal/settings/models';
import * as SettingsService from '@bindings/azfoundrydeck/internal/settings/service';

export type ThemeMode = 'dark' | 'light';

const THEME_QUERY_KEY = ['settings', 'theme'] as const;
export const saveThemeKey = ['settings', 'save-theme'] as const;

function toThemeMode(value: BindingThemeMode | string): ThemeMode {
  return value === BindingThemeMode.ThemeLight ? 'light' : 'dark';
}

function toBindingThemeMode(value: ThemeMode): BindingThemeMode {
  return value === 'light' ? BindingThemeMode.ThemeLight : BindingThemeMode.ThemeDark;
}

export const themeQueryOptions = () => ({
  queryKey: THEME_QUERY_KEY,
  queryFn: async (): Promise<ThemeMode> => toThemeMode(await SettingsService.GetTheme()),
  staleTime: Infinity,
  gcTime: Infinity,
  retry: false,
});

export function useThemeQuery() {
  return useQuery(themeQueryOptions());
}

export function useSaveTheme() {
  const client = useQueryClient();
  return useMutation({
    mutationKey: saveThemeKey,
    mutationFn: async (theme: ThemeMode): Promise<ThemeMode> =>
      toThemeMode(await SettingsService.SetTheme(toBindingThemeMode(theme))),
    onSuccess: (saved) => {
      client.setQueryData(THEME_QUERY_KEY, saved);
    },
  });
}

export function useLatestSaveThemeState() {
  const states = useMutationState({
    filters: { mutationKey: saveThemeKey },
    select: (mutation) => mutation.state,
  });
  return states[states.length - 1];
}

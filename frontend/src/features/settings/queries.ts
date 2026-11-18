import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

export type ThemeMode = 'dark' | 'light';

const THEME_QUERY_KEY = ['settings', 'theme'] as const;
const MOCK_SESSION_KEY = 'azfoundrydeck.mock.theme';

function readMockTheme(): ThemeMode {
  try {
    const value = window.sessionStorage.getItem(MOCK_SESSION_KEY);
    return value === 'light' ? 'light' : 'dark';
  } catch {
    return 'dark';
  }
}

function writeMockTheme(theme: ThemeMode): ThemeMode {
  try {
    window.sessionStorage.setItem(MOCK_SESSION_KEY, theme);
  } catch {
    // Keep in-memory state when session storage is unavailable during mock review.
  }
  return theme;
}

export const themeQueryOptions = () => ({
  queryKey: THEME_QUERY_KEY,
  queryFn: async (): Promise<ThemeMode> => readMockTheme(),
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
    mutationFn: async (theme: ThemeMode): Promise<ThemeMode> => writeMockTheme(theme),
    onSuccess: (saved) => {
      client.setQueryData(THEME_QUERY_KEY, saved);
    },
  });
}

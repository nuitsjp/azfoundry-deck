import { ActionIcon, Tooltip, useMantineColorScheme } from '@mantine/core';
import { useEffect } from 'react';
import { useSaveTheme, useThemeQuery, type ThemeMode } from '../../features/settings/queries';

function SunIcon() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" aria-hidden="true">
      <g
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <circle cx="12" cy="12" r="4" />
        <path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M6.34 17.66l-1.41 1.41M19.07 4.93l-1.41 1.41" />
      </g>
    </svg>
  );
}

function MoonIcon() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" aria-hidden="true">
      <path
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"
      />
    </svg>
  );
}

export function ThemeSync() {
  const { setColorScheme } = useMantineColorScheme();
  const theme = useThemeQuery();

  useEffect(() => {
    if (theme.data) {
      setColorScheme(theme.data);
    }
  }, [theme.data, setColorScheme]);

  return null;
}

export function ThemeToggle() {
  const { colorScheme, setColorScheme } = useMantineColorScheme();
  const theme = useThemeQuery();
  const save = useSaveTheme();
  const current: ThemeMode = theme.data ?? (colorScheme === 'light' ? 'light' : 'dark');
  const next: ThemeMode = current === 'dark' ? 'light' : 'dark';
  const label = next === 'light' ? 'ライトテーマに切り替え' : 'ダークテーマに切り替え';

  return (
    <Tooltip label={label} position="bottom">
      <ActionIcon
        variant="subtle"
        color="gray"
        size="lg"
        aria-label={label}
        disabled={save.isPending}
        onClick={() => {
          save.mutate(next, {
            onSuccess: (saved) => {
              setColorScheme(saved);
            },
          });
        }}
      >
        {current === 'dark' ? <SunIcon /> : <MoonIcon />}
      </ActionIcon>
    </Tooltip>
  );
}

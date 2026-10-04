import { useEffect, useState } from 'react';
import { Outlet } from '@tanstack/react-router';
import { useQuery } from '@tanstack/react-query';
import { Alert, AppShell, Group, Text, UnstyledButton } from '@mantine/core';
import { Window } from '@wailsio/runtime';
import { appInfo } from '../features/application/queries';
import { ErrorNotice } from '../shared/ErrorNotice';
import { AccountBadge, LoginModal } from '../usecases/azure-login/AzureLogin';

function AppIcon() {
  return (
    <svg width="22" height="22" viewBox="0 0 24 24" aria-hidden="true">
      <rect x="3" y="4" width="18" height="4" rx="1.5" fill="#4dabf7" />
      <rect x="3" y="10" width="18" height="4" rx="1.5" fill="#339af0" />
      <rect x="3" y="16" width="18" height="4" rx="1.5" fill="#1c7ed6" />
    </svg>
  );
}

function WindowControls() {
  const [maximised, setMaximised] = useState(false);
  useEffect(() => {
    const sync = () => void Window.IsMaximised().then(setMaximised);
    sync();
    window.addEventListener('resize', sync);
    return () => window.removeEventListener('resize', sync);
  }, []);
  return (
    <Group gap={0} wrap="nowrap" className="window-controls">
      <UnstyledButton aria-label="最小化" onClick={() => void Window.Minimise()}>
        <svg width="10" height="10" viewBox="0 0 10 10" aria-hidden="true">
          <path d="M0 5.5h10" stroke="currentColor" />
        </svg>
      </UnstyledButton>
      <UnstyledButton
        aria-label={maximised ? '元に戻す' : '最大化'}
        onClick={() => void Window.ToggleMaximise()}
      >
        <svg width="10" height="10" viewBox="0 0 10 10" aria-hidden="true" fill="none">
          {maximised ? (
            <path d="M2.5 2.5v-2h7v7h-2M.5 2.5h7v7h-7z" stroke="currentColor" />
          ) : (
            <path d="M.5.5h9v9h-9z" stroke="currentColor" />
          )}
        </svg>
      </UnstyledButton>
      <UnstyledButton
        aria-label="閉じる"
        className="window-close"
        onClick={() => void Window.Close()}
      >
        <svg width="10" height="10" viewBox="0 0 10 10" aria-hidden="true">
          <path d="M0 0l10 10M10 0L0 10" stroke="currentColor" />
        </svg>
      </UnstyledButton>
    </Group>
  );
}

export function Shell() {
  const info = useQuery(appInfo());
  return (
    <AppShell header={{ height: 48 }} padding="lg" className="app-shell">
      <AppShell.Header withBorder={false}>
        <Group h="100%" pl="md" justify="space-between" wrap="nowrap">
          <Group gap="xs" wrap="nowrap">
            <AppIcon />
            <Text fw={700} size="sm">
              Azure Foundry Deck
            </Text>
          </Group>
          <Group h="100%" gap="sm" wrap="nowrap" className="no-drag">
            <AccountBadge />
            {info.data?.server === false && <WindowControls />}
          </Group>
        </Group>
      </AppShell.Header>
      <AppShell.Main>
        <ErrorNotice error={info.error} />
        {info.data && !info.data.diagnosticsAvailable && (
          <Alert color="yellow" mb="lg">
            診断ログを保存できません。データ領域のアクセス権と空き容量を確認してください。
          </Alert>
        )}
        <Outlet />
      </AppShell.Main>
      <LoginModal />
    </AppShell>
  );
}

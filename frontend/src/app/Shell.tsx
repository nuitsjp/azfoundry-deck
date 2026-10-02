import { useEffect, useState } from 'react';
import { Link, Outlet } from '@tanstack/react-router';
import { useIsMutating, useQuery } from '@tanstack/react-query';
import { Alert, AppShell, Button, Group, Modal, NavLink, Stack, Text } from '@mantine/core';
import { appInfo, confirmQuit, ready, subscribeClose } from '../features/application/queries';
import { ErrorNotice } from '../shared/ErrorNotice';
import { ExitProvider, useExit } from '../shared/ExitContext';
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

function Content() {
  const info = useQuery(appInfo());
  const { dirty } = useExit();
  const busy = useIsMutating() > 0;
  const [closing, setClosing] = useState(false);
  const [error, setError] = useState<unknown>(null);
  useEffect(() => {
    const off = subscribeClose(() => setClosing(true));
    void ready().catch(setError);
    return off;
  }, []);
  async function close() {
    try {
      await confirmQuit();
    } catch (failure) {
      setError(failure);
    }
  }
  return (
    <AppShell header={{ height: 52 }} navbar={{ width: 200, breakpoint: 0 }} padding="lg">
      <AppShell.Header>
        <Group h="100%" px="md" justify="space-between" wrap="nowrap">
          <Group gap="xs" wrap="nowrap">
            <AppIcon />
            <Text fw={700}>AzFoundryDeck</Text>
          </Group>
          <AccountBadge />
        </Group>
      </AppShell.Header>
      <AppShell.Navbar p="xs" aria-label="メインナビゲーション">
        <NavLink component={Link} to="/" label="Home" active />
      </AppShell.Navbar>
      <AppShell.Main>
        <ErrorNotice error={info.error || error} />
        {info.data && !info.data.diagnosticsAvailable && (
          <Alert color="yellow" mb="lg">
            診断ログを保存できません。データ領域のアクセス権と空き容量を確認してください。
          </Alert>
        )}
        <Outlet />
      </AppShell.Main>
      <LoginModal />
      <Modal
        opened={closing}
        onClose={() => setClosing(false)}
        title="アプリを終了しますか？"
        centered
      >
        <Stack>
          <Text>
            {busy
              ? '処理中です。完了または中止を待ってから終了してください。'
              : dirty
                ? '未保存の入力内容は破棄されます。'
                : 'アプリを終了します。'}
          </Text>
          <ErrorNotice error={error} />
          <Group justify="flex-end">
            <Button variant="default" onClick={() => setClosing(false)}>
              戻る
            </Button>
            <Button disabled={busy} onClick={() => void close()}>
              終了する
            </Button>
          </Group>
        </Stack>
      </Modal>
    </AppShell>
  );
}
export function Shell() {
  return (
    <ExitProvider>
      <Content />
    </ExitProvider>
  );
}

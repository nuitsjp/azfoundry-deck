import { useQuery } from '@tanstack/react-query';
import { Avatar, Button, Group, Text, Tooltip } from '@mantine/core';
import { Phase } from '@bindings/azfoundrydeck/internal/azauth/models';
import { authStatus, useLogin } from '../../features/auth/queries';
import { publicError } from '../../shared/errors';

// Header control: sign-in button, or tenant name and account avatar.
export function AzureLogin() {
  const status = useQuery(authStatus());
  const login = useLogin();
  if (status.isPending) return null;
  if (status.error) {
    const info = publicError(status.error);
    return (
      <Text size="xs" c="red.4" role="alert">
        {info.code}: {info.message}
      </Text>
    );
  }
  const account = status.data.account;
  if (status.data.phase === Phase.SignedIn && account) {
    return (
      <Group gap="sm" wrap="nowrap">
        <Text size="sm" c="dimmed">
          {account.tenantName}
        </Text>
        <Tooltip label={account.username} position="bottom-end">
          <Avatar name={account.username} color="initials" radius="xl" size="sm" />
        </Tooltip>
      </Group>
    );
  }
  const waiting = login.isPending || status.data.phase === Phase.SigningIn;
  const failure = !waiting && login.error ? publicError(login.error) : null;
  return (
    <Group gap="sm" wrap="nowrap">
      {failure && (
        <Tooltip label={failure.message} position="bottom-end" multiline w={320}>
          <Text size="xs" c="red.4" role="alert" maw={320} truncate>
            {failure.code}: {failure.message}
          </Text>
        </Tooltip>
      )}
      {waiting && (
        <Text size="xs" c="dimmed" role="status">
          ブラウザーでサインインしてください
        </Text>
      )}
      <Button size="xs" disabled={waiting} onClick={() => login.mutate()}>
        {waiting ? 'サインイン中…' : 'Azureにログイン'}
      </Button>
    </Group>
  );
}

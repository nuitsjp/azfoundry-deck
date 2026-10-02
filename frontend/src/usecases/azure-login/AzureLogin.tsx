import { useQuery } from '@tanstack/react-query';
import { Button, Group, Loader, Modal, Stack, Text, Title } from '@mantine/core';
import { Phase } from '@bindings/azfoundrydeck/internal/azauth/models';
import { authStatus, useLogin } from '../../features/auth/queries';
import { publicError } from '../../shared/errors';
import { AccountMenu } from '../azure-logout/AccountMenu';

// Header: tenant name and account avatar after sign-in.
export function AccountBadge() {
  const status = useQuery(authStatus());
  const account = status.data?.phase === Phase.SignedIn ? status.data.account : null;
  if (!account) return null;
  return (
    <Group gap="sm" wrap="nowrap">
      <Text size="sm" c="dimmed">
        {account.tenantName}
      </Text>
      <AccountMenu account={account} />
    </Group>
  );
}

// Blocks the app until sign-in completes. Not shown while the status is loading.
export function LoginModal() {
  const status = useQuery(authStatus());
  const login = useLogin();
  const error = status.error ?? login.error;
  const waiting = login.isPending || status.data?.phase === Phase.SigningIn;
  const failure = !waiting && error ? publicError(error) : null;
  const opened = status.isError || (!!status.data && status.data.phase !== Phase.SignedIn);
  return (
    <Modal
      opened={opened}
      onClose={() => {}}
      withCloseButton={false}
      closeOnEscape={false}
      closeOnClickOutside={false}
      centered
      size="sm"
    >
      <Stack align="center" gap="lg" py="md">
        <Title order={3}>AzFoundryDeck</Title>
        {waiting ? (
          <Group gap="sm" role="status">
            <Loader size="sm" />
            <Text size="sm" c="dimmed">
              ブラウザーでサインインしてください
            </Text>
          </Group>
        ) : (
          <Button onClick={() => login.mutate()}>Azureにログイン</Button>
        )}
        {failure && (
          <Text size="xs" c="red.4" role="alert" ta="center">
            {failure.code}: {failure.message}
          </Text>
        )}
      </Stack>
    </Modal>
  );
}

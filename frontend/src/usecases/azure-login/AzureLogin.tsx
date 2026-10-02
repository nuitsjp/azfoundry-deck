import { useQuery } from '@tanstack/react-query';
import {
  Button,
  Combobox,
  Group,
  InputBase,
  Loader,
  Modal,
  Stack,
  Text,
  Title,
  useCombobox,
} from '@mantine/core';
import { Phase } from '@bindings/azfoundrydeck/internal/azauth/models';
import { authStatus, useLogin } from '../../features/auth/queries';
import { publicError } from '../../shared/errors';
import { AccountMenu } from '../azure-logout/AccountMenu';

// Header: tenant name and account avatar after sign-in.
export function AccountBadge() {
  const status = useQuery(authStatus());
  const account = status.data?.phase === Phase.SignedIn ? status.data.account : null;
  const combobox = useCombobox({ onDropdownClose: () => combobox.resetSelectedOption() });
  if (!account?.tenants) return null;
  const selected = account.tenants.find((tenant) => tenant.id === account.selectedTenantId);
  return (
    <Group gap="sm" wrap="nowrap">
      {selected && (
        <Combobox store={combobox} onOptionSubmit={() => combobox.closeDropdown()}>
          <Combobox.Target targetType="button" withExpandedAttribute>
            <InputBase
              component="button"
              type="button"
              aria-label="テナント"
              rightSection={<Combobox.Chevron />}
              rightSectionPointerEvents="none"
              onClick={() => combobox.toggleDropdown()}
              w={220}
              styles={{ input: { textAlign: 'left' } }}
            >
              {selected.displayName}
            </InputBase>
          </Combobox.Target>
          <Combobox.Dropdown>
            <Combobox.Options>
              {account.tenants.map((tenant) => (
                <Combobox.Option
                  key={tenant.id}
                  value={tenant.id}
                  active={tenant.id === account.selectedTenantId}
                  aria-selected={tenant.id === account.selectedTenantId}
                >
                  {tenant.displayName}
                </Combobox.Option>
              ))}
            </Combobox.Options>
          </Combobox.Dropdown>
        </Combobox>
      )}
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

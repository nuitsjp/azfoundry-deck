import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import {
  Button,
  Combobox,
  Group,
  InputBase,
  Loader,
  Modal,
  Select,
  Stack,
  Text,
  Title,
  useCombobox,
} from '@mantine/core';
import { Phase, type Account } from '@bindings/azfoundrydeck/internal/azauth/models';
import {
  authStatus,
  useChangeTenant,
  useLogin,
  useSelectTenant,
} from '../../features/auth/queries';
import { publicError } from '../../shared/errors';
import { AccountMenu } from '../azure-logout/AccountMenu';

// Header: tenant name and account avatar after sign-in.
export function AccountBadge() {
  const status = useQuery(authStatus());
  const account = status.data?.phase === Phase.SignedIn ? status.data.account : null;
  const combobox = useCombobox({ onDropdownClose: () => combobox.resetSelectedOption() });
  const change = useChangeTenant();
  if (!account?.selectedTenantId || !account.tenants) return null;
  const selected = account.tenants.find((tenant) => tenant.id === account.selectedTenantId);
  return (
    <Group gap="sm" wrap="nowrap">
      {selected && (
        <Combobox
          store={combobox}
          onOptionSubmit={(tenantId) => {
            combobox.closeDropdown();
            if (tenantId !== account.selectedTenantId) change.mutate(tenantId);
          }}
        >
          <Combobox.Target targetType="button" withExpandedAttribute>
            <InputBase
              component="button"
              type="button"
              aria-label="テナント"
              rightSection={<Combobox.Chevron />}
              rightSectionPointerEvents="none"
              onClick={() => combobox.toggleDropdown()}
              disabled={change.isPending}
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

function TenantSelection({ account }: { account: Account }) {
  const [tenantId, setTenantId] = useState<string | null>(null);
  const selection = useSelectTenant();
  const failure = selection.error ? publicError(selection.error) : null;
  return (
    <Stack gap="lg" py="md">
      <Title order={3}>テナントを選択</Title>
      <Text size="sm" c="dimmed">
        Azureにサインインしました。利用するテナントを選択してください。
      </Text>
      <Select
        label="テナント"
        placeholder="テナントを選んでください"
        data={account.tenants?.map((tenant) => ({ value: tenant.id, label: tenant.displayName }))}
        value={tenantId}
        onChange={setTenantId}
        allowDeselect={false}
        disabled={selection.isPending}
      />
      <Button
        disabled={!tenantId}
        loading={selection.isPending}
        onClick={() => {
          if (tenantId) selection.mutate(tenantId);
        }}
      >
        確定
      </Button>
      {failure && (
        <Text size="xs" c="red.4" role="alert">
          {failure.code}: {failure.message}
        </Text>
      )}
    </Stack>
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
      {status.data?.phase === Phase.SelectingTenant && status.data.account ? (
        <TenantSelection account={status.data.account} />
      ) : (
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
      )}
    </Modal>
  );
}

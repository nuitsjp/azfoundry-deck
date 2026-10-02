import { useState } from 'react';
import { Avatar, Menu, Text, Tooltip, UnstyledButton } from '@mantine/core';
import type { Account } from '@bindings/azfoundrydeck/internal/azauth/models';
import { useLogout } from '../../features/auth/queries';
import { publicError } from '../../shared/errors';

// The header user icon. Its menu holds the account name and logout only; it
// stays open on failure so the error shows next to a retryable logout.
export function AccountMenu({ account }: { account: Account }) {
  const logout = useLogout();
  const [opened, setOpened] = useState(false);
  const failure = logout.error ? publicError(logout.error) : null;
  return (
    <Menu
      opened={opened}
      onChange={setOpened}
      position="bottom-end"
      closeOnItemClick={false}
      onClose={() => logout.reset()}
    >
      <Tooltip label={account.username} position="bottom-end" disabled={opened}>
        <Menu.Target>
          <UnstyledButton aria-label="アカウント">
            <Avatar name={account.username} color="initials" radius="xl" size="sm" />
          </UnstyledButton>
        </Menu.Target>
      </Tooltip>
      <Menu.Dropdown>
        <Menu.Label>{account.username}</Menu.Label>
        <Menu.Item disabled={logout.isPending} onClick={() => logout.mutate()}>
          ログアウト
        </Menu.Item>
        {failure && (
          <Text size="xs" c="red.4" role="alert" px="sm" py={4} maw={280}>
            {failure.code}: {failure.message}
          </Text>
        )}
      </Menu.Dropdown>
    </Menu>
  );
}

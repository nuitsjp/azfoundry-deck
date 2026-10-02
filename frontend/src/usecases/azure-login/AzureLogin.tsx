import { useQuery } from '@tanstack/react-query';
import { Button, Card, Loader, Stack, Table, Text, Title } from '@mantine/core';
import { Phase } from '@bindings/azfoundrydeck/internal/azauth/models';
import { authStatus, useLogin } from '../../features/auth/queries';
import { ErrorNotice } from '../../shared/ErrorNotice';

export function AzureLogin() {
  const status = useQuery(authStatus());
  const login = useLogin();
  if (status.isPending) return <Loader aria-label="読み込み中" />;
  if (status.error) return <ErrorNotice error={status.error} />;
  const account = status.data.account;
  if (status.data.phase === Phase.SignedIn && account) {
    return (
      <Card withBorder padding="xl" maw={560}>
        <Stack>
          <Title order={2}>Azureにログイン済み</Title>
          <Table variant="vertical" withTableBorder>
            <Table.Tbody>
              <Table.Tr>
                <Table.Th w={140}>アカウント名</Table.Th>
                <Table.Td>{account.username}</Table.Td>
              </Table.Tr>
              <Table.Tr>
                <Table.Th>テナントID</Table.Th>
                <Table.Td>{account.tenantID}</Table.Td>
              </Table.Tr>
            </Table.Tbody>
          </Table>
        </Stack>
      </Card>
    );
  }
  const waiting = login.isPending || status.data.phase === Phase.SigningIn;
  return (
    <Card withBorder padding="xl" maw={560}>
      <Stack align="flex-start">
        <ErrorNotice error={waiting ? null : login.error} />
        {waiting && <Text role="status">ブラウザーでサインインしてください</Text>}
        <Button disabled={waiting} onClick={() => login.mutate()}>
          Azureにログイン
        </Button>
      </Stack>
    </Card>
  );
}

import { Badge, Group, Loader, Modal, ScrollArea, Stack, Table, Text } from '@mantine/core';
import type { FoundryProgress, AcquisitionPhase } from '../../features/foundry/progress';

const colors: Record<AcquisitionPhase, string> = {
  waiting: 'gray',
  running: 'blue',
  completed: 'teal',
};

function Status({ phase, runningLabel }: { phase: AcquisitionPhase; runningLabel: string }) {
  return (
    <Badge color={colors[phase]} variant="light">
      {phase === 'waiting' ? '待機中' : phase === 'running' ? runningLabel : '完了'}
    </Badge>
  );
}

export function AcquisitionProgressModal({
  opened,
  progress,
  modelsOnly = false,
}: {
  opened: boolean;
  progress: FoundryProgress;
  modelsOnly?: boolean;
}) {
  const completed = progress.subscriptions.filter(
    (subscription) => subscription.phase === 'completed',
  ).length;
  const waiting = progress.subscriptions.filter(
    (subscription) => subscription.phase === 'waiting',
  ).length;
  const running = progress.subscriptions.filter(
    (subscription) => subscription.phase === 'running',
  ).length;
  const searching = progress.subscriptionSearch === 'searching';
  const pending = progress.subscriptions.filter(
    (subscription) => subscription.phase !== 'completed',
  );

  return (
    <Modal.Root
      opened={opened}
      onClose={() => {}}
      size="lg"
      centered
      closeOnEscape={false}
      closeOnClickOutside={false}
    >
      <Modal.Overlay />
      <Modal.Content>
        <Modal.Header role="presentation">
          <Modal.Title>デプロイモデルを取得しています</Modal.Title>
        </Modal.Header>
        <Modal.Body>
          <Stack gap="md">
            {!modelsOnly && (
              <>
                <Group gap="sm">
                  {searching && <Loader size="sm" />}
                  <Text fw={600}>
                    {searching ? 'サブスクリプションを検索中' : 'サブスクリプションの検索完了'}
                  </Text>
                  <Text size="sm" c="dimmed">
                    発見 {progress.subscriptions.length} 件
                  </Text>
                </Group>
                <Stack gap={4}>
                  <Text size="sm" role="status">
                    完了 {completed} / 発見 {progress.subscriptions.length} 件
                    {searching ? '（検索中のため総数は未確定）' : ''}
                  </Text>
                  <Text size="sm" c="dimmed">
                    待機中 {waiting} 件・Foundry取得中 {running} 件
                  </Text>
                </Stack>
                <ScrollArea.Autosize mah={300} type="auto">
                  <Table aria-label="サブスクリプションの取得状況" verticalSpacing="xs">
                    <Table.Thead>
                      <Table.Tr>
                        <Table.Th>サブスクリプション</Table.Th>
                        <Table.Th>状態</Table.Th>
                        <Table.Th ta="right">Foundry</Table.Th>
                      </Table.Tr>
                    </Table.Thead>
                    <Table.Tbody>
                      {pending.map((subscription) => (
                        <Table.Tr key={subscription.id}>
                          <Table.Td style={{ overflowWrap: 'anywhere' }}>
                            {subscription.name}
                          </Table.Td>
                          <Table.Td>
                            <Status phase={subscription.phase} runningLabel="Foundry取得中" />
                          </Table.Td>
                          <Table.Td ta="right">{subscription.foundryCount} 件</Table.Td>
                        </Table.Tr>
                      ))}
                    </Table.Tbody>
                  </Table>
                  {pending.length === 0 && (
                    <Text size="sm" c="dimmed" py="md">
                      {searching
                        ? 'サブスクリプションの発見を待っています。'
                        : 'すべてのサブスクリプションの取得が完了しました。'}
                    </Text>
                  )}
                </ScrollArea.Autosize>
              </>
            )}
            <Stack gap="xs" pt="xs" style={{ borderTop: '1px solid var(--mantine-color-dark-4)' }}>
              <Group justify="space-between">
                <Text fw={600} size="sm">
                  デプロイモデルの取得
                </Text>
                <Status phase={progress.modelPhase} runningLabel="取得中" />
              </Group>
              <Text size="sm" style={{ overflowWrap: 'anywhere' }}>
                {progress.selectedFoundryName || '選択先のFoundryを探しています。'}
              </Text>
              <Text size="sm" c="dimmed">
                取得したモデル {progress.modelCount} 件
              </Text>
              <Group justify="space-between" mt="xs">
                <Text fw={600} size="sm">
                  ファイルへの保存
                </Text>
                <Status phase={progress.savePhase} runningLabel="保存中" />
              </Group>
            </Stack>
          </Stack>
        </Modal.Body>
      </Modal.Content>
    </Modal.Root>
  );
}

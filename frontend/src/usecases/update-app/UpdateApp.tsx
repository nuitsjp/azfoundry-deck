import { Alert, Button, Group, Text } from '@mantine/core';
import { useUpdates } from '../../features/updates/queries';
import { ErrorNotice } from '../../shared/ErrorNotice';
import { publicError } from '../../shared/errors';

// Shown only while a verified installer of a newer version is staged.
export function UpdateApp() {
  const update = useUpdates();
  const status = update.status.data;
  if (!status?.available || !['ready', 'untrusted', 'handed-off'].includes(status.phase)) {
    return null;
  }
  const untrusted = status.phase === 'untrusted';
  return (
    <Alert color={untrusted ? 'red' : 'blue'} mb="lg" aria-label="更新">
      <Group justify="space-between" wrap="nowrap">
        {untrusted ? (
          <Text size="sm" role="alert">
            更新を検証できませんでした。次回の起動時に確認し直します。
          </Text>
        ) : (
          <Text size="sm">バージョン {status.version} の準備ができました。</Text>
        )}
        {!untrusted && (
          <Button
            size="xs"
            loading={update.apply.isPending || status.phase === 'handed-off'}
            onClick={() => update.apply.mutate()}
          >
            更新して再起動
          </Button>
        )}
      </Group>
      {update.apply.error && publicError(update.apply.error).code !== 'UPDATE_UNTRUSTED' && (
        <ErrorNotice error={update.apply.error} />
      )}
    </Alert>
  );
}

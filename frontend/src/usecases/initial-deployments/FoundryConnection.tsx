import { ActionIcon, Group, Loader, Text, Tooltip } from '@mantine/core';
import { useQuery } from '@tanstack/react-query';
import { Events } from '@wailsio/runtime';
import { useEffect, useState } from 'react';
import { getConnectionState } from '../../features/foundry/connection';

function CopyIcon() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" aria-hidden="true">
      <path
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M9 9h11v11H9zM5 15H4V4h11v1"
      />
    </svg>
  );
}

// The key is never rendered in full; only its last 4 characters are shown.
function maskKey(key: string) {
  return `${'•'.repeat(12)}${key.slice(-4)}`;
}

function ConnectionItem({
  name,
  shown,
  value,
  loading,
  title,
}: {
  name: string;
  shown: string;
  value: string | undefined;
  loading: boolean;
  title?: string;
}) {
  const [copied, setCopied] = useState(false);
  useEffect(() => {
    if (!copied) return;
    const timer = setTimeout(() => setCopied(false), 2000);
    return () => clearTimeout(timer);
  }, [copied]);
  return (
    <Group gap="sm" align="center" wrap="nowrap" style={{ minHeight: 28 }}>
      <Text size="sm" fw={500} w={140} style={{ flexShrink: 0 }}>
        {name}
      </Text>
      {loading ? (
        <Group gap={6} role="status" align="center" wrap="nowrap" aria-label={`${name} loading`}>
          <Loader size="xs" />
          <Text size="sm">Loading...</Text>
        </Group>
      ) : (
        <Tooltip label={title} disabled={!title} multiline maw={600}>
          <Text
            size="sm"
            ff="monospace"
            aria-label={name}
            style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}
          >
            {shown}
          </Text>
        </Tooltip>
      )}
      <Tooltip label={copied ? 'Copied' : `Copy ${name}`}>
        <ActionIcon
          variant="subtle"
          aria-label={copied ? 'Copied' : `Copy ${name}`}
          disabled={loading || !value}
          onClick={() => {
            if (!value) return;
            void navigator.clipboard.writeText(value).then(() => setCopied(true));
          }}
          style={{ flexShrink: 0 }}
        >
          {copied ? <Text size="xs">Copied</Text> : <CopyIcon />}
        </ActionIcon>
      </Tooltip>
    </Group>
  );
}

export function FoundryConnection({
  foundryID,
  fetchedAt,
}: {
  foundryID: string;
  fetchedAt: string;
}) {
  const [revision, setRevision] = useState(0);
  const state = useQuery({
    queryKey: ['foundry', 'connection', foundryID, fetchedAt, revision],
    queryFn: () => getConnectionState(),
    enabled: !!foundryID,
    staleTime: Infinity,
    gcTime: 0,
    retry: false,
  });
  useEffect(
    () =>
      Events.On('foundry:connection-ready', (event) => {
        if (event.data === foundryID) {
          setRevision((value) => value + 1);
        }
      }),
    [foundryID],
  );
  if (!foundryID) return null;
  const loading = state.isPending || state.data?.loading === true;
  const connection = state.data?.connection ?? null;
  return (
    <section aria-label="Connection">
      <ConnectionItem
        name="Endpoint"
        shown={connection?.endpoint ?? ''}
        value={connection?.endpoint}
        loading={loading}
        title={connection?.endpoint}
      />
      <ConnectionItem
        name="API key"
        shown={connection ? maskKey(connection.key) : ''}
        value={connection?.key}
        loading={loading}
      />
    </section>
  );
}

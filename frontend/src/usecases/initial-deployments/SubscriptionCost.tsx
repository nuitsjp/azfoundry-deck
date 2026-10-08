import { ActionIcon, Group, Loader, Text, Tooltip } from '@mantine/core';
import { useMutation, useQuery } from '@tanstack/react-query';
import { Events } from '@wailsio/runtime';
import { useEffect, useState } from 'react';
import { getCostState, refreshCost } from '../../features/foundry/cost';
import { publicError } from '../../shared/errors';
import { RefreshIcon } from './RefreshIcon';

// Yen is rounded to whole yen; other billing currencies keep 2 decimals and their code.
// Without usage Azure returns no currency, so the cost is a bare 0.
function formatCost(amount: number, currency: string) {
  if (!currency) return '0';
  if (currency === 'JPY') return `¥${Math.round(amount).toLocaleString('en-US')}`;
  const shown = amount.toLocaleString('en-US', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  });
  return `${shown} ${currency}`;
}

export function SubscriptionCost({
  foundryID,
  fetchedAt,
}: {
  foundryID: string;
  fetchedAt: string;
}) {
  const [revision, setRevision] = useState(0);
  const state = useQuery({
    queryKey: ['foundry', 'cost', foundryID, fetchedAt, revision],
    queryFn: () => getCostState(),
    enabled: !!foundryID,
    staleTime: Infinity,
    gcTime: 0,
    retry: false,
  });
  useEffect(
    () =>
      Events.On('foundry:cost-ready', (event) => {
        if (event.data === foundryID) {
          setRevision((value) => value + 1);
        }
      }),
    [foundryID],
  );
  const refresh = useMutation({
    mutationFn: refreshCost,
    onSettled: () => setRevision((value) => value + 1),
  });
  const loading = state.isPending || refresh.isPending || state.data?.loading === true;
  const cost = state.data?.cost ?? null;
  const error = refresh.error ? publicError(refresh.error) : (state.data?.error ?? null);
  return (
    <Group gap="sm" align="center" wrap="nowrap" style={{ minWidth: 0 }}>
      <Text size="sm" fw={500} style={{ flexShrink: 0, whiteSpace: 'nowrap' }}>
        This month
      </Text>
      {loading ? (
        <Group gap={6} role="status" align="center" wrap="nowrap" aria-label="This month loading">
          <Loader size="xs" />
          <Text size="sm">Loading...</Text>
        </Group>
      ) : error ? (
        <Text size="xs" c="red" style={{ whiteSpace: 'nowrap' }}>
          {`${error.code}: ${error.message}`}
        </Text>
      ) : (
        cost && (
          <Text size="sm" ff="monospace" aria-label="This month" style={{ whiteSpace: 'nowrap' }}>
            {formatCost(cost.amount, cost.currency)}
          </Text>
        )
      )}
      <Tooltip label="Refresh cost">
        <ActionIcon
          variant="subtle"
          aria-label="Refresh cost"
          disabled={loading}
          onClick={() => refresh.mutate()}
          style={{ flexShrink: 0 }}
        >
          <RefreshIcon />
        </ActionIcon>
      </Tooltip>
    </Group>
  );
}

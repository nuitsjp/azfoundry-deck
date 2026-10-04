import {
  ActionIcon,
  Badge,
  Button,
  Group,
  Loader,
  Stack,
  Table,
  Title,
  Tooltip,
} from '@mantine/core';
import { useMutation } from '@tanstack/react-query';
import { useEffect, useRef, useState } from 'react';
import { getCapacityMaximum } from '../../features/foundry/capacity-maximum';
import type { Deployment } from '../../features/foundry/models';
import { ErrorNotice } from '../../shared/ErrorNotice';

function EditIcon() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" aria-hidden="true">
      <path
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"
      />
    </svg>
  );
}

function TrashIcon() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" aria-hidden="true">
      <path
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M3 6h18M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2M10 11v6M14 11v6"
      />
    </svg>
  );
}

const upgradePolicyLabels = new Map([
  ['OnceNewDefaultVersionAvailable', 'Upgrade to new default'],
  ['OnceCurrentVersionExpired', 'Upgrade on retirement'],
  ['NoAutoUpgrade', 'No automatic upgrade'],
]);

function capacityValue(value: number | null | undefined) {
  return value?.toLocaleString('en-US') ?? 'Not set';
}

export function DeploymentDetails({
  deployments,
  busy,
  onDelete,
  onEdit,
}: {
  deployments: Deployment[];
  busy: boolean;
  onDelete: (deployment: Deployment) => void;
  onEdit: (deployment: Deployment) => void;
}) {
  const [selectedID, setSelectedID] = useState<string | null>(null);
  const [maximum, setMaximum] = useState<number | null | undefined>(undefined);
  const [error, setError] = useState<unknown>(null);
  const request = useRef(0);
  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
      request.current += 1;
    };
  }, []);
  // The details come from the listed deployment; only the capacity maximum is fetched.
  const fetchMaximum = useMutation({
    // The key also keeps the tenant selector disabled while the maximum is fetched.
    mutationKey: ['foundry', 'deployment-detail'],
    gcTime: 0,
    retry: false,
    mutationFn: async ({ id, sequence }: { id: string; sequence: number }) => {
      try {
        const result = await getCapacityMaximum(id);
        if (mounted.current && request.current === sequence) setMaximum(result);
      } catch (failure) {
        if (mounted.current && request.current === sequence) setError(failure);
      }
    },
  });
  const detail = deployments.find((deployment) => deployment.id === selectedID) ?? null;

  function selectDeployment(id: string) {
    if (busy || fetchMaximum.isPending) return;
    setSelectedID(id);
    setMaximum(undefined);
    setError(null);
    fetchMaximum.mutate({ id, sequence: ++request.current });
  }

  return (
    <div className="deployment-split">
      <section className="deployment-list-pane" aria-label="Deployments">
        <header className="deployment-pane-heading">
          <Title order={5}>Deployments</Title>
        </header>
        <div className="deployment-table-wrap">
          <Table aria-label="Deployments" striped highlightOnHover>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>Deployment</Table.Th>
                <Table.Th>Model</Table.Th>
                <Table.Th>Version</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {deployments.map((deployment) => (
                <Table.Tr
                  key={deployment.id}
                  className={`deployment-row${selectedID === deployment.id ? ' deployment-selected' : ''}`}
                  aria-disabled={busy || fetchMaximum.isPending}
                  onClick={() => selectDeployment(deployment.id)}
                >
                  <Table.Td>
                    <button
                      type="button"
                      className="deployment-select"
                      aria-pressed={selectedID === deployment.id}
                      disabled={busy || fetchMaximum.isPending}
                      onClick={(event) => {
                        event.stopPropagation();
                        selectDeployment(deployment.id);
                      }}
                    >
                      {deployment.deploymentName}
                    </button>
                  </Table.Td>
                  <Table.Td>{deployment.modelName}</Table.Td>
                  <Table.Td>{deployment.version}</Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        </div>
      </section>
      <section
        className="deployment-detail-pane"
        aria-label="Details"
        aria-busy={fetchMaximum.isPending}
      >
        <header className="deployment-pane-heading">
          <Title order={5}>Details</Title>
        </header>
        {detail ? (
          <>
            <Group
              className="deployment-detail-title"
              justify="space-between"
              align="center"
              wrap="nowrap"
            >
              <Group gap="xs" align="center" wrap="nowrap" style={{ minWidth: 0 }}>
                <Title order={3}>{detail.deploymentName}</Title>
                {detail.provisioningState && (
                  <Badge
                    color={detail.provisioningState === 'Succeeded' ? 'green' : 'gray'}
                    variant="light"
                  >
                    {detail.provisioningState}
                  </Badge>
                )}
              </Group>
              <Tooltip label="Edit deployment">
                <ActionIcon
                  variant="default"
                  size={32}
                  aria-label="Edit deployment"
                  disabled={busy || fetchMaximum.isPending}
                  onClick={() => onEdit(detail)}
                >
                  <EditIcon />
                </ActionIcon>
              </Tooltip>
            </Group>
            <dl className="deployment-detail-fields">
              <dt>Model</dt>
              <dd>{detail.modelName || 'Not set'}</dd>
              <dt>Version</dt>
              <dd>{detail.version || 'Not set'}</dd>
              <dt>SKU</dt>
              <dd>{detail.skuName ?? 'Not set'}</dd>
              <dt>Capacity</dt>
              <dd>
                {capacityValue(detail.capacity)} /{' '}
                {fetchMaximum.isPending ? (
                  <Group component="span" gap={6} role="status" display="inline-flex">
                    <Loader size="xs" />
                    <span>Loading...</span>
                  </Group>
                ) : (
                  capacityValue(maximum)
                )}
                {detail.capacityUnit ? ` ${detail.capacityUnit}` : ''}
              </dd>
              {error ? (
                <>
                  <dt aria-hidden="true" />
                  <dd>
                    <Stack gap={6} align="flex-start">
                      <ErrorNotice error={error} />
                      <Button
                        variant="default"
                        size="xs"
                        disabled={busy}
                        onClick={() => selectDeployment(detail.id)}
                      >
                        Retry
                      </Button>
                    </Stack>
                  </dd>
                </>
              ) : null}
              <dt>Provisioning state</dt>
              <dd>{detail.provisioningState ?? 'Not set'}</dd>
              <dt>Upgrade policy</dt>
              <dd>
                {detail.versionUpgradePolicy == null
                  ? 'Not set'
                  : (upgradePolicyLabels.get(detail.versionUpgradePolicy) ??
                    detail.versionUpgradePolicy)}
              </dd>
            </dl>
            <div className="deployment-detail-actions">
              <Tooltip label="Delete deployment">
                <ActionIcon
                  color="red"
                  variant="subtle"
                  size={32}
                  aria-label="Delete deployment"
                  disabled={busy || fetchMaximum.isPending}
                  onClick={() => onDelete(detail)}
                >
                  <TrashIcon />
                </ActionIcon>
              </Tooltip>
            </div>
          </>
        ) : null}
      </section>
    </div>
  );
}

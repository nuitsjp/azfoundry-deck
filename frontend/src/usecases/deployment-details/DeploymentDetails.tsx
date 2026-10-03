import { Badge, Button, Group, Loader, Stack, Table, Text, Title } from '@mantine/core';
import { useMutation } from '@tanstack/react-query';
import { useEffect, useRef, useState } from 'react';
import { getDeploymentDetail } from '../../features/foundry/deployment-detail';
import type { Deployment, DeploymentDetail } from '../../features/foundry/models';
import { ErrorNotice } from '../../shared/ErrorNotice';

function fetchedAt(value: string) {
  const time = new Date(value);
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${time.getFullYear()}-${pad(time.getMonth() + 1)}-${pad(time.getDate())} ${pad(time.getHours())}:${pad(time.getMinutes())}`;
}

export function DeploymentDetails({
  deployments,
  busy,
}: {
  deployments: Deployment[];
  busy: boolean;
}) {
  const [selectedID, setSelectedID] = useState<string | null>(null);
  const [detail, setDetail] = useState<DeploymentDetail | null>(null);
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
  const fetchDetail = useMutation({
    mutationKey: ['foundry', 'deployment-detail'],
    gcTime: 0,
    retry: false,
    mutationFn: async ({ id, sequence }: { id: string; sequence: number }) => {
      try {
        const result = await getDeploymentDetail(id);
        if (mounted.current && request.current === sequence) setDetail(result);
      } catch (failure) {
        if (mounted.current && request.current === sequence) setError(failure);
      }
    },
  });

  function selectDeployment(id: string) {
    setSelectedID(id);
    setDetail(null);
    setError(null);
    fetchDetail.mutate({ id, sequence: ++request.current });
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
                  className={selectedID === deployment.id ? 'deployment-selected' : undefined}
                >
                  <Table.Td>
                    <button
                      type="button"
                      className="deployment-select"
                      aria-pressed={selectedID === deployment.id}
                      disabled={busy}
                      onClick={() => selectDeployment(deployment.id)}
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
        aria-busy={fetchDetail.isPending}
      >
        <header className="deployment-pane-heading">
          <Title order={5}>Details</Title>
        </header>
        {fetchDetail.isPending ? (
          <Group className="deployment-detail-content" role="status">
            <Loader size="sm" />
            <Text size="sm">Loading...</Text>
          </Group>
        ) : error ? (
          <Stack className="deployment-detail-content" align="flex-start">
            <ErrorNotice error={error} />
            <Button
              variant="default"
              size="xs"
              disabled={busy}
              onClick={() => selectDeployment(selectedID!)}
            >
              Retry
            </Button>
          </Stack>
        ) : detail ? (
          <>
            <Group className="deployment-detail-title" justify="space-between" wrap="nowrap">
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
            <dl className="deployment-detail-fields">
              <dt>Model</dt>
              <dd>{detail.modelName || 'Not set'}</dd>
              <dt>Version</dt>
              <dd>{detail.version || 'Not set'}</dd>
              <dt>SKU</dt>
              <dd>{detail.skuName ?? 'Not set'}</dd>
              <dt>Capacity</dt>
              <dd>{detail.capacity ?? 'Not set'}</dd>
              <dt>Provisioning state</dt>
              <dd>{detail.provisioningState ?? 'Not set'}</dd>
              <dt>Version upgrade policy</dt>
              <dd>{detail.versionUpgradePolicy ?? 'Not set'}</dd>
            </dl>
            <footer className="deployment-detail-footer">
              Last fetched {fetchedAt(detail.fetchedAt)}
            </footer>
          </>
        ) : null}
      </section>
    </div>
  );
}

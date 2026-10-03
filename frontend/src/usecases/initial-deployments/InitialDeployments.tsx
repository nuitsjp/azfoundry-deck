import { useIsMutating, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import {
  ActionIcon,
  Combobox,
  Group,
  Input,
  InputBase,
  Stack,
  Table,
  Text,
  Title,
  Tooltip,
  useCombobox,
} from '@mantine/core';
import { changeTenantKey } from '../../features/auth/queries';
import { changeFoundry } from '../../features/foundry/change-view';
import { loadInitialView } from '../../features/foundry/initial-view';
import { refreshFoundries } from '../../features/foundry/refresh-view';
import { refreshDeployments } from '../../features/foundry/refresh-deployments';
import type { Foundry } from '../../features/foundry/models';
import { ErrorNotice } from '../../shared/ErrorNotice';
import type { FoundryProgress } from '../../features/foundry/progress';
import { AcquisitionProgressModal } from './AcquisitionProgressModal';

function foundryLabel(foundry: Foundry) {
  return `${foundry.name}（${foundry.subscriptionName} - ${foundry.resourceGroupName}）`;
}

// Local time as YYYY-MM-DD HH:mm.
function fetchedAt(value: string) {
  const time = new Date(value);
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${time.getFullYear()}-${pad(time.getMonth() + 1)}-${pad(time.getDate())} ${pad(time.getHours())}:${pad(time.getMinutes())}`;
}

function RefreshIcon() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" aria-hidden="true">
      <path
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M20 11a8.1 8.1 0 0 0-15.5-2m-.5-5v5h5M4 13a8.1 8.1 0 0 0 15.5 2m.5 5v-5h-5"
      />
    </svg>
  );
}

export function InitialDeployments() {
  const [progress, setProgress] = useState<FoundryProgress | null>(null);
  const [changeProgress, setChangeProgress] = useState<FoundryProgress | null>(null);
  const [refreshProgress, setRefreshProgress] = useState<FoundryProgress | null>(null);
  const [modelsProgress, setModelsProgress] = useState<FoundryProgress | null>(null);
  const client = useQueryClient();
  const initial = useQuery({
    queryKey: ['foundry', 'initial-view'],
    queryFn: () => loadInitialView(setProgress),
    staleTime: Infinity,
    gcTime: Infinity,
    retry: false,
  });
  const change = useMutation({
    mutationFn: (id: string) => changeFoundry(id, setChangeProgress),
    onSuccess: (view) => client.setQueryData(['foundry', 'initial-view'], view),
  });
  const refresh = useMutation({
    mutationFn: () => refreshFoundries(setRefreshProgress),
    onSuccess: (view) => client.setQueryData(['foundry', 'initial-view'], view),
  });
  const refreshModels = useMutation({
    mutationFn: () => refreshDeployments(setModelsProgress),
    onSuccess: (view) => client.setQueryData(['foundry', 'initial-view'], view),
  });
  const changingTenant = useIsMutating({ mutationKey: changeTenantKey }) > 0;
  const busy = change.isPending || refresh.isPending || refreshModels.isPending || changingTenant;
  const combobox = useCombobox({ onDropdownClose: () => combobox.resetSelectedOption() });
  const view = initial.data;
  if (!view)
    return (
      <>
        {progress && <AcquisitionProgressModal opened={initial.isPending} progress={progress} />}
        <ErrorNotice error={initial.error} />
      </>
    );
  // With no Foundry, nothing is selected and the dropdown and the models are empty.
  const selected = view.foundries.find((foundry) => foundry.id === view.selectedFoundryId);
  const label = selected ? foundryLabel(selected) : '';

  return (
    <Stack gap="lg">
      {changeProgress && (
        <AcquisitionProgressModal
          opened={change.isPending}
          progress={changeProgress}
          mode="change"
        />
      )}
      {refreshProgress && (
        <AcquisitionProgressModal
          opened={refresh.isPending}
          progress={refreshProgress}
          mode="refresh"
        />
      )}
      {modelsProgress && (
        <AcquisitionProgressModal
          opened={refreshModels.isPending}
          progress={modelsProgress}
          mode="deployments"
        />
      )}
      <ErrorNotice
        error={change.error || refresh.error || refreshModels.error}
        onClose={() => {
          change.reset();
          refresh.reset();
          refreshModels.reset();
        }}
      />
      <Stack gap={6}>
        <Group gap="xs" align="flex-end" wrap="nowrap">
          <Combobox
            store={combobox}
            onOptionSubmit={(id) => {
              combobox.closeDropdown();
              if (id !== view.selectedFoundryId) {
                setChangeProgress(null);
                change.mutate(id);
              }
            }}
            withinPortal
          >
            <Combobox.Target targetType="button" withExpandedAttribute>
              <InputBase
                component="button"
                type="button"
                disabled={busy}
                label="Foundry"
                aria-label="Foundry"
                rightSection={<Combobox.Chevron />}
                rightSectionPointerEvents="none"
                onClick={() => combobox.toggleDropdown()}
                w="100%"
                maw={520}
                styles={{ input: { textAlign: 'left' } }}
              >
                <Tooltip label={label} disabled={combobox.dropdownOpened} multiline maw={600}>
                  <Input.Placeholder
                    component="span"
                    c="inherit"
                    style={{
                      display: 'block',
                      overflow: 'hidden',
                      textOverflow: 'ellipsis',
                      whiteSpace: 'nowrap',
                    }}
                  >
                    {label}
                  </Input.Placeholder>
                </Tooltip>
              </InputBase>
            </Combobox.Target>
            <Combobox.Dropdown w="min(900px, calc(100vw - 48px))">
              <Combobox.Options>
                {view.foundries.map((foundry) => (
                  <Combobox.Option
                    key={foundry.id}
                    value={foundry.id}
                    active={foundry.id === view.selectedFoundryId}
                    aria-selected={foundry.id === view.selectedFoundryId}
                  >
                    <Text size="sm" style={{ overflowWrap: 'anywhere' }}>
                      {foundryLabel(foundry)}
                    </Text>
                  </Combobox.Option>
                ))}
              </Combobox.Options>
            </Combobox.Dropdown>
          </Combobox>
          <Tooltip label="Foundry一覧を更新">
            <ActionIcon
              variant="default"
              size={36}
              aria-label="Foundry一覧を更新"
              disabled={busy}
              onClick={() => {
                setRefreshProgress(null);
                refresh.mutate();
              }}
            >
              <RefreshIcon />
            </ActionIcon>
          </Tooltip>
        </Group>
        <Text size="xs" c="dimmed">
          Foundry一覧の最終取得 {fetchedAt(view.foundriesFetchedAt)}
        </Text>
      </Stack>
      <Stack gap="xs">
        <Group gap="sm" align="center">
          <Title order={4}>デプロイ済みモデル</Title>
          <Tooltip label="モデルを更新">
            <ActionIcon
              variant="default"
              aria-label="モデルを更新"
              disabled={busy || !selected}
              onClick={() => {
                setModelsProgress(null);
                refreshModels.mutate();
              }}
            >
              <RefreshIcon />
            </ActionIcon>
          </Tooltip>
          <Text size="xs" c="dimmed">
            {view.deployments.length} 件
            {view.deploymentsFetchedAt && `・最終取得 ${fetchedAt(view.deploymentsFetchedAt)}`}
          </Text>
        </Group>
        <Table.ScrollContainer minWidth={480}>
          <Table aria-label="デプロイ済みモデル" striped highlightOnHover>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>デプロイ名</Table.Th>
                <Table.Th>モデル名</Table.Th>
                <Table.Th>バージョン</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {view.deployments.map((deployment) => (
                <Table.Tr key={deployment.id}>
                  <Table.Td>{deployment.deploymentName}</Table.Td>
                  <Table.Td>{deployment.modelName}</Table.Td>
                  <Table.Td>{deployment.version}</Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        </Table.ScrollContainer>
      </Stack>
    </Stack>
  );
}

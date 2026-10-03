import { useIsMutating, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import {
  ActionIcon,
  Combobox,
  Group,
  Input,
  InputBase,
  Stack,
  Text,
  Tooltip,
  useCombobox,
} from '@mantine/core';
import { changeFoundry } from '../../features/foundry/change-view';
import { loadInitialView } from '../../features/foundry/initial-view';
import { refreshFoundries } from '../../features/foundry/refresh-view';
import { refreshDeployments } from '../../features/foundry/refresh-deployments';
import type { Foundry } from '../../features/foundry/models';
import { ErrorNotice } from '../../shared/ErrorNotice';
import type { FoundryProgress } from '../../features/foundry/progress';
import { AcquisitionProgressModal } from './AcquisitionProgressModal';
import { DeploymentDetails } from '../deployment-details/DeploymentDetails';

function foundryLabel(foundry: Foundry) {
  return `${foundry.name} (${foundry.subscriptionName} - ${foundry.resourceGroupName})`;
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
  const [detailRevision, setDetailRevision] = useState(0);
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
    onSuccess: (view) => {
      client.setQueryData(['foundry', 'initial-view'], view);
      setDetailRevision((revision) => revision + 1);
    },
  });
  const refreshModels = useMutation({
    mutationFn: () => refreshDeployments(setModelsProgress),
    onSuccess: (view) => {
      client.setQueryData(['foundry', 'initial-view'], view);
      setDetailRevision((revision) => revision + 1);
    },
  });
  const busy = useIsMutating() > 0;
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
          <Group gap="sm" wrap="nowrap" h={36}>
            <Tooltip label="Refresh Foundries">
              <ActionIcon
                variant="default"
                size={36}
                aria-label="Refresh Foundries"
                disabled={busy}
                onClick={() => {
                  setRefreshProgress(null);
                  refresh.mutate();
                }}
              >
                <RefreshIcon />
              </ActionIcon>
            </Tooltip>
            <Text size="xs" c="dimmed" style={{ whiteSpace: 'nowrap' }}>
              Last fetched {fetchedAt(view.foundriesFetchedAt)}
            </Text>
          </Group>
        </Group>
      </Stack>
      <section aria-label="Deployments and details">
        <Group className="deployment-workspace-title" justify="flex-start" gap="sm">
          <Text size="sm" fw={500}>
            Deployed Models
          </Text>
          <Tooltip label="Refresh models">
            <ActionIcon
              variant="subtle"
              aria-label="Refresh models"
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
            {view.deployments.length}
            {view.deploymentsFetchedAt && ` · Last fetched ${fetchedAt(view.deploymentsFetchedAt)}`}
          </Text>
        </Group>
        <div className="deployment-workspace">
          <DeploymentDetails
            key={`${view.selectedFoundryId}:${detailRevision}`}
            deployments={view.deployments}
            busy={busy}
          />
        </div>
      </section>
    </Stack>
  );
}

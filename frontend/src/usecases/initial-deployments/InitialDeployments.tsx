import { useIsMutating, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import {
  ActionIcon,
  Button,
  Combobox,
  Group,
  Input,
  InputBase,
  Modal,
  Stack,
  Text,
  Tooltip,
  useCombobox,
} from '@mantine/core';
import { changeFoundry } from '../../features/foundry/change-view';
import { createDeployment } from '../../features/foundry/create-deployment';
import {
  createFoundry,
  type FoundryCreateProgress,
  type FoundryCreateSpec,
} from '../../features/foundry/create-foundry';
import { deleteDeployment } from '../../features/foundry/delete-deployment';
import {
  deleteFoundry,
  inspectFoundryDeletion,
  type FoundryDeleteProgress,
  type FoundryDeletionPlan,
} from '../../features/foundry/delete-foundry';
import { updateDeployment } from '../../features/foundry/update-deployment';
import { loadInitialView } from '../../features/foundry/initial-view';
import { refreshFoundries } from '../../features/foundry/refresh-view';
import { refreshDeployments } from '../../features/foundry/refresh-deployments';
import type {
  Deployment,
  DeploymentCreateSpec,
  DeploymentUpdateSpec,
  Foundry,
} from '../../features/foundry/models';
import { ErrorNotice } from '../../shared/ErrorNotice';
import type { FoundryProgress } from '../../features/foundry/progress';
import { AcquisitionProgressModal } from './AcquisitionProgressModal';
import { AddDeploymentModal } from './AddDeploymentModal';
import { AddFoundryModal } from './AddFoundryModal';
import { CreateFoundryProgressModal } from './CreateFoundryProgressModal';
import { DeleteFoundryProgressModal } from './DeleteFoundryProgressModal';
import { EditDeploymentModal } from './EditDeploymentModal';
import { ConnectionItem, FoundryConnection } from './FoundryConnection';
import { SubscriptionCost } from './SubscriptionCost';
import { RefreshIcon } from './RefreshIcon';
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

function PlusIcon() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" aria-hidden="true">
      <path
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M12 5v14M5 12h14"
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

export function InitialDeployments() {
  const [progress, setProgress] = useState<FoundryProgress | null>(null);
  const [changeProgress, setChangeProgress] = useState<FoundryProgress | null>(null);
  const [refreshProgress, setRefreshProgress] = useState<FoundryProgress | null>(null);
  const [modelsProgress, setModelsProgress] = useState<FoundryProgress | null>(null);
  const [detailRevision, setDetailRevision] = useState(0);
  const [editTarget, setEditTarget] = useState<Deployment | null>(null);
  const [updating, setUpdating] = useState(false);
  const [updateProgress, setUpdateProgress] = useState<FoundryProgress | null>(null);
  const [updateError, setUpdateError] = useState<unknown>(null);
  const [deleteTarget, setDeleteTarget] = useState<Deployment | null>(null);
  const [deleteProgress, setDeleteProgress] = useState<FoundryProgress | null>(null);
  const [addOpened, setAddOpened] = useState(false);
  const [deploying, setDeploying] = useState(false);
  const [deployProgress, setDeployProgress] = useState<FoundryProgress | null>(null);
  const [deployError, setDeployError] = useState<string | null>(null);
  const [addFoundryOpened, setAddFoundryOpened] = useState(false);
  const [createFoundryProgress, setCreateFoundryProgress] = useState<FoundryCreateProgress | null>(
    null,
  );
  const [deletePlan, setDeletePlan] = useState<FoundryDeletionPlan | null>(null);
  const [deleteFoundryProgress, setDeleteFoundryProgress] = useState<FoundryDeleteProgress | null>(
    null,
  );
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
  const remove = useMutation({
    mutationFn: (id: string) => deleteDeployment(id),
    onSuccess: (view) => {
      client.setQueryData(['foundry', 'initial-view'], view);
      setDetailRevision((revision) => revision + 1);
    },
    onSettled: () => setDeleteTarget(null),
  });
  const update = useMutation({
    mutationFn: (spec: DeploymentUpdateSpec) => updateDeployment(spec),
    onSuccess: async (view) => {
      setUpdateProgress({
        foundryPhase: 'completed',
        foundryCount: view.foundries.length,
        selectedFoundryName: editTarget?.deploymentName ?? '',
        modelPhase: 'completed',
        modelCount: view.deployments.length,
      });
      await new Promise((resolve) => setTimeout(resolve, 600));
      setEditTarget(null);
      setUpdateError(null);
      client.setQueryData(['foundry', 'initial-view'], view);
    },
    onError: (err) => setUpdateError(err),
    onSettled: () => {
      setUpdating(false);
      setUpdateProgress(null);
    },
  });
  const deploy = useMutation({
    mutationFn: (spec: DeploymentCreateSpec) => createDeployment(spec),
    onSuccess: async (view, spec) => {
      setDeployProgress({
        foundryPhase: 'completed',
        foundryCount: view.foundries.length,
        selectedFoundryName: spec.deploymentName,
        modelPhase: 'completed',
        modelCount: 0,
      });
      await new Promise((resolve) => setTimeout(resolve, 600));
      setAddOpened(false);
      setDeployError(null);
      client.setQueryData(['foundry', 'initial-view'], view);
      setDetailRevision((revision) => revision + 1);
    },
    onError: (err) => {
      setDeployError(err instanceof Error ? err.message : 'Deployment failed');
    },
    onSettled: () => {
      setDeploying(false);
      setDeployProgress(null);
    },
  });
  const create = useMutation({
    mutationFn: (spec: FoundryCreateSpec) => createFoundry(spec, setCreateFoundryProgress),
    onSuccess: (view) => {
      client.setQueryData(['foundry', 'initial-view'], view);
      setDetailRevision((revision) => revision + 1);
      setAddFoundryOpened(false);
    },
    onSettled: () => setCreateFoundryProgress(null),
  });
  const inspect = useMutation({
    mutationFn: inspectFoundryDeletion,
    onSuccess: setDeletePlan,
  });
  const removeFoundry = useMutation({
    mutationFn: () => deleteFoundry(setDeleteFoundryProgress),
    onSuccess: (view) => {
      client.setQueryData(['foundry', 'initial-view'], view);
      setDetailRevision((revision) => revision + 1);
    },
    onSettled: () => {
      setDeletePlan(null);
      setDeleteFoundryProgress(null);
    },
  });
  const busy = useIsMutating() > 0 || deploying || updating;

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
  const subscriptionID = selected?.id.match(/^\/subscriptions\/([^/]+)/i)?.[1] ?? '';

  return (
    <Stack gap="lg">
      {addFoundryOpened && (
        <AddFoundryModal
          busy={create.isPending}
          error={create.error}
          onClose={() => {
            if (create.isPending) return;
            setAddFoundryOpened(false);
            create.reset();
          }}
          onCreate={(spec) => {
            setCreateFoundryProgress({
              resourceGroupName: spec.resourceGroupName,
              foundryName: spec.foundryName,
              resourceGroupPhase: 'waiting',
              foundryPhase: 'waiting',
              viewPhase: 'waiting',
            } as FoundryCreateProgress);
            create.mutate(spec);
          }}
        />
      )}
      {create.isPending && createFoundryProgress && (
        <CreateFoundryProgressModal progress={createFoundryProgress} />
      )}
      {removeFoundry.isPending && deleteFoundryProgress && (
        <DeleteFoundryProgressModal progress={deleteFoundryProgress} />
      )}
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
      {deleteProgress && (
        <AcquisitionProgressModal
          opened={remove.isPending}
          progress={deleteProgress}
          mode="delete"
        />
      )}
      {deployProgress && (
        <AcquisitionProgressModal opened={deploying} progress={deployProgress} mode="deploy" />
      )}
      {updateProgress && (
        <AcquisitionProgressModal opened={updating} progress={updateProgress} mode="update" />
      )}
      <EditDeploymentModal
        deploymentID={editTarget?.id ?? null}
        busy={updating}
        error={updateError}
        onClose={() => {
          if (updating) return;
          setEditTarget(null);
          setUpdateError(null);
        }}
        onClearError={() => setUpdateError(null)}
        onUpdate={(spec) => {
          setUpdateError(null);
          setUpdating(true);
          setUpdateProgress({
            foundryPhase: 'completed',
            foundryCount: view.foundries.length,
            selectedFoundryName: editTarget?.deploymentName ?? '',
            modelPhase: 'running',
            modelCount: 0,
          });
          setTimeout(() => update.mutate(spec), 700);
        }}
      />
      <AddDeploymentModal
        key={view.selectedFoundryId}
        foundryID={view.selectedFoundryId}
        opened={addOpened}
        onClose={() => {
          setAddOpened(false);
          setDeployError(null);
        }}
        busy={deploying}
        error={deployError}
        onClearError={() => setDeployError(null)}
        existingDeploymentNames={view.deployments.map((d) => d.deploymentName)}
        onDeploy={(spec) => {
          setDeployError(null);
          setDeploying(true);
          setDeployProgress({
            foundryPhase: 'completed',
            foundryCount: view.foundries.length,
            selectedFoundryName: spec.deploymentName,
            modelPhase: 'running',
            modelCount: 0,
          });
          setTimeout(() => {
            deploy.mutate(spec);
          }, 700);
        }}
      />
      <Modal
        opened={deleteTarget !== null && !remove.isPending}
        onClose={() => setDeleteTarget(null)}
        title="Delete deployment"
        centered
      >
        <Stack gap="md">
          <Text size="sm" style={{ overflowWrap: 'anywhere' }}>
            Delete deployment “{deleteTarget?.deploymentName}” from Foundry “{selected?.name}”? This
            cannot be undone.
          </Text>
          <Group justify="flex-end" gap="sm">
            <Button variant="default" onClick={() => setDeleteTarget(null)}>
              Cancel
            </Button>
            <Button
              color="red"
              onClick={() => {
                if (!deleteTarget) return;
                setDeleteProgress({
                  foundryPhase: 'completed',
                  foundryCount: view.foundries.length,
                  selectedFoundryName: deleteTarget.deploymentName,
                  modelPhase: 'running',
                  modelCount: 0,
                });
                remove.mutate(deleteTarget.id);
              }}
            >
              Delete
            </Button>
          </Group>
        </Stack>
      </Modal>
      <Modal
        opened={deletePlan !== null && !removeFoundry.isPending}
        onClose={() => setDeletePlan(null)}
        title="Delete Foundry"
        centered
      >
        <Stack gap="md">
          <Text size="sm" style={{ overflowWrap: 'anywhere' }}>
            {deletePlan?.deleteResourceGroup
              ? `Delete Foundry “${deletePlan?.foundryName}”? Resource group “${deletePlan?.resourceGroupName}” contains only Foundry resources, so the whole resource group will be deleted and the Foundry purged. This cannot be undone.`
              : `Delete Foundry “${deletePlan?.foundryName}”? The Foundry will be deleted and purged. Other resources in resource group “${deletePlan?.resourceGroupName}” will be preserved. This cannot be undone.`}
          </Text>
          <Group justify="flex-end" gap="sm">
            <Button variant="default" onClick={() => setDeletePlan(null)}>
              Cancel
            </Button>
            <Button
              color="red"
              onClick={() => {
                if (!deletePlan) return;
                setDeleteFoundryProgress({
                  foundryName: deletePlan.foundryName,
                  resourceGroupName: deletePlan.resourceGroupName,
                  deleteResourceGroup: deletePlan.deleteResourceGroup,
                  foundryPhase: 'waiting',
                  resourceGroupPhase: 'waiting',
                } as FoundryDeleteProgress);
                removeFoundry.mutate();
              }}
            >
              Delete
            </Button>
          </Group>
        </Stack>
      </Modal>
      <ErrorNotice
        error={
          change.error ||
          refresh.error ||
          refreshModels.error ||
          remove.error ||
          inspect.error ||
          removeFoundry.error
        }
        onClose={() => {
          change.reset();
          refresh.reset();
          refreshModels.reset();
          remove.reset();
          inspect.reset();
          removeFoundry.reset();
        }}
      />
      <Stack gap={6}>
        <Group justify="space-between" gap="sm" align="center" style={{ minHeight: 36 }}>
          <Group gap="sm" align="center">
            <Text size="sm" fw={500} w={140}>
              Foundry
            </Text>
            <Tooltip label="Add Foundry">
              <ActionIcon
                variant="subtle"
                aria-label="Add Foundry"
                disabled={busy}
                onClick={() => {
                  create.reset();
                  setAddFoundryOpened(true);
                }}
              >
                <PlusIcon />
              </ActionIcon>
            </Tooltip>
            <Tooltip label="Refresh Foundries">
              <ActionIcon
                variant="subtle"
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
          <Tooltip label="Delete Foundry">
            <ActionIcon
              variant="subtle"
              color="red"
              aria-label="Delete Foundry"
              disabled={busy || !selected}
              loading={inspect.isPending}
              onClick={() => {
                removeFoundry.reset();
                inspect.mutate();
              }}
            >
              <TrashIcon />
            </ActionIcon>
          </Tooltip>
        </Group>
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
              aria-label="Foundry"
              rightSection={<Combobox.Chevron />}
              rightSectionPointerEvents="none"
              onClick={() => combobox.toggleDropdown()}
              style={{ width: '100%' }}
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
        {subscriptionID && (
          <Group
            gap="xl"
            align="center"
            wrap="nowrap"
            style={{ minHeight: 28, marginTop: 10, paddingLeft: 13 }}
          >
            <ConnectionItem
              name="Subscription ID"
              shown={subscriptionID}
              value={subscriptionID}
              loading={false}
            />
            <SubscriptionCost
              key={view.selectedFoundryId}
              foundryID={view.selectedFoundryId}
              fetchedAt={view.deploymentsFetchedAt}
            />
          </Group>
        )}
        <FoundryConnection
          foundryID={view.selectedFoundryId}
          fetchedAt={view.deploymentsFetchedAt}
        />
      </Stack>
      <section aria-label="Deployments and details">
        <Group
          className="deployment-workspace-title"
          justify="space-between"
          gap="sm"
          align="center"
        >
          <Group gap="sm" align="center">
            <Text size="sm" fw={500} w={140}>
              Deployed Models
            </Text>
            <Tooltip label="Add deployment">
              <ActionIcon
                variant="subtle"
                aria-label="Add deployment"
                disabled={busy || !selected}
                onClick={() => setAddOpened(true)}
              >
                <PlusIcon />
              </ActionIcon>
            </Tooltip>
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
              {view.deploymentsFetchedAt &&
                ` · Last fetched ${fetchedAt(view.deploymentsFetchedAt)}`}
            </Text>
          </Group>
        </Group>
        <div className="deployment-workspace">
          <DeploymentDetails
            key={`${view.selectedFoundryId}:${detailRevision}`}
            foundryID={view.selectedFoundryId}
            deployments={view.deployments}
            busy={busy}
            onDelete={setDeleteTarget}
            onEdit={setEditTarget}
          />
        </div>
      </section>
    </Stack>
  );
}

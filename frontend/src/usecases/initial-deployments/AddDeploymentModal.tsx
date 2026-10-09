import { useEffect, useMemo, useState } from 'react';
import {
  Alert,
  Badge,
  Box,
  Button,
  Checkbox,
  Group,
  Loader,
  Modal,
  ScrollArea,
  Select,
  Slider,
  Stack,
  Text,
  TextInput,
} from '@mantine/core';
import { Events } from '@wailsio/runtime';
import { getModelCatalog, type ModelCatalogView } from '../../features/foundry/model-catalog';
import type { DeploymentCreateSpec, ModelCatalogItem } from '../../features/foundry/models';

function SearchIcon() {
  return (
    <svg
      width="15"
      height="15"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <circle cx="11" cy="11" r="8" />
      <path d="m21 21-4.35-4.35" />
    </svg>
  );
}

interface AddDeploymentModalProps {
  foundryID: string;
  opened: boolean;
  onClose: () => void;
  onDeploy: (spec: DeploymentCreateSpec) => void;
  busy?: boolean;
  error?: string | null;
  onClearError?: () => void;
  existingDeploymentNames?: string[];
}

const EMPTY_CATALOG: ModelCatalogItem[] = [];

export function AddDeploymentModal({
  foundryID,
  opened,
  onClose,
  onDeploy,
  busy,
  error,
  onClearError,
  existingDeploymentNames,
}: AddDeploymentModalProps) {
  const [catalogView, setCatalogView] = useState<ModelCatalogView | null>(null);
  const [catalogError, setCatalogError] = useState<string | null>(null);
  const catalog = catalogView?.models ?? EMPTY_CATALOG;
  useEffect(() => {
    if (!opened) return;
    let active = true;
    let request = 0;
    const load = () => {
      const currentRequest = ++request;
      void getModelCatalog(foundryID)
        .then((result) => {
          if (active && currentRequest === request) {
            setCatalogView(result);
            setCatalogError(null);
          }
        })
        .catch((cause: unknown) => {
          if (active && currentRequest === request) {
            setCatalogError(cause instanceof Error ? cause.message : String(cause));
          }
        });
    };
    setCatalogView(null);
    setCatalogError(null);
    const unsubscribe = Events.On('foundry:capacity-ready', (event) => {
      if (event.data === foundryID) load();
    });
    load();
    return () => {
      active = false;
      unsubscribe();
    };
  }, [opened, foundryID]);

  // Filter state
  const [searchQuery, setSearchQuery] = useState('');
  const [publisherFilter, setPublisherFilter] = useState<string>('all');
  const [optionFilter, setOptionFilter] = useState<string>('all');
  const [selectedTasks, setSelectedTasks] = useState<string[]>([]);

  // Form state
  const [selectedModelName, setSelectedModelName] = useState<string | null>(null);
  const selectedModel = catalog.find((model) => model.name === selectedModelName) ?? null;
  const [deploymentName, setDeploymentName] = useState('');
  const [selectedVersion, setSelectedVersion] = useState('');
  const [selectedSKU, setSelectedSKU] = useState('');
  const [capacity, setCapacity] = useState<number>(80000);
  const [capacityInputText, setCapacityInputText] = useState('80000');
  const [capacityEntered, setCapacityEntered] = useState(false);
  const [upgradePolicy, setUpgradePolicy] = useState('OnceNewDefaultVersionAvailable');

  const isDuplicateName = useMemo(() => {
    const trimmed = deploymentName.trim().toLowerCase();
    if (!trimmed) return false;
    return (existingDeploymentNames ?? []).some((name) => name.toLowerCase() === trimmed);
  }, [deploymentName, existingDeploymentNames]);

  const availableSKUs = selectedModel?.skus ?? [];
  const availableVersions = selectedModel?.versions ?? [];
  const deploymentOptionsUnavailable = availableSKUs.length === 0 || availableVersions.length === 0;

  const currentSKU = availableSKUs.find((s) => s.name === selectedSKU);

  const currentMaxCapacity = useMemo(() => {
    if (catalogView?.quotaStatus !== 'ready') return null;
    return currentSKU?.maxCapacity ?? null;
  }, [catalogView?.quotaStatus, currentSKU]);
  const quotaLoading = catalogView?.quotaStatus === 'loading';
  const quotaError = catalogView?.quotaStatus === 'error' ? catalogView.quotaError : null;
  const capacityUnavailable = currentMaxCapacity === null || currentMaxCapacity < 1000;
  const parsedCapacity = Number(capacityInputText);
  const capacityInvalid =
    selectedModel?.option === 'Standard' &&
    (capacityUnavailable ||
      !Number.isInteger(parsedCapacity) ||
      parsedCapacity < 1000 ||
      parsedCapacity % 1000 !== 0 ||
      parsedCapacity > currentMaxCapacity!);

  useEffect(() => {
    if (!capacityEntered && currentMaxCapacity !== null) {
      const half = Math.floor(currentMaxCapacity / 2000) * 1000;
      setCapacity(half);
      setCapacityInputText(String(half));
    }
  }, [capacityEntered, currentMaxCapacity]);

  // Automatically select first model when catalog loads
  useEffect(() => {
    if (catalog.length > 0 && !selectedModelName) {
      const initial = catalog[0];
      setSelectedModelName(initial.name);
      setDeploymentName(initial.name);
      setSelectedVersion(initial.versions?.[0] ?? '');
      const skus = initial.skus ?? [];
      const defSku = skus.find((s) => s.name === 'GlobalStandard') ?? skus[0];
      setSelectedSKU(defSku?.name ?? '');
      const skuCap = defSku?.maxCapacity;
      if (skuCap) {
        const half = Math.floor(skuCap / 2000) * 1000;
        setCapacity(half);
        setCapacityInputText(String(half));
      }
    }
  }, [catalog, selectedModelName]);

  // When model is changed, sync deployment name and defaults
  const handleSelectModel = (model: ModelCatalogItem) => {
    setSelectedModelName(model.name);
    setCapacityEntered(false);
    setDeploymentName(model.name);
    setSelectedVersion(model.versions?.[0] ?? '');
    const skus = model.skus ?? [];
    const defSku = skus.find((s) => s.name === 'GlobalStandard') ?? skus[0];
    setSelectedSKU(defSku?.name ?? '');
    const skuCap = defSku?.maxCapacity;
    if (skuCap) {
      const half = Math.floor(skuCap / 2000) * 1000;
      setCapacity(half);
      setCapacityInputText(String(half));
    }
  };

  // Publisher options derived from catalog
  const publisherOptions = useMemo(() => {
    const set = new Set<string>();
    catalog.forEach((m) => set.add(m.publisher));
    return [
      { value: 'all', label: 'All publishers' },
      ...Array.from(set).map((p) => ({ value: p, label: p })),
    ];
  }, [catalog]);

  // Filtered model list
  const filteredModels = useMemo(() => {
    const q = searchQuery.toLowerCase().trim();
    return catalog.filter((m) => {
      if (q && !m.name.toLowerCase().includes(q) && !m.publisher.toLowerCase().includes(q)) {
        return false;
      }
      if (publisherFilter !== 'all' && m.publisher !== publisherFilter) {
        return false;
      }
      if (optionFilter !== 'all' && m.option !== optionFilter) {
        return false;
      }
      if (selectedTasks.length > 0) {
        const hasTask = (m.tasks ?? []).some((t) => selectedTasks.includes(t));
        if (!hasTask) return false;
      }
      return true;
    });
  }, [catalog, searchQuery, publisherFilter, optionFilter, selectedTasks]);

  const handleSelectSKU = (newSkuName: string) => {
    setSelectedSKU(newSkuName);
    onClearError?.();
    const skuItem = availableSKUs.find((s) => s.name === newSkuName);
    if (skuItem?.maxCapacity) {
      const newMax = skuItem.maxCapacity;
      setCapacity((prev) => {
        const clamped = Math.min(prev, newMax);
        setCapacityInputText(String(clamped));
        return clamped;
      });
    }
  };

  // Handle capacity direct input
  const handleCapacityTextChange = (val: string) => {
    setCapacityEntered(true);
    setCapacityInputText(val);
    const parsed = Number(val);
    if (Number.isFinite(parsed)) setCapacity(parsed);
  };

  const handleSliderChange = (val: number) => {
    setCapacityEntered(true);
    setCapacity(val);
    setCapacityInputText(String(val));
  };

  const handleDeploy = () => {
    if (
      !selectedModel ||
      !deploymentName.trim() ||
      !selectedVersion ||
      !selectedSKU ||
      capacityInvalid ||
      catalogError
    )
      return;

    const spec: DeploymentCreateSpec = {
      deploymentName: deploymentName.trim(),
      modelName: selectedModel.name,
      version: selectedVersion,
      sku: selectedSKU,
      capacity: selectedModel.option === 'Pay-as-you-go' ? null : capacity,
      upgradePolicy: upgradePolicy,
    };
    onDeploy(spec);
  };

  return (
    <Modal
      opened={opened}
      onClose={onClose}
      size={980}
      centered
      title="Add deployment"
      closeButtonProps={{ disabled: busy }}
      padding={0}
      styles={{
        header: {
          borderBottom: '1px solid var(--mantine-color-border)',
          background: 'var(--mantine-color-dark-8)',
          margin: 0,
          padding: '12px 20px',
        },
        title: {
          fontWeight: 600,
        },
        body: {
          height: 580,
          display: 'flex',
          flexDirection: 'column',
          overflow: 'hidden',
          padding: 0,
        },
        content: {
          overflow: 'hidden',
        },
      }}
    >
      <Box
        style={{
          flex: 1,
          minHeight: 0,
          display: 'flex',
          flexDirection: 'column',
          overflow: 'hidden',
        }}
      >
        {catalogError ? (
          <Alert color="red" title="Model catalog unavailable" m="md">
            {catalogError}
          </Alert>
        ) : !catalogView ? (
          <Box
            style={{
              flex: 1,
              minHeight: 0,
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              justifyContent: 'center',
              gap: 12,
            }}
          >
            <Loader size="md" />
            <Text size="sm" c="dimmed">
              Loading model catalog...
            </Text>
          </Box>
        ) : (
          <Box
            style={{
              flex: 1,
              minHeight: 0,
              display: 'grid',
              gridTemplateColumns: '440px 1fr',
              overflow: 'hidden',
            }}
          >
            {/* Left Pane: Search, Filter, List */}
            <Box
              style={{
                borderRight: '1px solid var(--mantine-color-border)',
                display: 'flex',
                flexDirection: 'column',
                background: 'var(--mantine-color-dark-8)',
                minHeight: 0,
                overflow: 'hidden',
              }}
            >
              <Box
                p="xs"
                style={{
                  borderBottom: '1px solid var(--mantine-color-border)',
                  background: 'var(--mantine-color-dark-7)',
                }}
              >
                <Stack gap={8}>
                  <TextInput
                    size="xs"
                    placeholder="Search models..."
                    leftSection={<SearchIcon />}
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.currentTarget.value)}
                  />
                  <Group grow gap={6}>
                    <Select
                      size="xs"
                      data={publisherOptions}
                      value={publisherFilter}
                      onChange={(val) => setPublisherFilter(val ?? 'all')}
                      allowDeselect={false}
                    />
                    <Select
                      size="xs"
                      data={[
                        { value: 'all', label: 'All options' },
                        { value: 'Standard', label: 'Standard' },
                        { value: 'Pay-as-you-go', label: 'Pay-as-you-go' },
                      ]}
                      value={optionFilter}
                      onChange={(val) => setOptionFilter(val ?? 'all')}
                      allowDeselect={false}
                    />
                  </Group>
                  <Group gap={10} style={{ paddingTop: 2 }}>
                    {['Chat', 'Reasoning', 'Embeddings', 'Multimodal'].map((task) => (
                      <Checkbox
                        key={task}
                        size="xs"
                        label={task}
                        checked={selectedTasks.includes(task)}
                        onChange={(e) => {
                          if (e.currentTarget.checked) {
                            setSelectedTasks([...selectedTasks, task]);
                          } else {
                            setSelectedTasks(selectedTasks.filter((t) => t !== task));
                          }
                        }}
                      />
                    ))}
                  </Group>
                </Stack>
              </Box>
              <ScrollArea style={{ flex: 1 }}>
                {filteredModels.length === 0 ? (
                  <Text p="md" size="sm" c="dimmed" ta="center">
                    No models match the filter.
                  </Text>
                ) : (
                  <Box component="ul" style={{ listStyle: 'none', margin: 0, padding: 0 }}>
                    {filteredModels.map((m) => {
                      const isSelected = selectedModel?.name === m.name;
                      return (
                        <Box
                          component="li"
                          key={m.name}
                          onClick={() => handleSelectModel(m)}
                          p="xs"
                          px="sm"
                          style={{
                            borderBottom: '1px solid var(--mantine-color-dark-5)',
                            cursor: 'pointer',
                            background: isSelected
                              ? 'var(--mantine-color-primary-light)'
                              : undefined,
                            borderLeft: isSelected
                              ? '3px solid var(--mantine-primary-color)'
                              : '3px solid transparent',
                          }}
                        >
                          <Group justify="space-between" wrap="nowrap" mb={2}>
                            <Text
                              size="sm"
                              fw={600}
                              truncate
                              c={isSelected ? 'var(--mantine-color-white)' : undefined}
                            >
                              {m.name}
                            </Text>
                            <Group gap={4} wrap="nowrap">
                              <Badge
                                size="xs"
                                variant="light"
                                color={m.option === 'Pay-as-you-go' ? 'yellow' : 'blue'}
                              >
                                {m.option}
                              </Badge>
                              {m.tasks?.[0] && (
                                <Badge size="xs" variant="default">
                                  {m.tasks[0]}
                                </Badge>
                              )}
                            </Group>
                          </Group>
                          <Text size="xs" c="dimmed">
                            {m.publisher} · {m.sub}
                          </Text>
                        </Box>
                      );
                    })}
                  </Box>
                )}
              </ScrollArea>
            </Box>

            {/* Right Pane: Configuration */}
            <ScrollArea
              style={{ flex: 1, minHeight: 0, background: 'var(--mantine-color-dark-7)' }}
            >
              {selectedModel ? (
                <Stack p="md" gap="md">
                  {error && (
                    <Alert
                      color="red"
                      variant="light"
                      withCloseButton
                      onClose={onClearError}
                      title="Deployment failed"
                    >
                      {error}
                    </Alert>
                  )}
                  {quotaError && selectedModel.option === 'Standard' && (
                    <Alert color="red" title="Quota unavailable">
                      {quotaError}
                    </Alert>
                  )}
                  <Box pb="xs" style={{ borderBottom: '1px solid var(--mantine-color-border)' }}>
                    <Group justify="space-between" align="flex-start">
                      <Box>
                        <Text size="lg" fw={700} c="white">
                          {selectedModel.name}
                        </Text>
                        <Text size="xs" c="dimmed">
                          {selectedModel.publisher} · {selectedModel.sub}
                        </Text>
                      </Box>
                      <Badge
                        size="sm"
                        variant="light"
                        color={selectedModel.option === 'Pay-as-you-go' ? 'yellow' : 'blue'}
                      >
                        {selectedModel.option}
                      </Badge>
                    </Group>
                  </Box>

                  {deploymentOptionsUnavailable && (
                    <Alert color="red" title="Deployment unavailable">
                      No deployment options available for this model.
                    </Alert>
                  )}

                  <TextInput
                    label="Deployment name"
                    required
                    value={deploymentName}
                    error={
                      isDuplicateName
                        ? 'This deployment name already exists in this Foundry.'
                        : undefined
                    }
                    onChange={(e) => {
                      onClearError?.();
                      setDeploymentName(e.currentTarget.value);
                    }}
                  />

                  <Select
                    label="Model version"
                    data={availableVersions.map((v) => ({
                      value: v,
                      label: v,
                    }))}
                    value={selectedVersion}
                    onChange={(val) => {
                      onClearError?.();
                      setSelectedVersion(val ?? '');
                    }}
                    allowDeselect={false}
                  />

                  {availableSKUs.length > 0 && selectedModel.option !== 'Pay-as-you-go' && (
                    <Select
                      label="Deployment type (SKU)"
                      data={availableSKUs.map((s) => ({
                        value: s.name,
                        label: s.name,
                      }))}
                      value={selectedSKU}
                      onChange={(val) => {
                        if (val) handleSelectSKU(val);
                      }}
                      allowDeselect={false}
                    />
                  )}

                  {selectedModel.option === 'Standard' && (
                    <Stack gap={6}>
                      <Group justify="space-between" align="flex-end">
                        <Text size="sm" fw={500}>
                          Capacity
                        </Text>
                        <Text size="xs" c="dimmed">
                          <Text span fw={600} c="white">
                            {quotaLoading && !capacityEntered ? '—' : capacity.toLocaleString()}
                          </Text>{' '}
                          /{' '}
                          <Text span fw={600} c="white">
                            {quotaLoading ? (
                              <span role="status">Loading...</span>
                            ) : (
                              (currentMaxCapacity?.toLocaleString() ?? 'Not available')
                            )}
                          </Text>{' '}
                          TPM (Available)
                        </Text>
                      </Group>
                      <TextInput
                        aria-label="Capacity"
                        disabled={capacityUnavailable}
                        error={
                          !capacityUnavailable && capacityInvalid
                            ? 'Enter a capacity within the available quota, in steps of 1,000.'
                            : undefined
                        }
                        value={capacityInputText}
                        onChange={(e) => {
                          onClearError?.();
                          handleCapacityTextChange(e.currentTarget.value);
                        }}
                        placeholder="e.g. 80000"
                      />
                      <Box pt="xs" pb="xs">
                        <Slider
                          min={1000}
                          max={Math.max(1000, currentMaxCapacity ?? 1000)}
                          disabled={capacityUnavailable}
                          step={1000}
                          value={capacity}
                          onChange={handleSliderChange}
                          label={(val) => val.toLocaleString()}
                        />
                      </Box>
                    </Stack>
                  )}

                  {selectedModel.option === 'Pay-as-you-go' && (
                    <Box
                      p="sm"
                      style={{
                        background: 'var(--mantine-color-dark-8)',
                        border: '1px solid var(--mantine-color-border)',
                        borderRadius: 'var(--mantine-radius-sm)',
                      }}
                    >
                      <Text size="xs" fw={600} c="yellow" mb={6}>
                        Serverless API (Pay-as-you-go)
                      </Text>
                      <Group grow gap="xs">
                        <Box>
                          <Text size="xs" c="dimmed">
                            Input rate
                          </Text>
                          <Text size="sm" fw={600}>
                            {selectedModel.inputRate ?? 'N/A'}
                          </Text>
                        </Box>
                        <Box>
                          <Text size="xs" c="dimmed">
                            Output rate
                          </Text>
                          <Text size="sm" fw={600}>
                            {selectedModel.outputRate ?? 'N/A'}
                          </Text>
                        </Box>
                      </Group>
                    </Box>
                  )}

                  <Select
                    label="Version update policy"
                    data={[
                      {
                        value: 'OnceNewDefaultVersionAvailable',
                        label: 'Once a new default version is available (Default)',
                      },
                      {
                        value: 'OnceNewVersionAvailable',
                        label: 'Once a new version is available',
                      },
                      {
                        value: 'NoAutoUpdate',
                        label: 'No auto-update',
                      },
                    ]}
                    value={upgradePolicy}
                    onChange={(val) => setUpgradePolicy(val ?? 'OnceNewDefaultVersionAvailable')}
                    allowDeselect={false}
                  />
                </Stack>
              ) : (
                <Box p="lg" ta="center">
                  <Text size="sm" c="dimmed">
                    No model selected.
                  </Text>
                </Box>
              )}
            </ScrollArea>
          </Box>
        )}
      </Box>
      <Box
        p="sm"
        px="md"
        style={{
          borderTop: '1px solid var(--mantine-color-border)',
          background: 'var(--mantine-color-dark-8)',
          display: 'flex',
          justifyContent: 'flex-end',
          gap: 10,
        }}
      >
        <Button variant="default" onClick={onClose} disabled={busy}>
          Cancel
        </Button>
        <Button
          onClick={handleDeploy}
          disabled={
            busy ||
            !selectedModel ||
            !deploymentName.trim() ||
            !selectedVersion ||
            !selectedSKU ||
            isDuplicateName ||
            capacityInvalid ||
            !!catalogError
          }
        >
          Deploy
        </Button>
      </Box>
    </Modal>
  );
}

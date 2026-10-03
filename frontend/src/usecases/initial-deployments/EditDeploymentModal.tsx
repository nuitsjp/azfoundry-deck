import {
  Box,
  Button,
  Group,
  Loader,
  Modal,
  Select,
  Slider,
  Stack,
  Text,
  TextInput,
} from '@mantine/core';
import { useQuery } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import { getDeploymentSettings } from '../../features/foundry/deployment-settings';
import type { DeploymentSettings, DeploymentUpdateSpec } from '../../features/foundry/models';
import { ErrorNotice } from '../../shared/ErrorNotice';

const upgradePolicies = [
  { value: 'OnceNewDefaultVersionAvailable', label: 'Upgrade to new default' },
  { value: 'OnceCurrentVersionExpired', label: 'Upgrade on retirement' },
  { value: 'NoAutoUpgrade', label: 'No automatic upgrade' },
];

function snapCapacity(value: number, maximum: number) {
  const stepped = Math.round(value / 1000) * 1000;
  return Math.min(maximum, Math.max(1000, stepped));
}

export function EditDeploymentModal({
  deploymentID,
  busy,
  error,
  onClose,
  onClearError,
  onUpdate,
}: {
  deploymentID: string | null;
  busy: boolean;
  error: unknown;
  onClose: () => void;
  onClearError: () => void;
  onUpdate: (spec: DeploymentUpdateSpec) => void;
}) {
  const [attempt, setAttempt] = useState(0);
  const [baseline, setBaseline] = useState<DeploymentSettings | null>(null);
  const [version, setVersion] = useState('');
  const [policy, setPolicy] = useState('');
  const [capacity, setCapacity] = useState(0);
  const [capacityText, setCapacityText] = useState('');
  const settings = useQuery({
    queryKey: ['foundry', 'deployment-settings', deploymentID, attempt],
    queryFn: () => getDeploymentSettings(deploymentID!),
    enabled: deploymentID !== null,
    retry: false,
    gcTime: 0,
  });

  useEffect(() => {
    if (!settings.data) return;
    const loaded = settings.data;
    const current = loaded.capacity ?? 0;
    setBaseline(loaded);
    setVersion(loaded.version);
    setPolicy(loaded.upgradePolicy);
    setCapacity(current);
    setCapacityText(String(current));
  }, [settings.data]);

  const opened = deploymentID !== null;
  const standard = baseline !== null && baseline.option !== 'Pay-as-you-go';
  const maximum = baseline?.capacityMaximum ?? 0;
  const versionList = baseline?.versions ?? [];
  const versions = baseline
    ? versionList.includes(baseline.version)
      ? versionList
      : [baseline.version, ...versionList]
    : [];
  const policies =
    baseline && upgradePolicies.some((item) => item.value === baseline.upgradePolicy)
      ? upgradePolicies
      : [...upgradePolicies, { value: policy, label: policy }];
  const overMaximum = standard && maximum > 0 && capacity > maximum;
  const unchanged =
    baseline !== null &&
    version === baseline.version &&
    policy === baseline.upgradePolicy &&
    (!standard || capacity === (baseline.capacity ?? 0));

  function changeCapacityText(value: string) {
    onClearError();
    setCapacityText(value);
    const parsed = Number.parseInt(value, 10);
    if (Number.isNaN(parsed) || maximum <= 0) return;
    const next = snapCapacity(parsed, maximum);
    setCapacity(next);
  }

  function changeSlider(value: number) {
    onClearError();
    const next = snapCapacity(value, maximum);
    setCapacity(next);
    setCapacityText(String(next));
  }

  return (
    <Modal
      opened={opened}
      onClose={onClose}
      title="Edit deployment"
      centered
      closeOnEscape={!busy}
      closeOnClickOutside={!busy}
      closeButtonProps={{ disabled: busy }}
    >
      {settings.isPending ? (
        <Group justify="center" py="xl" role="status">
          <Loader size="sm" />
          <Text size="sm">Loading deployment settings...</Text>
        </Group>
      ) : settings.isError ? (
        <Stack gap="md">
          <ErrorNotice error={settings.error} />
          <Group justify="flex-end">
            <Button variant="default" onClick={onClose}>
              Cancel
            </Button>
            <Button
              variant="default"
              onClick={() => {
                onClearError();
                setAttempt((value) => value + 1);
              }}
            >
              Retry
            </Button>
          </Group>
        </Stack>
      ) : baseline ? (
        <Stack gap="md">
          <ErrorNotice error={error} />
          <TextInput label="Deployment name" value={baseline.deploymentName} readOnly />
          <TextInput label="Model" value={baseline.modelName} readOnly />
          <TextInput label="SKU" value={baseline.skuName || 'Not set'} readOnly />
          <Select
            label="Version"
            data={versions}
            value={version}
            onChange={(value) => {
              onClearError();
              setVersion(value ?? '');
            }}
            allowDeselect={false}
            disabled={busy}
          />
          {standard && maximum > 0 && (
            <Stack gap={6}>
              <Group justify="space-between" align="flex-end">
                <Text size="sm" fw={500}>
                  Capacity
                </Text>
                <Text size="xs" c="dimmed">
                  <Text span fw={600} c="white">
                    {capacity.toLocaleString('en-US')}
                  </Text>{' '}
                  /{' '}
                  <Text span fw={600} c="white">
                    {maximum.toLocaleString('en-US')}
                  </Text>{' '}
                  {baseline.capacityUnit ?? 'TPM'}
                </Text>
              </Group>
              <TextInput
                aria-label="Capacity"
                value={capacityText}
                onChange={(event) => changeCapacityText(event.currentTarget.value)}
                disabled={busy}
              />
              <Box pt="xs">
                <Slider
                  min={1000}
                  max={maximum}
                  step={1000}
                  value={Math.min(maximum, Math.max(1000, capacity))}
                  onChange={changeSlider}
                  label={(value) => value.toLocaleString('en-US')}
                  disabled={busy}
                />
              </Box>
            </Stack>
          )}
          <Select
            label="Upgrade policy"
            data={policies}
            value={policy}
            onChange={(value) => {
              onClearError();
              setPolicy(value ?? '');
            }}
            allowDeselect={false}
            disabled={busy}
          />
          <Group justify="flex-end" gap="sm">
            <Button variant="default" onClick={onClose} disabled={busy}>
              Cancel
            </Button>
            <Button
              disabled={busy || unchanged || overMaximum || version === '' || policy === ''}
              onClick={() =>
                onUpdate({
                  deploymentId: baseline.deploymentId,
                  version,
                  capacity: standard ? capacity : null,
                  upgradePolicy: policy,
                })
              }
            >
              Update
            </Button>
          </Group>
        </Stack>
      ) : null}
    </Modal>
  );
}

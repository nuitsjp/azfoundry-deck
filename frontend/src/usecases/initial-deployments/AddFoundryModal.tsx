import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import {
  Badge,
  Button,
  Divider,
  Group,
  Modal,
  Select,
  Stack,
  Text,
  TextInput,
} from '@mantine/core';
import {
  getFoundryRegions,
  getFoundrySubscriptions,
  type FoundryCreateSpec,
} from '../../features/foundry/create-foundry';
import { ErrorNotice } from '../../shared/ErrorNotice';

export function AddFoundryModal({
  busy,
  error,
  onClose,
  onCreate,
}: {
  busy: boolean;
  error: unknown;
  onClose: () => void;
  onCreate: (spec: FoundryCreateSpec) => void;
}) {
  const [subscription, setSubscription] = useState<string | null>(null);
  const [region, setRegion] = useState<string | null | undefined>(undefined);
  const [keyword, setKeyword] = useState('sample');
  const [resourceGroupOverride, setResourceGroupOverride] = useState<string>();
  const [foundryOverride, setFoundryOverride] = useState<string>();
  const subscriptions = useQuery({
    queryKey: ['foundry', 'subscriptions'],
    queryFn: getFoundrySubscriptions,
    retry: false,
  });
  const subscriptionID = subscription ?? subscriptions.data?.[0]?.id ?? '';
  const regions = useQuery({
    queryKey: ['foundry', 'regions', subscriptionID],
    queryFn: () => getFoundryRegions(subscriptionID),
    enabled: subscriptionID !== '',
    retry: false,
  });
  const regionName =
    region === undefined
      ? (regions.data?.find((candidate) => candidate.name === 'eastus2')?.name ?? null)
      : region;
  const key = keyword
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9-]/g, '-')
    .replace(/-+/g, '-')
    .replace(/^-|-$/g, '');
  const resourceGroupName =
    resourceGroupOverride ?? (key && regionName ? `rg-${key}-${regionName}` : '');
  const foundryName = foundryOverride ?? (key && regionName ? `foundry-${key}-${regionName}` : '');
  const valid =
    !!subscriptionID && !!regionName && !!key && !!resourceGroupName.trim() && !!foundryName.trim();
  function regenerate() {
    setResourceGroupOverride(undefined);
    setFoundryOverride(undefined);
  }

  return (
    <Modal
      opened
      onClose={onClose}
      title="Add Foundry"
      centered
      size={580}
      closeOnEscape={!busy}
      closeOnClickOutside={!busy}
      withCloseButton={!busy}
    >
      <form
        onSubmit={(event) => {
          event.preventDefault();
          if (!valid || busy || !regionName) return;
          onCreate({
            subscriptionId: subscriptionID,
            resourceGroupName: resourceGroupName.trim(),
            foundryName: foundryName.trim(),
            region: regionName,
          } as FoundryCreateSpec);
        }}
      >
        <Stack gap="md">
          <Text size="sm" c="dimmed">
            Create a new resource group and Foundry.
          </Text>
          <ErrorNotice error={error || subscriptions.error || regions.error} />
          <Select
            label="Subscription"
            placeholder={subscriptions.isPending ? 'Loading subscriptions…' : 'Select subscription'}
            data={(subscriptions.data ?? []).map((item) => ({
              value: item.id,
              label: item.displayName,
            }))}
            value={subscriptionID || null}
            disabled={busy || subscriptions.isPending}
            onChange={(value) => {
              setSubscription(value);
              setRegion(undefined);
              regenerate();
            }}
            allowDeselect={false}
          />
          <Group align="flex-start" grow>
            <Select
              label="Region"
              placeholder={regions.isFetching ? 'Loading regions…' : 'Select region'}
              description="Used for both resources."
              data={(regions.data ?? []).map((item) => ({
                value: item.name,
                label: item.displayName,
              }))}
              value={regionName}
              disabled={busy || !subscriptionID || regions.isFetching}
              onChange={(value) => {
                setRegion(value);
                regenerate();
              }}
              allowDeselect={false}
              searchable
            />
            <TextInput
              label="Keyword"
              description="Use letters, numbers and hyphens."
              value={keyword}
              disabled={busy}
              autoComplete="off"
              spellCheck={false}
              onChange={(event) => {
                setKeyword(event.currentTarget.value);
                regenerate();
              }}
            />
          </Group>
          <Divider />
          <TextInput
            label="Resource group name"
            value={resourceGroupName}
            disabled={busy}
            rightSection={
              <Badge size="xs" variant="light">
                {resourceGroupOverride === undefined ? 'Auto' : 'Edited'}
              </Badge>
            }
            rightSectionWidth={65}
            autoComplete="off"
            spellCheck={false}
            onChange={(event) => setResourceGroupOverride(event.currentTarget.value)}
          />
          <TextInput
            label="Foundry name"
            value={foundryName}
            disabled={busy}
            rightSection={
              <Badge size="xs" variant="light">
                {foundryOverride === undefined ? 'Auto' : 'Edited'}
              </Badge>
            }
            rightSectionWidth={65}
            autoComplete="off"
            spellCheck={false}
            onChange={(event) => setFoundryOverride(event.currentTarget.value)}
          />
          <Text size="xs" c="dimmed">
            Names can be edited directly. Changing the keyword or region regenerates both names.
          </Text>
          <Group justify="flex-end">
            <Button variant="default" disabled={busy} onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" disabled={!valid || busy || regions.isFetching}>
              Create
            </Button>
          </Group>
        </Stack>
      </form>
    </Modal>
  );
}

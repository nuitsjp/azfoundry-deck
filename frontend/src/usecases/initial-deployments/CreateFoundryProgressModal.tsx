import { Badge, Box, Group, Loader, Modal, Stack, Text } from '@mantine/core';
import type { FoundryCreateProgress } from '../../features/foundry/create-foundry';

export function Step({ label, name, phase }: { label: string; name: string; phase: string }) {
  return (
    <Group wrap="nowrap" py="md" style={{ borderTop: '1px solid var(--mantine-color-dark-4)' }}>
      <Box w={22} ta="center">
        {phase === 'running' ? (
          <Loader size={18} />
        ) : (
          <Text c={phase === 'completed' ? 'teal' : 'dimmed'} aria-hidden="true">
            {phase === 'completed' ? '✓' : '○'}
          </Text>
        )}
      </Box>
      <Stack gap={3} style={{ flex: 1, minWidth: 0 }}>
        <Text fw={500}>{label}</Text>
        <Text size="xs" c="dimmed" style={{ overflowWrap: 'anywhere' }}>
          {name}
        </Text>
      </Stack>
      <Badge
        color={phase === 'completed' ? 'teal' : phase === 'running' ? 'blue' : 'gray'}
        variant="light"
      >
        {phase === 'completed' ? 'Completed' : phase === 'running' ? 'In progress' : 'Waiting'}
      </Badge>
    </Group>
  );
}

export function CreateFoundryProgressModal({ progress }: { progress: FoundryCreateProgress }) {
  return (
    <Modal
      opened
      onClose={() => {}}
      title="Creating Foundry"
      centered
      size={620}
      zIndex={300}
      withCloseButton={false}
      closeOnEscape={false}
      closeOnClickOutside={false}
    >
      <Text size="sm" c="dimmed" mb="md">
        Creating the resource group and Foundry.
      </Text>
      <Box aria-live="polite" aria-busy="true">
        <Step
          label="Create resource group"
          name={progress.resourceGroupName}
          phase={progress.resourceGroupPhase}
        />
        <Step label="Create Foundry" name={progress.foundryName} phase={progress.foundryPhase} />
        <Step
          label="Update Home"
          name="Update the list and select the new Foundry"
          phase={progress.viewPhase}
        />
      </Box>
      <Text size="xs" c="dimmed" mt="md">
        Please wait until creation is complete.
      </Text>
    </Modal>
  );
}

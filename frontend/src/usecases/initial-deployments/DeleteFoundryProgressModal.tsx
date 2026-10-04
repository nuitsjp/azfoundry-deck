import { Box, Modal, Text } from '@mantine/core';
import type { FoundryDeleteProgress } from '../../features/foundry/delete-foundry';
import { Step } from './CreateFoundryProgressModal';

export function DeleteFoundryProgressModal({ progress }: { progress: FoundryDeleteProgress }) {
  return (
    <Modal
      opened
      onClose={() => {}}
      title="Deleting Foundry"
      centered
      size={620}
      zIndex={300}
      withCloseButton={false}
      closeOnEscape={false}
      closeOnClickOutside={false}
    >
      <Text size="sm" c="dimmed" mb="md">
        Deleting the Foundry, purging it and deleting the resource group. This can take a few
        minutes.
      </Text>
      <Box aria-live="polite" aria-busy="true">
        <Step label="Delete Foundry" name={progress.foundryName} phase={progress.foundryPhase} />
        <Step label="Purge Foundry" name={progress.foundryName} phase={progress.purgePhase} />
        <Step
          label="Delete resource group"
          name={progress.resourceGroupName}
          phase={progress.resourceGroupPhase}
        />
        <Step
          label="Update Home"
          name="Update the list and select the next Foundry"
          phase={progress.viewPhase}
        />
      </Box>
      <Text size="xs" c="dimmed" mt="md">
        Please wait until deletion is complete.
      </Text>
    </Modal>
  );
}

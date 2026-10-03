import { Badge, Box, Loader, Modal, Stack, Text } from '@mantine/core';
import type { ReactNode } from 'react';
import type { FoundryProgress, AcquisitionPhase } from '../../features/foundry/progress';

const colors: Record<AcquisitionPhase, string> = {
  waiting: 'gray',
  running: 'blue',
  completed: 'teal',
};

const labels: Record<AcquisitionPhase, string> = {
  waiting: 'Waiting',
  running: 'Loading',
  completed: 'Completed',
};

function Mark({ phase }: { phase: AcquisitionPhase }) {
  if (phase === 'running') return <Loader size={16} />;
  if (phase === 'completed')
    return (
      <svg
        width="18"
        height="18"
        viewBox="0 0 16 16"
        fill="none"
        stroke="var(--mantine-color-teal-4)"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
        aria-hidden="true"
      >
        <path d="M3 8.5l3.2 3.2L13 4.8" />
      </svg>
    );
  return (
    <Box w={8} h={8} style={{ borderRadius: '50%', background: 'var(--mantine-color-dark-4)' }} />
  );
}

// Every row keeps the same height and layout in every phase, so the modal never changes size.
function Step({
  label,
  phase,
  bordered,
  children,
}: {
  label: string;
  phase: AcquisitionPhase;
  bordered?: boolean;
  children?: ReactNode;
}) {
  return (
    <Box
      display="grid"
      mih={68}
      style={{
        gridTemplateColumns: '20px auto minmax(0, 1fr) 64px',
        alignItems: 'center',
        columnGap: 12,
        borderTop: bordered ? '1px solid var(--mantine-color-dark-4)' : undefined,
      }}
    >
      <Box display="flex" style={{ justifyContent: 'center' }}>
        <Mark phase={phase} />
      </Box>
      <Text
        fw={phase === 'waiting' ? 500 : 600}
        c={phase === 'waiting' ? 'dimmed' : undefined}
        style={{ whiteSpace: 'nowrap' }}
      >
        {label}
      </Text>
      <Stack gap={0} ta="right" mih="2.9em" justify="center" miw={0}>
        {children}
      </Stack>
      <Badge color={colors[phase]} variant="light" fullWidth>
        {labels[phase]}
      </Badge>
    </Box>
  );
}

export function AcquisitionProgressModal({
  opened,
  progress,
  mode = 'initial',
}: {
  opened: boolean;
  progress: FoundryProgress;
  mode?: 'initial' | 'change' | 'refresh' | 'deployments' | 'delete';
}) {
  const modelsOnly = mode !== 'initial' && mode !== 'refresh';

  return (
    <Modal.Root
      opened={opened}
      onClose={() => {}}
      size={640}
      centered
      closeOnEscape={false}
      closeOnClickOutside={false}
    >
      <Modal.Overlay />
      <Modal.Content>
        <Modal.Header role="presentation">
          <Modal.Title>
            {mode === 'refresh'
              ? 'Refreshing Foundries'
              : mode === 'deployments'
                ? 'Refreshing models'
                : mode === 'delete'
                  ? 'Deleting deployment'
                  : 'Loading deployments'}
          </Modal.Title>
        </Modal.Header>
        <Modal.Body>
          {!modelsOnly && (
            <Step label="Foundries" phase={progress.foundryPhase as AcquisitionPhase}>
              {progress.foundryPhase === 'completed' && (
                <Text size="sm">{progress.foundryCount} Foundries</Text>
              )}
            </Step>
          )}
          <Step
            label={mode === 'delete' ? 'Delete' : 'Deployments'}
            phase={progress.modelPhase as AcquisitionPhase}
            bordered={!modelsOnly}
          >
            {progress.selectedFoundryName !== '' && (
              <>
                <Text size="sm" truncate="end" title={progress.selectedFoundryName}>
                  {progress.selectedFoundryName}
                </Text>
                {mode !== 'delete' && (
                  <Text size="sm" c="dimmed">
                    {progress.modelCount} models
                  </Text>
                )}
              </>
            )}
          </Step>
        </Modal.Body>
      </Modal.Content>
    </Modal.Root>
  );
}

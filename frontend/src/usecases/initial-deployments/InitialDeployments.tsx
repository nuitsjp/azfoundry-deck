import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import {
  Combobox,
  Input,
  InputBase,
  Stack,
  Table,
  Text,
  Tooltip,
  useCombobox,
} from '@mantine/core';
import { loadInitialView } from '../../features/foundry/initial-view';
import type { Foundry } from '../../features/foundry/models';
import { ErrorNotice } from '../../shared/ErrorNotice';
import { initialProgress } from '../../features/foundry/progress';
import { AcquisitionProgressModal } from './AcquisitionProgressModal';

function foundryLabel(foundry: Foundry) {
  return `${foundry.name}（${foundry.subscriptionName} - ${foundry.resourceGroupName}）`;
}

export function InitialDeployments() {
  const [progress, setProgress] = useState(initialProgress);
  const initial = useQuery({
    queryKey: ['foundry', 'initial-view'],
    queryFn: () => loadInitialView(setProgress),
    staleTime: Infinity,
    gcTime: Infinity,
    retry: false,
  });
  const combobox = useCombobox({ onDropdownClose: () => combobox.resetSelectedOption() });
  const view = initial.data;
  if (!view)
    return (
      <>
        <AcquisitionProgressModal opened={initial.isPending} progress={progress} />
        <ErrorNotice error={initial.error} />
      </>
    );
  const selected = view.foundries.find((foundry) => foundry.id === view.selectedFoundryId)!;
  const label = foundryLabel(selected);

  return (
    <Stack gap="lg">
      <Combobox store={combobox} onOptionSubmit={() => combobox.closeDropdown()} withinPortal>
        <Combobox.Target targetType="button" withExpandedAttribute>
          <InputBase
            component="button"
            type="button"
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
  );
}

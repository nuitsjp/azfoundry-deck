import { createFileRoute } from '@tanstack/react-router';
import { Title } from '@mantine/core';
import { useState } from 'react';
import { useMutationState, useQuery } from '@tanstack/react-query';
import { Phase } from '@bindings/azfoundrydeck/internal/azauth/models';
import { authStatus, changeTenantKey } from '../features/auth/queries';
import { ErrorNotice } from '../shared/ErrorNotice';
import { InitialDeployments } from '../usecases/initial-deployments/InitialDeployments';

function Home() {
  const status = useQuery(authStatus());
  const changes = useMutationState({
    filters: { mutationKey: changeTenantKey },
    select: (m) => m.state,
  });
  const latest = changes[changes.length - 1];
  const [closed, setClosed] = useState<number | null>(null);
  const failure = latest?.status === 'error' && latest.submittedAt !== closed;
  return (
    <>
      <Title order={2} mb="lg">
        Home
      </Title>
      {failure && (
        <ErrorNotice error={latest.error} onClose={() => setClosed(latest.submittedAt)} />
      )}
      {status.data?.phase === Phase.SignedIn && status.data.account?.selectedTenantId ? (
        <InitialDeployments />
      ) : null}
    </>
  );
}

export const Route = createFileRoute('/')({ component: Home });

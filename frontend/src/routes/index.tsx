import { createFileRoute } from '@tanstack/react-router';
import { Title } from '@mantine/core';
import { useQuery } from '@tanstack/react-query';
import { Phase } from '@bindings/azfoundrydeck/internal/azauth/models';
import { authStatus } from '../features/auth/queries';
import { InitialDeployments } from '../usecases/initial-deployments/InitialDeployments';

function Home() {
  const status = useQuery(authStatus());
  return (
    <>
      <Title order={2} mb="lg">
        Home
      </Title>
      {status.data?.phase === Phase.SignedIn && status.data.account?.selectedTenantId ? (
        <InitialDeployments />
      ) : null}
    </>
  );
}

export const Route = createFileRoute('/')({ component: Home });

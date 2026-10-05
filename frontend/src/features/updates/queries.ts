import { useEffect } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Events } from '@wailsio/runtime';
import * as Updates from '@bindings/azfoundrydeck/internal/updates/service';
const key = ['updates', 'status'] as const;
export function useUpdates() {
  const client = useQueryClient();
  const status = useQuery({ queryKey: key, queryFn: () => Updates.GetStatus() });
  useEffect(
    () => Events.On('updates:progress', (event) => client.setQueryData(key, event.data)),
    [client],
  );
  // The Go service quits the app once the installer has been handed off.
  const apply = useMutation({
    mutationKey: ['updates', 'apply'],
    mutationFn: () => Updates.Apply(),
  });
  return { status, apply };
}

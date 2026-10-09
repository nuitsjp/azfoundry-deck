import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query';
import * as Auth from '@bindings/azfoundrydeck/internal/azauth/service';

const statusKey = ['auth', 'status'] as const;
export const authStatus = () =>
  queryOptions({
    queryKey: statusKey,
    queryFn: () => Auth.GetStatus(),
  });
export function useLogin() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: () => Auth.Login(),
    onSuccess: (status) => {
      client.setQueryData(statusKey, status);
    },
    onError: () => client.invalidateQueries({ queryKey: statusKey }),
  });
}
export function useLogout() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: () => Auth.Logout(),
    onSuccess: (status) => {
      client.removeQueries({ queryKey: ['foundry'] });
      client.setQueryData(statusKey, status);
    },
  });
}

export function useSelectTenant() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (tenantId: string) => Auth.SelectTenant(tenantId),
    onSuccess: (status) => client.setQueryData(statusKey, status),
  });
}

export const changeTenantKey = ['tenant', 'change'] as const;
export function useChangeTenant() {
  const client = useQueryClient();
  return useMutation({
    mutationKey: changeTenantKey,
    mutationFn: (tenantId: string) => Auth.ChangeTenant(tenantId),
    onSuccess: (status) => {
      client.removeQueries({ queryKey: ['foundry'] });
      client.setQueryData(statusKey, status);
    },
  });
}

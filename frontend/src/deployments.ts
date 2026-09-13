import type { Deployment } from "../bindings/github.com/nuitsjp/azfoundry-deck/internal/service/models";

export type Filters = { tenantId: string; subscriptionId: string; region: string; accountId: string; model: string; name: string };
export const emptyFilters: Filters = { tenantId: "", subscriptionId: "", region: "", accountId: "", model: "", name: "" };

export function filterDeployments(rows: Deployment[], filters: Filters): Deployment[] {
  const name = filters.name.trim().toLocaleLowerCase();
  return rows.filter(row => (!filters.tenantId || row.tenantId === filters.tenantId)
    && (!filters.subscriptionId || row.subscriptionId === filters.subscriptionId)
    && (!filters.region || row.region === filters.region)
    && (!filters.accountId || row.accountId === filters.accountId)
    && (!filters.model || row.model === filters.model)
    && (!name || row.name.toLocaleLowerCase().includes(name)));
}

export function formatQuantity(value: number | null): string {
  return value === null ? "不明" : value.toLocaleString("ja-JP");
}

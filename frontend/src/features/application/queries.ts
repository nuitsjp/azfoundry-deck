import { queryOptions } from '@tanstack/react-query';
import * as Application from '@bindings/azfoundrydeck/internal/desktop/service';
export const appInfo = () =>
  queryOptions({
    queryKey: ['app', 'info'],
    queryFn: () => Application.GetInfo(),
    staleTime: Infinity,
  });
export function reportFrontendError(error: unknown) {
  const message =
    error instanceof Error ? `${error.name}: ${error.message}` : 'Unhandled frontend error';
  void Application.ReportFrontendError(message.slice(0, 2000)).catch(() =>
    console.error('診断情報をGoへ送信できませんでした。'),
  );
}

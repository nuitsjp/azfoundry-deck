import type { Deployment, ModelCandidate, ModelResult } from "../bindings/github.com/nuitsjp/azfoundry-deck/internal/service/models";

type ModelCandidatesProps = {
  account: Deployment;
  result: ModelResult | null;
  loading: boolean;
  error: string | null;
  search: string;
  onSearchChange: (value: string) => void;
};

function displayValue(value: string): string {
  return value || "不明";
}

function matchesSearch(candidate: ModelCandidate, search: string): boolean {
  const term = search.trim().toLocaleLowerCase();
  if (!term) return true;
  return [candidate.name, candidate.format, candidate.version]
    .some(value => value.toLocaleLowerCase().includes(term));
}

function modelFailureHeading(error: string | null, failures: number): string {
  if (error) return "モデル候補の取得に失敗しました";
  return `${failures}件の取得エラーがあります`;
}

export default function ModelCandidates({ account, result, loading, error, search, onSearchChange }: ModelCandidatesProps) {
  const models = result?.models ?? [];
  const visible = models.filter(candidate => matchesSearch(candidate, search));
  const failures = result?.failures ?? [];
  const errorCount = failures.length + (error ? 1 : 0);
  const hasRows = visible.length > 0;

  return (
    <section className="model-candidates" aria-labelledby="models-heading" aria-busy={loading}>
      <div className="model-target">
        <div>
          <span className="eyebrow">選択中のアカウント</span>
          <strong>{account.accountName}</strong>
        </div>
        <span>{account.subscriptionName} · {account.region} · {account.accountId}</span>
      </div>

      <div className={`fetch-status model-fetch-status ${errorCount > 0 ? "warning" : ""}`} role="status">
        {loading ? <div className="fetch-progress"><div className="progress-heading"><span className="spinner" aria-hidden="true" /><strong>モデル候補を取得中…</strong><span>{result ? "前回の結果を表示しています。" : "対象アカウントを読み取っています。"}</span></div></div> : errorCount === 0 && result ? <><span className="status-dot" /><strong>モデル候補の取得完了</strong><span>{`${result.models.length}モデルバージョン`}</span></> : null}
        {errorCount > 0 ? <div className="error-summary"><span className="status-dot amber" /><strong>{modelFailureHeading(error, failures.length)}</strong></div> : null}
      </div>

      <div className="section-heading table-heading model-table-heading">
        <h2 id="models-heading">モデル候補 <span className="result-count">{visible.length}<small>{search.trim() ? ` / ${models.length}件` : "件"}</small></span></h2>
        <label className="model-search">モデル / フォーマット / バージョン<input aria-label="モデル候補を検索" type="search" placeholder="候補を検索" value={search} onChange={event => onSearchChange(event.target.value)} /></label>
      </div>

      {errorCount > 0 ? <div className="model-error-list" aria-label="モデル候補の取得エラー">
        {error ? <article className="error-item"><div><strong>取得処理</strong></div><div><p>{error}</p><p className="error-action">更新して再試行してください。</p></div></article> : null}
        {failures.map((failure, index) => <article className="error-item" key={`${failure.scope}-${failure.code}-${index}`}><div><strong>{failure.accountName || failure.scope}</strong><span>{[failure.tenantName, failure.subscriptionName].filter(Boolean).join(" / ")}</span></div><div><p><code>{failure.code}</code> {failure.message}</p><p className="error-action">{failure.action}</p></div></article>)}
      </div> : null}

      {hasRows ? <div className="table-scroll model-table-scroll" tabIndex={0} aria-label="モデル候補一覧のスクロール領域"><table className="model-candidates-table">
        <thead><tr><th scope="col">モデル</th><th scope="col">フォーマット</th><th scope="col">バージョン</th><th scope="col">ライフサイクル</th><th scope="col">既定バージョン</th><th scope="col">利用可能SKU</th></tr></thead>
        <tbody>{visible.map((candidate, index) => <tr key={`${candidate.name}-${candidate.version}-${index}`}>
          <td><strong className="model-name">{displayValue(candidate.name)}</strong></td>
          <td>{displayValue(candidate.format)}</td>
          <td>{displayValue(candidate.version)}</td>
          <td><span className="model-lifecycle">{displayValue(candidate.lifecycle)}</span></td>
          <td>{candidate.isDefaultVersion ? <span className="model-default">既定</span> : <span className="model-unknown">—</span>}</td>
          <td><div className="model-sku-list">{candidate.skus.length > 0 ? candidate.skus.map((sku, skuIndex) => <span className="sku model-sku" key={`${sku.name}-${skuIndex}`}>{sku.name}</span>) : <span className="model-unknown">不明</span>}</div></td>
        </tr>)}</tbody>
      </table></div> : <div className="empty-state model-empty-state"><span className="empty-icon" aria-hidden="true">{loading ? "…" : errorCount > 0 ? "!" : "≡"}</span><h3>{loading ? "モデル候補を取得しています" : errorCount > 0 ? "モデル候補を表示できません" : result && models.length > 0 ? "条件に一致するモデル候補がありません" : "モデル候補はありません"}</h3><p>{loading ? "完了するまでお待ちください。" : errorCount > 0 ? "エラーの原因を確認してから更新してください。" : result && models.length > 0 ? "検索条件を変更してください。" : "対象アカウントで利用可能なモデル候補はありません。"}</p></div>}
    </section>
  );
}

import { useCallback, useEffect, useRef, useState } from "react";
import { GetDeployments, GetEnvironment } from "../bindings/github.com/nuitsjp/azfoundry-deck/internal/service/deploymentservice";
import { GetModels } from "../bindings/github.com/nuitsjp/azfoundry-deck/internal/service/modelservice";
import { GetModelScenario, GetScenario, SetModelScenario, SetScenario } from "../bindings/github.com/nuitsjp/azfoundry-deck/internal/mock/mockservice";
import { Events } from "@wailsio/runtime";
import "../bindings/github.com/wailsapp/wails/v3/internal/eventcreate";
import type { Deployment, DeploymentProgress, DeploymentResult, ModelResult } from "../bindings/github.com/nuitsjp/azfoundry-deck/internal/service/models";
import { emptyFilters, filterDeployments, formatQuantity, type Filters } from "./deployments";
import ModelCandidates from "./ModelCandidates";
import AddModel from "./AddModel";
import { addModelMock, loadMockModels } from "./addModelMock";

const scenarios = [
  ["success", "通常 · 複数テナント / 10アカウント"],
  ["empty", "0件 · 全アカウント取得成功"],
  ["partial", "部分失敗 · 4アカウントで取得エラー"],
  ["failure", "全体失敗 · 未ログイン"],
  ["cli-missing", "全体失敗 · Azure CLI未導入"],
  ["all-accounts-failed", "全アカウントで取得失敗"],
  ["loading", "読込中 · 30秒かけて順次取得"],
  ["delayed", "遅延応答 · 8秒後に古い設定値"],
];

const modelScenarios = [
  ["success", "通常 · 6モデルバージョン"],
  ["empty", "0件 · 候補なし"],
  ["failure", "取得失敗 · 権限不足"],
  ["loading", "読込中 · 30秒待機"],
  ["delayed", "遅延応答 · 2秒待機"],
];

type View = "deployments" | "models";

export default function App() {
  const [addingModel, setAddingModel] = useState(false);
  const [result, setResult] = useState<DeploymentResult | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [isMock, setIsMock] = useState(false);
  const [ready, setReady] = useState(false);
  const [scenario, setScenario] = useState("success");
  const [configuring, setConfiguring] = useState(false);
  const [scenarioError, setScenarioError] = useState<string | null>(null);
  const [filters, setFilters] = useState<Filters>(emptyFilters);
  const [showErrors, setShowErrors] = useState(false);
  const [view, setView] = useState<View>("deployments");
  const [selectedAccount, setSelectedAccount] = useState<Deployment | null>(null);
  const [modelResult, setModelResult] = useState<ModelResult | null>(null);
  const [modelLoading, setModelLoading] = useState(false);
  const [modelError, setModelError] = useState<string | null>(null);
  const [modelScenario, setModelScenario] = useState("success");
  const [modelScenarioError, setModelScenarioError] = useState<string | null>(null);
  const [modelSearch, setModelSearch] = useState("");
  const pageHeading = useRef<HTMLHeadingElement>(null);
  const request = useRef({ id: "", sequence: 0 });
  const modelRequest = useRef({ id: "" });
  const [progress, setProgress] = useState<DeploymentProgress | null>(null);

  useEffect(() => { pageHeading.current?.focus(); }, [showErrors, view]);

  useEffect(() => {
    const unsubscribe = Events.On("deployments:progress", event => {
      const next = event.data;
      if (next.requestId !== request.current.id || next.sequence <= request.current.sequence) return;
      request.current.sequence = next.sequence;
      setProgress(next);
    });
    return () => { unsubscribe(); request.current.id = ""; };
  }, []);

  const refresh = useCallback(async () => {
    const current = crypto.randomUUID();
    request.current = { id: current, sequence: 0 };
    setProgress(null);
    setLoading(true);
    setError(null);
    try {
      const next = await GetDeployments(current);
      if (current === request.current.id) setResult(next);
    } catch (cause) {
      if (current === request.current.id) {
        setResult(null);
        setError(String(cause));
      }
    } finally {
      if (current === request.current.id) {
        request.current.id = "";
        setProgress(null);
        setLoading(false);
      }
    }
  }, []);

  const refreshModels = useCallback(async (accountId: string) => {
    const current = crypto.randomUUID();
    modelRequest.current = { id: current };
    setModelLoading(true);
    setModelError(null);
    try {
      const next = await GetModels(accountId);
      if (current === modelRequest.current.id) setModelResult(next);
    } catch (cause) {
      if (current === modelRequest.current.id) setModelError(String(cause));
    } finally {
      if (current === modelRequest.current.id) {
        modelRequest.current.id = "";
        setModelLoading(false);
      }
    }
  }, []);

  useEffect(() => {
    let active = true;
    void GetEnvironment().then(async environment => {
      if (!active) return;
      setIsMock(environment.mock);
      if (environment.mock) {
        const [selected, selectedModel] = await Promise.all([GetScenario(), GetModelScenario()]);
        if (!active) return;
        setScenario(selected);
        setModelScenario(selectedModel);
      }
      setReady(true);
      void refresh();
    }).catch(cause => {
      if (!active) return;
      setError(String(cause));
      setLoading(false);
    });
    return () => { active = false; request.current.id = ""; modelRequest.current.id = ""; };
  }, [refresh]);

  async function changeScenario(value: string) {
    setConfiguring(true);
    setScenarioError(null);
    try {
      await SetScenario(value);
      setScenario(value);
    } catch (cause) {
      setScenarioError(String(cause));
    } finally {
      setConfiguring(false);
    }
  }

  async function changeModelScenario(value: string) {
    setConfiguring(true);
    setModelScenarioError(null);
    try {
      await SetModelScenario(value);
      setModelScenario(value);
    } catch (cause) {
      setModelScenarioError(String(cause));
    } finally {
      setConfiguring(false);
    }
  }

  function openModels(row: Deployment) {
    modelRequest.current.id = "";
    setSelectedAccount(row);
    setModelResult(null);
    setModelError(null);
    setModelSearch("");
    setShowErrors(false);
    setView("models");
    void refreshModels(row.accountId);
  }

  function closeModels() {
    modelRequest.current.id = "";
    setModelLoading(false);
    setModelError(null);
    setView("deployments");
  }

  function refreshCurrentView() {
    if (view === "models" && selectedAccount) {
      void refreshModels(selectedAccount.accountId);
      return;
    }
    void refresh();
  }

  const hasCurrentResult = progress !== null && progress.completedAccounts > 0;
  const displayedResult = hasCurrentResult ? progress.result : result;
  const rows = displayedResult?.deployments ?? [];
  const visible = filterDeployments(rows, filters);
  const failures = displayedResult?.failures ?? [];
  const incomplete = failures.length > 0;
  const errorCount = failures.length + (error ? 1 : 0);
  const failureSummary = error ? "取得に失敗しました" : displayedResult?.totalAccounts === null ? "対象の探索に失敗しました" : `${failures.length}アカウントで取得失敗`;
  const discovering = !progress || progress.stage === "discovering";
  const progressCompleted = discovering ? progress?.completedSubscriptions ?? 0 : progress.completedAccounts;
  const progressTotal = discovering ? progress?.totalSubscriptions ?? null : progress.result.totalAccounts;
  const activeFilters = Object.values(filters).filter(Boolean).length;
  const setFilter = (key: keyof Filters, value: string) => setFilters(previous => ({ ...previous, [key]: value }));
  const selectFields = [
    { key: "tenantId", label: "テナント", name: "tenantName" },
    { key: "subscriptionId", label: "サブスクリプション", name: "subscriptionName" },
    { key: "region", label: "リージョン", name: "region" },
    { key: "accountId", label: "アカウント", name: "accountName" },
    { key: "model", label: "モデル", name: "model" },
  ] as const;

  return (
    <main>
      <header className="app-header">
        <div className="brand"><span className="brand-mark" aria-hidden="true">A<span>F</span></span><div><p className="eyebrow">AZFOUNDRY DECK</p><h1 ref={pageHeading} tabIndex={-1}>{view === "models" ? "モデル候補" : showErrors ? "エラー詳細" : "デプロイ一覧"}</h1></div></div>
        <div className="header-actions">{view === "models" ? <button className="back-button" onClick={closeModels}>デプロイ一覧に戻る</button> : showErrors ? <button className="back-button" onClick={() => setShowErrors(false)}>デプロイ一覧に戻る</button> : null}<div className="update-meta">{view === "models" && selectedAccount ? <><span>{selectedAccount.accountName} のモデル候補</span><span>{modelResult ? `最終取得 ${new Date(modelResult.fetchedAt).toLocaleTimeString("ja-JP")}` : "まだ取得が完了していません"}</span></> : <><span>{result ? `最終取得 ${new Date(result.fetchedAt).toLocaleTimeString("ja-JP")}` : "まだ取得が完了していません"}</span></>}</div><button className="primary" onClick={refreshCurrentView} disabled={!ready || configuring}><span aria-hidden="true">↻</span> {view === "models" && modelLoading || loading ? "再取得" : "更新"}</button></div>
      </header>

      {isMock ? <aside className="mock-bar" aria-label="モック設定"><span className="mock-badge">MOCK</span><span>架空データ · Azure接続なし</span><label htmlFor="scenario">次回の取得状態</label><select id="scenario" value={scenario} disabled={configuring} onChange={event => void changeScenario(event.target.value)}>{scenarios.map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select>{view === "models" ? <><label htmlFor="model-scenario">モデル候補の次回の取得状態</label><select id="model-scenario" value={modelScenario} disabled={configuring} onChange={event => void changeModelScenario(event.target.value)}>{modelScenarios.map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></> : null}<span className="mock-hint">選択後に「{loading || modelLoading ? "再取得" : "更新"}」</span>{scenarioError ? <p role="alert">状態を変更できませんでした: {scenarioError}</p> : null}{modelScenarioError ? <p role="alert">モデル候補の状態を変更できませんでした: {modelScenarioError}</p> : null}</aside> : null}

      {view === "models" && selectedAccount ? <ModelCandidates account={selectedAccount} result={modelResult} loading={modelLoading} error={modelError} search={modelSearch} onSearchChange={setModelSearch} /> : <>
        {!showErrors ? <section className="filter-panel" aria-labelledby="filter-heading">
          <div className="section-heading"><h2 id="filter-heading">絞り込み {activeFilters ? <span className="count-badge">{activeFilters}</span> : null}</h2><button className="text-button" disabled={!activeFilters} onClick={() => setFilters(emptyFilters)}>すべて解除</button></div>
          <div className="filter-grid">
            {selectFields.map(field => {
              const candidates = filterDeployments(rows, { ...filters, [field.key]: "" });
              const options = new Map(candidates.map(row => [row[field.key], row[field.name]]));
              return <label key={field.key}>{field.label}<select value={filters[field.key]} onChange={event => setFilter(field.key, event.target.value)}><option value="">すべて</option>{filters[field.key] && !options.has(filters[field.key]) ? <option value={filters[field.key]} disabled>選択中（現在の条件に該当なし）</option> : null}{[...options].sort((a, b) => a[1].localeCompare(b[1])).map(([id, name]) => <option key={id} value={id}>{name}{[...options.values()].filter(value => value === name).length > 1 ? ` (${id})` : ""}</option>)}</select></label>;
            })}
            <label>デプロイ名<input type="search" placeholder="名前の一部で検索" value={filters.name} onChange={event => setFilter("name", event.target.value)} /></label>
          </div>

        </section> : null}

        <div className={`fetch-status ${incomplete || error ? "warning" : ""}`} role="status">
          {loading ? <div className="fetch-progress"><div className="progress-heading"><span className="spinner" aria-hidden="true" /><strong>取得中…</strong><span>{discovering ? "対象を探索 · サブスクリプション" : "デプロイを取得 · アカウント"} {progressCompleted}{progressTotal === null ? "件完了（総数を確認中）" : ` / ${progressTotal} 完了`}</span><progress aria-label="取得の進捗" max={progressTotal || 1} value={progressTotal === null ? undefined : progressCompleted} /></div>{displayedResult ? <span className="progress-note">{hasCurrentResult ? "途中結果" : "前回の結果"}</span> : null}</div> : errorCount === 0 && result ? <><span className="status-dot" /><strong>全対象の取得完了</strong><span>{`${result.successfulAccounts} / ${result.totalAccounts} アカウント取得成功`}</span></> : null}
          {errorCount > 0 ? <div className="error-summary"><span className="status-dot amber" /><strong>{loading && !hasCurrentResult ? "前回: " : ""}{failureSummary}（一覧は不完全）</strong>{!showErrors ? <button className="text-button" onClick={() => setShowErrors(true)}>エラー詳細</button> : null}</div> : null}
        </div>

        {showErrors ? <section className="errors" aria-labelledby="error-heading" aria-busy={loading}>
          <div className="section-heading"><h2 id="error-heading">取得エラー <span className="count-badge">{errorCount}</span></h2></div>
          {errorCount > 0 ? <div className="error-list" tabIndex={0} aria-label="取得エラーのスクロール領域">
            {error ? <article className="error-item"><div><strong>取得処理</strong></div><div><p>{error}</p><p className="error-action">アプリの起動状態を確認して、更新してください。</p></div></article> : null}
            {failures.map((failure, index) => <article className="error-item" key={`${failure.scope}-${failure.code}-${index}`}><div><strong>{failure.accountName || failure.scope}</strong><span>{[failure.tenantName, failure.subscriptionName].filter(Boolean).join(" / ")}</span></div><div><p><code>{failure.code}</code> {failure.message}</p><p className="error-action">{failure.action}</p></div></article>)}
          </div> : <div className="empty-state"><h3>{loading ? "デプロイを取得しています" : "取得エラーはありません"}</h3><p>{loading ? "完了するまでお待ちください。" : "デプロイ一覧に戻って、取得結果を確認できます。"}</p></div>}
        </section> : <section className="deployments" aria-labelledby="deployments-heading" aria-busy={loading}>
          <div className="section-heading table-heading"><div className="deployment-list-actions"><h2 id="deployments-heading">デプロイ <span className="result-count">{visible.length}<small>{activeFilters ? ` / ${rows.length}件` : "件"}</small></span></h2>{isMock ? <button className="primary" onClick={() => setAddingModel(true)}>＋ 新規追加</button> : null}</div></div>
          {visible.length ? <div className="table-scroll" tabIndex={0} aria-label="デプロイ一覧のスクロール領域"><table>
              <thead><tr>
                <th scope="col">テナント</th>
                <th scope="col">サブスクリプション</th>
                <th scope="col">リージョン</th>
                <th scope="col">アカウント</th>
                <th scope="col">モデル / バージョン</th>
                <th scope="col">デプロイ名</th>
                <th scope="col">SKU</th>
                <th scope="col" className="numeric">設定 TPM</th>
                <th scope="col" className="numeric">設定 RPM</th>
              </tr></thead>
              <tbody>{visible.map(row => <tr key={row.id}>
                <td><span title={row.tenantId}>{row.tenantName}</span></td>
                <td><span title={row.subscriptionId}>{row.subscriptionName}</span></td>
                <td>{row.region}</td>
                <td><div className="account-cell"><span title={row.accountId}>{row.accountName}</span><button className="text-button model-candidates-button" aria-label={`モデル候補 ${row.accountName}`} onClick={() => openModels(row)}>モデル候補</button></div></td>
                <td>{row.model}<span className="secondary">{row.modelVersion}</span></td>
                <td className="deployment-name"><strong title={row.id}>{row.name}</strong></td>
                <td><span className="sku">{row.sku}</span></td>
                <td className={`numeric tpm ${row.tpm === null ? "unknown" : ""}`}>{formatQuantity(row.tpm)}</td>
                <td className={`numeric ${row.rpm === null ? "unknown" : ""}`}>{formatQuantity(row.rpm)}</td>
              </tr>)}</tbody>
            </table></div> : <div className="empty-state"><span className="empty-icon" aria-hidden="true">{loading ? "…" : incomplete || error ? "!" : "≡"}</span><h3>{loading ? "デプロイを取得しています" : error || incomplete && !rows.length ? "取得失敗のため表示できるデプロイがありません" : !result ? "取得結果がありません" : rows.length ? "条件に一致するデプロイがありません" : "デプロイはありません"}</h3><p>{loading ? "完了するまでお待ちください。絞り込み条件は取得中も変更できます。" : error || incomplete && !rows.length ? "「エラー詳細」で原因を確認して、復旧後に更新してください。" : rows.length ? "絞り込み条件を変更するか、すべて解除してください。" : result ? "全対象アカウントの取得に成功し、0件でした。" : "取得状態を確認してください。"}</p></div>}
        </section>}
      </>}
      {isMock && addingModel ? <AddModel catalog={addModelMock} loadModels={loadMockModels} onClose={() => setAddingModel(false)} /> : null}

    </main>
  );
}

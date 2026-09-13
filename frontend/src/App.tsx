import { useState } from "react";
import { GetDeploymentSample } from "../bindings/github.com/nuitsjp/azfoundry-deck/internal/service/compatibilityservice";
import type { DeploymentSample } from "../bindings/github.com/nuitsjp/azfoundry-deck/internal/service/models";

export default function App() {
  const [sample, setSample] = useState<DeploymentSample | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function loadSample() {
    setLoading(true);
    setError(null);
    setSample(null);
    try {
      setSample(await GetDeploymentSample());
    } catch (cause) {
      setError(String(cause));
    } finally {
      setLoading(false);
    }
  }

  return (
    <main>
      <header>
        <p className="badge">MOCK · 架空データ</p>
        <h1>AzFoundry Deck</h1>
        <p>互換性確認用の最小画面です。Azure への接続は行いません。</p>
      </header>
      <section aria-labelledby="sample-heading">
        <h2 id="sample-heading">デプロイの表示確認</h2>
        <button disabled={loading} onClick={loadSample}>
          {loading ? "取得中…" : "固定データを取得"}
        </button>
        {error !== null ? <p role="alert">{error}</p> : null}
        <div aria-live="polite">
          {sample !== null ? (
            <table>
              <thead>
                <tr><th>名前</th><th>モデル</th><th>バージョン</th><th>SKU</th><th>Capacity</th><th>状態</th></tr>
              </thead>
              <tbody>
                <tr>
                  <td>{sample.name}</td><td>{sample.model}</td><td>{sample.modelVersion}</td>
                  <td>{sample.sku}</td><td>{sample.capacity ?? "不明"}</td><td>{sample.state}</td>
                </tr>
              </tbody>
            </table>
          ) : loading ? null : <p>「固定データを取得」で表示を確認できます。</p>}
        </div>
      </section>
    </main>
  );
}


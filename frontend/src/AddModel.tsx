import { useEffect, useRef, useState } from "react";
import type { addModelMock, ModelLoadScenario } from "./addModelMock";

type Props = { catalog: typeof addModelMock; loadModels: (foundry: string, scenario: ModelLoadScenario) => Promise<typeof addModelMock.models>; onClose: () => void };
type NewResource = { name: string; region: string };

export default function AddModel({ catalog, loadModels, onClose }: Props) {
  const dialog = useRef<HTMLDialogElement>(null);
  const heading = useRef<HTMLHeadingElement>(null);
  const [screen, setScreen] = useState<"model" | "foundry" | "group" | "review" | "done">("model");
  const [groupReturn, setGroupReturn] = useState<"model" | "foundry">("model");
  const [subscription, setSubscription] = useState("");
  const [group, setGroup] = useState("");
  const [foundry, setFoundry] = useState("");
  const [newGroup, setNewGroup] = useState<NewResource | null>(null);
  const [newFoundry, setNewFoundry] = useState<NewResource | null>(null);
  const [groupName, setGroupName] = useState("");
  const [groupRegion, setGroupRegion] = useState("japaneast");
  const [foundryName, setFoundryName] = useState("");
  const [foundryRegion, setFoundryRegion] = useState("japaneast");
  const modelRequest = useRef(0);
  const [models, setModels] = useState<typeof addModelMock.models>([]);
  const [modelStatus, setModelStatus] = useState<"idle" | "loading" | "ready" | "error">("idle");
  const [modelError, setModelError] = useState("");
  const [loadScenario, setLoadScenario] = useState<ModelLoadScenario>("success");
  useEffect(() => () => { modelRequest.current++; }, []);
  const [model, setModel] = useState("");
  const [version, setVersion] = useState("");
  const [sku, setSku] = useState("");
  const [name, setName] = useState("");
  const [capacity, setCapacity] = useState("10");
  useEffect(() => { dialog.current?.showModal(); }, []);
  useEffect(() => { heading.current?.focus(); }, [screen]);
  const groups = catalog.groups.filter(item => item.subscription === subscription);
  const foundries = catalog.foundries.filter(item => item.subscription === subscription && item.group === group);
  const selectedModel = models.find(item => item.name === model);
  const selectedSubscription = catalog.subscriptions.find(item => item.id === subscription);
  const region = newFoundry?.name === foundry ? newFoundry.region : foundries.find(item => item.name === foundry)?.region;
  const titles = { model: "モデルを追加する", foundry: "Foundryを追加する", group: "リソースグループを追加する", review: "追加内容の確認", done: "モデル追加の再現が完了しました" };
  const foundryRule = "2〜64文字の半角英数字・ハイフン。先頭と末尾は英数字。";
  const groupRule = "1〜90文字の文字・数字・_ - . ( )。末尾のピリオドと空白は使用できません。";
  const foundryError = foundryName && (!/^[a-zA-Z0-9][a-zA-Z0-9-]{0,62}[a-zA-Z0-9]$/.test(foundryName) ? foundryRule : foundries.some(item => item.name.toLowerCase() === foundryName.toLowerCase()) ? "同じリソースグループに同名のFoundryがあります。" : "");
  const groupError = groupName && (!/^[\p{L}\p{Nd}_().-]{1,90}$/u.test(groupName) || groupName.endsWith(".") ? groupRule : groups.some(item => item.name.toLowerCase() === groupName.toLowerCase()) ? "同じサブスクリプションに同名のリソースグループがあります。" : "");
  function resetModel() {
    modelRequest.current++;
    setModels([]); setModelStatus("idle"); setModelError("");
    setModel(""); setVersion(""); setSku(""); setName("");
  }
  async function fetchModels(target: string) {
    resetModel();
    if (!target) return;
    const request = modelRequest.current;
    setModelStatus("loading");
    try {
      const candidates = await loadModels(target, loadScenario);
      if (request !== modelRequest.current) return;
      setModels(candidates);
      setModelStatus("ready");
    } catch (cause) {
      if (request !== modelRequest.current) return;
      setModelError(String(cause instanceof Error ? cause.message : cause));
      setModelStatus("error");
    }
  }
  function chooseFoundry(value: string) {
    setFoundry(value);
    void fetchModels(value);
  }
  function chooseModel(value: string) {
    const candidate = models.find(item => item.name === value);
    setModel(value);
    setVersion(candidate?.defaultVersion ?? "");
    setSku(candidate?.defaultSku ?? "");
    setName(value);
  }
  function changeGroup(value: string) { setGroup(value); setFoundry(""); setNewFoundry(null); resetModel(); }
  function openGroup(from: "model" | "foundry") { setGroupReturn(from); setGroupName(""); setScreen("group"); }
  const groupPicker = <div className="add-field"><label htmlFor="add-group">リソースグループ</label><div className="add-select-action"><select id="add-group" required value={group} disabled={!subscription} onChange={event => changeGroup(event.target.value)}><option value="">選択してください</option>{groups.map(item => <option key={item.name}>{item.name}</option>)}{newGroup ? <option value={newGroup.name}>{newGroup.name}（新規作成予定）</option> : null}</select><button type="button" className="back-button" disabled={!subscription} onClick={() => openGroup(screen === "foundry" ? "foundry" : "model")}>リソースグループを新規作成</button></div></div>;
  return <dialog ref={dialog} className="add-dialog" aria-labelledby="add-title" onCancel={onClose}>
    <header className="add-heading"><div><h2 id="add-title" ref={heading} tabIndex={-1}>{titles[screen]}</h2></div><button className="back-button" onClick={onClose} aria-label="モデル追加を閉じる">閉じる</button></header>
    <p className="add-notice">MOCK · 作成・保存なし</p>
    {screen === "model" ? <form onSubmit={event => { event.preventDefault(); setScreen("review"); }}>
      <div className="add-section">
        <label className="add-field">サブスクリプション<select aria-label="サブスクリプション" required value={subscription} onChange={event => { setSubscription(event.target.value); changeGroup(""); setNewGroup(null); }}><option value="">選択してください</option>{catalog.subscriptions.map(item => <option key={item.id} value={item.id}>{item.name} — {item.tenant}</option>)}</select></label>
        {groupPicker}
        <div className="add-field"><label htmlFor="add-foundry">Foundry <small className="region-label">{region}</small></label><div className="add-select-action"><select id="add-foundry" aria-label="Foundry" required disabled={!group} value={foundry} onChange={event => chooseFoundry(event.target.value)}><option value="">{group && !foundries.length && !newFoundry ? "Foundryがありません。新規作成してください" : "選択してください"}</option>{foundries.map(item => <option key={item.name} value={item.name}>{item.name}{item.empty ? "（デプロイ0件）" : ""}</option>)}{newFoundry ? <option value={newFoundry.name}>{newFoundry.name}（新規作成予定）</option> : null}</select><button type="button" className="back-button" disabled={!subscription} onClick={() => { setFoundryName(""); setScreen("foundry"); }}>Foundryを新規作成</button></div></div>

      </div>
      <div className="add-model-load" aria-live="polite" aria-busy={modelStatus === "loading"}>
        {modelStatus === "loading" ? <><span className="spinner" aria-hidden="true" /> <span>{foundry} · 読み込み中…</span><progress aria-label="モデル候補の読込状況" /></> : modelStatus === "error" ? <><span className="add-error">{modelError}</span><button type="button" className="text-button" onClick={() => void fetchModels(foundry)}>モデル候補を再試行</button></> : modelStatus === "ready" ? <span>{models.length ? `${models.length}モデル` : "モデル候補なし"}</span> : <span />}
      </div>

      <fieldset className="add-section" disabled={modelStatus !== "ready" || !models.length}><legend className="sr-only">モデルとデプロイ設定</legend>
        <div className="add-grid"><label className="add-field">モデル<select aria-label="モデル" required value={model} onChange={event => chooseModel(event.target.value)}><option value="">選択してください</option>{models.map(item => <option key={item.name}>{item.name}</option>)}</select></label><label className="add-field">バージョン<select aria-label="バージョン" required value={version} onChange={event => setVersion(event.target.value)}><option value="">選択してください</option>{selectedModel?.versions.map(item => <option key={item}>{item}</option>)}</select></label><label className="add-field">SKU<select aria-label="SKU" required value={sku} onChange={event => setSku(event.target.value)}><option value="">選択してください</option>{selectedModel?.skus.map(item => <option key={item}>{item}</option>)}</select></label><label className="add-field">Capacity<input type="number" required min="1" step="1" value={capacity} onChange={event => setCapacity(event.target.value)} /></label></div>
        <label className="add-field">デプロイ名<input required value={name} pattern=".*\S.*" onChange={event => setName(event.target.value)} placeholder="例: chat-production" /></label>
      </fieldset>      <details className="add-load-controls"><summary>モック：モデル取得の再現</summary><label>モデル取得の状態<select aria-label="モデル取得の状態" value={loadScenario} onChange={event => setLoadScenario(event.target.value as ModelLoadScenario)}><option value="success">通常（約0.8秒）</option><option value="slow">時間がかかる（約8秒）</option><option value="empty">候補なし</option><option value="failure">取得失敗</option></select></label><button type="button" className="text-button" disabled={!foundry} onClick={() => void fetchModels(foundry)}>モデル候補を再取得</button></details><div className="add-actions"><button type="button" className="back-button" onClick={onClose}>キャンセル</button><button className="primary" disabled={modelStatus !== "ready" || !models.length}>追加内容を確認</button></div>
    </form> : null}
    {screen === "foundry" ? <form onSubmit={event => { event.preventDefault(); if (!foundryName || foundryError) return; setNewFoundry({ name: foundryName.trim(), region: foundryRegion }); chooseFoundry(foundryName.trim()); setScreen("model"); }}><p className="add-context">{selectedSubscription?.name}</p>{groupPicker}<label className="add-field">新しいFoundry名<input required inputMode="url" autoCapitalize="none" autoCorrect="off" spellCheck={false} aria-describedby="foundry-validation" aria-invalid={Boolean(foundryError)} value={foundryName} onChange={event => setFoundryName(event.target.value)} /></label><p id="foundry-validation" className={foundryError ? "add-validation add-error" : "add-validation"} aria-live="polite">{foundryError || foundryRule}</p><label className="add-field">Foundryのリージョン<select aria-label="Foundryのリージョン" value={foundryRegion} onChange={event => setFoundryRegion(event.target.value)}><option>japaneast</option><option>eastus</option><option>swedencentral</option></select></label><div className="add-actions"><button type="button" className="back-button" onClick={() => setScreen("model")}>モデル追加に戻る</button><button className="primary" disabled={!foundryName || Boolean(foundryError)}>このFoundryを使用</button></div></form> : null}
    {screen === "group" ? <form onSubmit={event => { event.preventDefault(); if (!groupName || groupError) return; setNewGroup({ name: groupName.trim(), region: groupRegion }); changeGroup(groupName.trim()); setScreen(groupReturn); }}><p className="add-context">{selectedSubscription?.name}</p><label className="add-field">新しいリソースグループ名<input required inputMode="url" autoCapitalize="none" autoCorrect="off" spellCheck={false} aria-describedby="group-validation" aria-invalid={Boolean(groupError)} value={groupName} onChange={event => setGroupName(event.target.value)} /></label><p id="group-validation" className={groupError ? "add-validation add-error" : "add-validation"} aria-live="polite">{groupError || groupRule}</p><label className="add-field">リソースグループのリージョン<select aria-label="リソースグループのリージョン" value={groupRegion} onChange={event => setGroupRegion(event.target.value)}><option>japaneast</option><option>eastus</option><option>swedencentral</option></select></label><div className="add-actions"><button type="button" className="back-button" onClick={() => setScreen(groupReturn)}>戻る</button><button className="primary" disabled={!groupName || Boolean(groupError)}>このリソースグループを使用</button></div></form> : null}
    {screen === "review" || screen === "done" ? <><dl className="add-summary">{[
      ["サブスクリプション", selectedSubscription?.name], ["リソースグループ", `${group}${newGroup?.name === group ? `（新規 / ${newGroup.region}）` : "（既存）"}`], ["Foundry", `${foundry}（${newFoundry?.name === foundry ? "新規" : "既存"} / ${region}）`], ["モデル / バージョン", `${model} / ${version}`], ["SKU / Capacity", `${sku} / ${capacity}`], ["デプロイ名", name],
    ].map(([label, value]) => <div key={label}><dt>{label}</dt><dd>{value}</dd></div>)}</dl><p>{screen === "done" ? "追加完了の表示までを再現しました。一覧への保存は行っていません。" : "新規指定したリソースグループ、Foundry、モデルの順に作成する想定です。"}</p><div className="add-actions">{screen === "review" ? <><button className="back-button" onClick={() => setScreen("model")}>入力内容を変更</button><button className="primary" onClick={() => setScreen("done")}>モデル追加を再現</button></> : <button className="primary" onClick={onClose}>一覧に戻る</button>}</div></> : null}
  </dialog>;
}

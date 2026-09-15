import { useEffect, useRef, useState } from "react";
import type {
  AddModelCatalog,
  CapacityContract,
  CreateResultScenario,
  ModelCandidate,
  ModelLoadScenario,
  ModelSku,
  ModelVersion,
} from "./addModelSource";

// ModelTarget is the Foundry the candidates are read for. An existing Foundry is
// read by its resource ID; a Foundry that has not been created yet is read by
// subscription and region, and that result stays provisional.
export type ModelTarget = { foundryId: string; subscription: string; region: string };

export type CreateRequest = {
  subscriptionId: string;
  resourceGroup: string;
  groupIsNew: boolean;
  groupRegion: string;
  foundryName: string;
  foundryIsNew: boolean;
  foundryRegion: string;
  format: string;
  model: string;
  version: string;
  sku: string;
  capacity: number;
  deploymentName: string;
};

export type CreateOutcome = {
  outcome: CreateResultScenario;
  operationId: string;
  createdGroup: boolean;
  createdFoundry: boolean;
  detail: string;
};

type Props = {
  catalog: AddModelCatalog;
  loadModels: (target: ModelTarget, scenario: ModelLoadScenario) => Promise<ModelCandidate[]>;
  loadDeploymentNames: (foundryId: string) => Promise<string[]>;
  onCreate: (request: CreateRequest) => Promise<CreateOutcome>;
  onCheck: (operationId: string) => Promise<CreateOutcome>;
  onRefreshList: () => Promise<void>;
  onCreateScenarioChange: (scenario: CreateResultScenario) => void;
  mock: boolean;
  onClose: () => void;
};
type NewResource = { name: string; region: string };
type Screen = "model" | "foundry" | "group" | "review" | "done";

const FOUNDRY_PATTERN = /^[a-zA-Z0-9][a-zA-Z0-9-]{0,62}[a-zA-Z0-9]$/;
const GROUP_PATTERN = /^[\p{L}\p{Nd}_().-]{1,90}$/u;
const DEPLOYMENT_PATTERN = /^[a-zA-Z0-9_.-]{2,64}$/;
const FOUNDRY_RULE = "2〜64文字の半角英数字・ハイフン。先頭と末尾は英数字。";
const GROUP_RULE = "1〜90文字の文字・数字・_ - . ( )。末尾のピリオドと空白は使用できません。";
const DEPLOYMENT_RULE = "2〜64文字の半角英数字・アンダースコア・ハイフン・ピリオド。";
const CAPACITY_UNKNOWN = "容量の設定条件を取得できません。モデル候補を再取得してください。";

const RESULT_TITLES: Record<CreateResultScenario, string> = {
  success: "モデルを追加しました",
  "group-failure": "Foundryを作成できませんでした",
  "foundry-candidate-mismatch": "選択した設定ではモデルを追加できません",
  "deployment-failure": "デプロイを作成できませんでした",
  unknown: "作成結果を確認できません",
  conflict: "同じ名前のデプロイが既にあります",
  "list-failure": "モデルは追加されましたが、一覧を更新できませんでした",
  interrupted: "作成結果の確認を中断しました",
};

function modelOptionValue(candidate: ModelCandidate): string {
  return candidate.format + "::" + candidate.name;
}

function getUniqueDefaultVersion(candidate?: ModelCandidate): ModelVersion | undefined {
  if (!candidate) return undefined;
  const defaults = candidate.versions.filter(version => version.isDefault === true);
  return defaults.length === 1 ? defaults[0] : undefined;
}

function getPreferredSku(version?: ModelVersion): ModelSku | undefined {
  const rank = (name: string) => name === "GlobalStandard" ? 0 : name.startsWith("Global") ? 1 : name === "DataZoneStandard" ? 2 : name === "Standard" ? 3 : 4;
  return version?.skus.slice().sort((a, b) => rank(a.name) - rank(b.name) || (a.name < b.name ? -1 : a.name > b.name ? 1 : 0))[0];
}

function hasCapacityConstraint(contract?: CapacityContract): boolean {
  if (!contract) return false;
  const { min, max, step } = contract;
  if ([min, max, step].some(value => value !== undefined && (!Number.isSafeInteger(value) || value < 0))) return false;
  if (step === 0 || (min !== undefined && max !== undefined && min > max)) return false;
  if (contract.allowedValues !== undefined) {
    return contract.allowedValues.length > 0 && contract.allowedValues.every(value =>
      Number.isSafeInteger(value) && value >= 0
      && (min === undefined || value >= min)
      && (max === undefined || value <= max)
      && (step === undefined || (min !== undefined && (value - min) % step === 0)));
  }
  // Azure reports a maximum for every SKU, but minimum and step only for the
  // provisioned ones. Requiring all three would exclude every pay-as-you-go SKU,
  // so a maximum alone is enough and the lower bound falls back to 1.
  return max !== undefined;
}

function getDefaultCapacity(sku?: ModelSku): string {
  const value = sku?.capacity?.default;
  return value !== undefined && !getCapacityError(String(value), sku?.capacity) ? String(value) : "";
}

function getCapacityError(value: string, contract?: CapacityContract): string {
  if (!hasCapacityConstraint(contract)) return CAPACITY_UNKNOWN;
  if (!value) return "容量を入力してください。";
  const number = Number(value);
  if (!Number.isSafeInteger(number)) return "容量は整数で入力してください。";
  if (contract?.allowedValues?.length && !contract.allowedValues.includes(number)) {
    return `容量は ${contract.allowedValues.join("、")} のいずれかを入力してください。`;
  }
  if (typeof contract?.min === "number" && number < contract.min) return `容量は ${contract.min} 以上で入力してください。`;
  if (typeof contract?.min !== "number" && number < 1) return "容量は 1 以上で入力してください。";
  if (typeof contract?.max === "number" && number > contract.max) return `容量は ${contract.max} 以下で入力してください。`;
  if (typeof contract?.step === "number" && contract.step > 0) {
    const base = typeof contract.min === "number" ? contract.min : 0;
    const ratio = (number - base) / contract.step;
    if (Math.abs(ratio - Math.round(ratio)) > 1e-9) return `容量は ${base} から ${contract.step} 刻みで入力してください。`;
  }
  return "";
}

function getCapacityHint(contract?: CapacityContract): string {
  if (!contract || !hasCapacityConstraint(contract)) return CAPACITY_UNKNOWN;
  const parts: string[] = [];
  if (contract.allowedValues?.length) parts.push("許可値: " + contract.allowedValues.join(", "));
  if (typeof contract.min === "number") parts.push("最小 " + contract.min);
  if (typeof contract.max === "number") parts.push("最大 " + contract.max);
  if (typeof contract.step === "number") parts.push("刻み " + contract.step);
  return parts.join(" / ");
}

export default function AddModel({ catalog, loadModels, loadDeploymentNames, onCreate, onCheck, onRefreshList, onCreateScenarioChange, mock, onClose }: Props) {
  const dialog = useRef<HTMLDialogElement>(null);
  const heading = useRef<HTMLHeadingElement>(null);
  const modelRequest = useRef(0);
  const [screen, setScreen] = useState<Screen>("model");
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
  const [models, setModels] = useState<ModelCandidate[]>([]);
  const [existingNames, setExistingNames] = useState<string[]>([]);
  const [modelStatus, setModelStatus] = useState<"idle" | "loading" | "ready" | "error">("idle");
  const [modelError, setModelError] = useState("");
  const [loadScenario, setLoadScenario] = useState<ModelLoadScenario>("success");
  const [model, setModel] = useState("");
  const [version, setVersion] = useState("");
  const [sku, setSku] = useState("");
  const [name, setName] = useState("");
  const [capacity, setCapacity] = useState("");
  const [createScenario, setCreateScenario] = useState<CreateResultScenario>("success");
  const [outcome, setOutcome] = useState<CreateOutcome | null>(null);
  const [creating, setCreating] = useState(false);
  const [followup, setFollowup] = useState("");
  const [checking, setChecking] = useState(false);

  useEffect(() => () => { modelRequest.current += 1; }, []);
  useEffect(() => { dialog.current?.showModal(); }, []);
  useEffect(() => { heading.current?.focus(); }, [screen]);

  const groups = catalog.groups.filter(item => item.subscription === subscription);
  const foundries = catalog.foundries.filter(item => item.subscription === subscription && item.group === group);
  const selectedModel = models.find(item => modelOptionValue(item) === model);
  const selectedVersion = selectedModel?.versions.find(item => item.version === version);
  const selectedSku = selectedVersion?.skus.find(item => item.name === sku);
  const selectedSubscription = catalog.subscriptions.find(item => item.id === subscription);
  const selectedExistingFoundry = foundries.find(item => item.name === foundry);
  const region = newFoundry?.name === foundry ? newFoundry.region : selectedExistingFoundry?.region;
  const groupIsNew = newGroup?.name === group;
  const foundryIsNew = newFoundry?.name === foundry;
  const existingDeployment = Boolean(name && existingNames.some(item => item.toLowerCase() === name.toLowerCase()));
  const foundryError = foundryName && (!FOUNDRY_PATTERN.test(foundryName)
    ? FOUNDRY_RULE
    : foundries.some(item => item.name.toLowerCase() === foundryName.toLowerCase())
      ? "同じリソースグループに同名のFoundryがあります。"
      : "");
  const groupError = groupName && (!GROUP_PATTERN.test(groupName) || groupName.endsWith(".")
    ? GROUP_RULE
    : groups.some(item => item.name.toLowerCase() === groupName.toLowerCase())
      ? "同じサブスクリプションに同名のリソースグループがあります。"
      : "");
  const deployError = name && !DEPLOYMENT_PATTERN.test(name)
    ? DEPLOYMENT_RULE
    : name && existingDeployment
      ? "同じFoundryに同名のデプロイがあります。"
      : "";
  const capacityError = selectedSku ? getCapacityError(capacity, selectedSku.capacity) : "";
  const canReview = modelStatus === "ready"
    && models.length > 0
    && Boolean(selectedModel?.format && selectedModel?.name && selectedVersion?.version && selectedSku && capacity && name)
    && (createScenario !== "group-failure" || foundryIsNew)
    && !deployError
    && !capacityError;
  const groupSummary = groupIsNew ? group + "（新規 / " + newGroup?.region + "）" : group + "（既存）";
  const foundrySummary = foundryIsNew ? foundry + "（新規 / " + region + "）" : foundry + "（既存 / " + region + "）";
  // The result screen follows what the service reported, not what was selected.
  const resultOutcome = outcome?.outcome ?? "success";
  const resultTitle = RESULT_TITLES[resultOutcome];
  const createdResources = [
    outcome?.createdGroup ? `リソースグループ「${group}」は作成済みです。` : "",
    outcome?.createdFoundry ? `Foundry「${foundry}」は作成済みです。` : "",
  ].filter(Boolean).join("\n");
  const resultDescriptions: Record<CreateResultScenario, string[]> = {
    success: [`デプロイ「${name}」を作成しました。`],
    "group-failure": [createdResources, `Foundry「${foundry}」がAzureに残っている可能性があります。Azureで状態を確認してください。`, "デプロイの作成は開始していません。"],
    "foundry-candidate-mismatch": ["選択したモデル・バージョン・SKU・容量の組み合わせが、現在の候補と一致しません。", createdResources, "デプロイの作成は開始していません。Foundryで利用できるモデルと設定を確認してください。"],
    "deployment-failure": [createdResources, `デプロイ「${name}」がAzureに残っている可能性があります。Azureで状態を確認してください。`],
    unknown: [`デプロイ「${name}」の作成が完了したか確認できませんでした。`, createdResources, "Azureで処理が続いている可能性があります。再度追加せず、「状態を確認」を押してください。"],
    conflict: [`デプロイ名「${name}」は既に使われています。既存のデプロイは変更していません。`, createdResources, "別の名前で追加してください。"],
    "list-failure": [`デプロイ「${name}」は作成済みです。`, "再度追加する必要はありません。「一覧を再取得」を押してください。"],
    interrupted: [`デプロイ「${name}」の作成処理はAzureで続いている可能性があります。`, createdResources, "この操作では作成を取り消せません。再度追加せず、「状態を確認」を押してください。"],
  };
  const resultDescription = [...resultDescriptions[resultOutcome], outcome?.detail ?? ""].filter(Boolean).join("\n");
  const resultFollowup = checking ? "確認しています…" : followup;

  async function submitCreate() {
    if (!canReview || creating) return;
    setCreating(true);
    setFollowup("");
    try {
      const created = await onCreate({
        subscriptionId: subscription,
        resourceGroup: group,
        groupIsNew,
        groupRegion: newGroup?.region ?? "",
        foundryName: foundry,
        foundryIsNew,
        foundryRegion: region ?? "",
        format: selectedModel?.format ?? "",
        model: selectedModel?.name ?? "",
        version,
        sku,
        capacity: Number(capacity),
        deploymentName: name,
      });
      setOutcome(created);
    } catch (cause) {
      setOutcome({ outcome: "unknown", operationId: "", createdGroup: false, createdFoundry: false, detail: String(cause instanceof Error ? cause.message : cause) });
    } finally {
      setCreating(false);
      setScreen("done");
    }
  }

  // The follow-up text reports what the re-read actually returned. A retry
  // that succeeded is never described as a failure, and a result that is still
  // unknown is never described as a success.
  async function checkResult() {
    if (checking) return;
    setChecking(true);
    try {
      if (resultOutcome === "list-failure") {
        await onRefreshList();
        setFollowup("一覧を更新しました。");
      } else if (outcome?.operationId) {
        const checked = await onCheck(outcome.operationId);
        setOutcome(checked);
        setFollowup(checked.outcome === "unknown" || checked.outcome === "interrupted"
          ? "まだ作成結果を確認できません。時間をおいて、もう一度確認してください。"
          : "");
      }
    } catch (cause) {
      setFollowup(resultOutcome === "list-failure"
        ? "一覧を取得できませんでした。時間をおいて、もう一度お試しください。"
        : String(cause instanceof Error ? cause.message : cause));
    } finally {
      setChecking(false);
    }
  }
  function resetModel() {
    modelRequest.current += 1;
    setModels([]);
    setExistingNames([]);
    setModelStatus("idle");
    setModelError("");
    setModel("");
    setVersion("");
    setSku("");
    setName("");
    setCapacity("");
  }

  function modelTarget(foundryName: string): ModelTarget | null {
    if (!foundryName) return null;
    const existing = catalog.foundries.find(item =>
      item.subscription === subscription && item.group === group && item.name === foundryName);
    if (existing) return { foundryId: existing.id, subscription, region: existing.region };
    if (newFoundry?.name === foundryName) return { foundryId: "", subscription, region: newFoundry.region };
    return null;
  }

  async function fetchModels(target: ModelTarget | null) {
    resetModel();
    if (!target) return;
    const request = modelRequest.current;
    setModelStatus("loading");
    try {
      const [candidates, names] = await Promise.all([
        loadModels(target, loadScenario),
        target.foundryId ? loadDeploymentNames(target.foundryId) : Promise.resolve<string[]>([]),
      ]);
      if (request !== modelRequest.current) return;
      setModels(candidates);
      setExistingNames(names);
      setModelStatus("ready");
    } catch (cause) {
      if (request !== modelRequest.current) return;
      setModelError(String(cause instanceof Error ? cause.message : cause));
      setModelStatus("error");
    }
  }

  function chooseFoundry(value: string) {
    setFoundry(value);
    void fetchModels(modelTarget(value));
  }

  function chooseModel(value: string) {
    const candidate = models.find(item => modelOptionValue(item) === value);
    const defaultVersion = getUniqueDefaultVersion(candidate);
    const defaultSku = getPreferredSku(defaultVersion);
    setModel(value);
    setVersion(defaultVersion?.version ?? "");
    setSku(defaultSku?.name ?? "");
    setCapacity(getDefaultCapacity(defaultSku));
    setName(candidate?.name ?? "");
  }

  function chooseVersion(value: string) {
    const nextVersion = selectedModel?.versions.find(item => item.version === value);
    const nextSku = getPreferredSku(nextVersion);
    setVersion(value);
    setSku(nextSku?.name ?? "");
    setCapacity(getDefaultCapacity(nextSku));
  }

  function chooseSku(value: string) {
    const nextSku = selectedVersion?.skus.find(item => item.name === value);
    setSku(value);
    setCapacity(getDefaultCapacity(nextSku));
  }

  function changeGroup(value: string) {
    setGroup(value);
    setFoundry("");
    setNewFoundry(null);
    resetModel();
  }

  function openGroup(from: "model" | "foundry") {
    setGroupReturn(from);
    setGroupName("");
    setScreen("group");
  }

  function handleModelSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!canReview) return;
    setScreen("review");
  }

  function handleFoundrySubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!foundryName || foundryError) return;
    const value = foundryName.trim();
    setNewFoundry({ name: value, region: foundryRegion });
    setFoundry(value);
    void fetchModels({ foundryId: "", subscription, region: foundryRegion });
    setScreen("model");
  }

  function handleGroupSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!groupName || groupError) return;
    const value = groupName.trim();
    setNewGroup({ name: value, region: groupRegion });
    changeGroup(value);
    setScreen(groupReturn);
  }

  const groupPicker = (
    <div className="add-field">
      <label htmlFor="add-group">リソースグループ</label>
      <div className="add-select-action">
        <select id="add-group" required value={group} disabled={!subscription} onChange={event => changeGroup(event.target.value)}>
          <option value="">選択してください</option>
          {groups.map(item => <option key={item.name} value={item.name}>{item.name}</option>)}
          {newGroup ? <option value={newGroup.name}>{newGroup.name}（新規作成予定）</option> : null}
        </select>
        <button type="button" className="back-button" disabled={!subscription} onClick={() => openGroup(screen === "foundry" ? "foundry" : "model")}>リソースグループを新規作成</button>
      </div>
    </div>
  );

  return (
    <dialog ref={dialog} className="add-dialog" aria-labelledby="add-title" onCancel={onClose}>
      <header className="add-heading">
        <div><h2 id="add-title" ref={heading} tabIndex={-1}>{screen === "done" ? resultTitle : screen === "model" ? "モデルを追加する" : screen === "foundry" ? "Foundryを追加する" : screen === "group" ? "リソースグループを追加する" : "追加内容の確認"}</h2></div>
        {screen === "done" ? null : <button type="button" className="back-button" onClick={onClose} aria-label="モデル追加を閉じる">閉じる</button>}
      </header>
      <p className="add-notice">{mock ? "MOCK · Azureへの作成・保存は行いません" : "読み取り専用 · 作成処理は未実装のため、Azureへ書き込みません"}</p>

      {screen === "model" ? (
        <form onSubmit={handleModelSubmit}>
          <div className="add-section">
            <label className="add-field">サブスクリプション
              <select aria-label="サブスクリプション" required value={subscription} onChange={event => { setSubscription(event.target.value); changeGroup(""); setNewGroup(null); }}>
                <option value="">選択してください</option>
                {catalog.subscriptions.map(item => <option key={item.id} value={item.id}>{item.name} — {item.tenant}</option>)}
              </select>
            </label>
            {groupPicker}
            <div className="add-field">
              <label htmlFor="add-foundry">Foundry <small className="region-label">{region}</small></label>
              <div className="add-select-action">
                <select id="add-foundry" aria-label="Foundry" required disabled={!group} value={foundry} onChange={event => chooseFoundry(event.target.value)}>
                  <option value="">{group && !foundries.length && !newFoundry ? "Foundryがありません。新規作成してください" : "選択してください"}</option>
                  {foundries.map(item => <option key={item.name} value={item.name}>{item.name}{item.empty ? "（デプロイ0件）" : ""}</option>)}
                  {newFoundry ? <option value={newFoundry.name}>{newFoundry.name}（新規作成予定）</option> : null}
                </select>
                <button type="button" className="back-button" disabled={!subscription} onClick={() => { setFoundryName(""); setScreen("foundry"); }}>Foundryを新規作成</button>
              </div>
            </div>
          </div>

          <div className="add-model-load" aria-live="polite" aria-busy={modelStatus === "loading"}>
            {foundryIsNew ? <span className="add-candidate-note">このリージョンの候補です。Foundryの作成後に再確認します。</span> : null}
            {modelStatus === "loading"
              ? <><span className="spinner" aria-hidden="true" /> <span>{foundry} · 読み込み中…</span><progress aria-label="モデル候補の読込状況" /></>
              : modelStatus === "error"
                ? <><span className="add-error">{modelError}</span><button type="button" className="text-button" onClick={() => void fetchModels(modelTarget(foundry))}>モデル候補を再試行</button></>
                : modelStatus === "ready"
                  ? <span>{models.length ? "モデル候補 " + models.length + "件" : "利用できるモデル候補がありません"}</span>
                  : <span />}
          </div>

          <fieldset className="add-section" disabled={modelStatus !== "ready" || !models.length}>
            <legend className="sr-only">モデルとデプロイ設定</legend>
            <div className="add-grid">
              <label className="add-field">モデル
                <select aria-label="モデル" required value={model} onChange={event => chooseModel(event.target.value)}>
                  <option value="">選択してください</option>
                  {models.map(item => <option key={modelOptionValue(item)} value={modelOptionValue(item)} disabled={!item.format || !item.name}>{item.format || "形式不明"} / {item.name || "モデル名不明"}</option>)}
                </select>
              </label>
              <label className="add-field">バージョン
                <select aria-label="バージョン" required value={version} onChange={event => chooseVersion(event.target.value)}>
                  <option value="">選択してください</option>
                  {selectedModel?.versions.map(item => <option key={item.version} value={item.version} disabled={!item.version}>{item.version || "バージョン不明"}</option>)}
                </select>
              </label>
              <label className="add-field">SKU
                <select aria-label="SKU" required value={sku} onChange={event => chooseSku(event.target.value)}>
                  <option value="">選択してください</option>
                  {selectedVersion?.skus.map(item => <option key={item.name} value={item.name}>{item.name}</option>)}
                </select>
              </label>
              <div className="add-field">
                <label htmlFor="add-capacity">Capacity</label>
                <input
                  id="add-capacity"
                  aria-label="Capacity"
                  aria-describedby="capacity-validation"
                  aria-invalid={Boolean(capacityError)}
                  type="number"
                  required
                  min={selectedSku?.capacity?.min}
                  max={selectedSku?.capacity?.max}
                  step={selectedSku?.capacity?.step}
                  list={selectedSku?.capacity?.allowedValues?.length ? "capacity-allowed-values" : undefined}
                  value={capacity}
                  onChange={event => setCapacity(event.target.value)}
                />
                {selectedSku?.capacity?.allowedValues?.length
                  ? <datalist id="capacity-allowed-values">{selectedSku.capacity.allowedValues.map(value => <option key={value} value={value} />)}</datalist>
                  : null}
              </div>
            </div>
            <p id="capacity-validation" className={capacityError ? "add-capacity-validation add-error" : "add-capacity-validation"} aria-live="polite">{selectedSku ? capacityError || getCapacityHint(selectedSku.capacity) : "SKUを選択すると容量条件を表示します。"}</p>
            <div className="add-grid">
              <label className="add-field">デプロイ名
                <input required inputMode="url" autoCapitalize="none" autoCorrect="off" spellCheck={false} aria-describedby="deploy-validation" aria-invalid={Boolean(deployError)} value={name} onChange={event => setName(event.target.value)} placeholder="例: chat-production" />
              </label>
              <p id="deploy-validation" className={deployError ? "add-validation add-validation-inline add-error" : "add-validation add-validation-inline"} aria-live="polite">{deployError || DEPLOYMENT_RULE}</p>
            </div>
          </fieldset>

          {mock ? (<>
          <details className="add-load-controls">
            <summary>モック：モデル取得の再現</summary>
            <label>モデル取得の状態
              <select aria-label="モデル取得の状態" value={loadScenario} onChange={event => setLoadScenario(event.target.value as ModelLoadScenario)}>
                <option value="success">通常（約0.8秒）</option>
                <option value="slow">時間がかかる（約8秒）</option>
                <option value="empty">候補なし</option>
                <option value="failure">取得失敗</option>
                <option value="no-default">既定版なし</option>
                <option value="missing-capacity">容量欠損</option>
              </select>
            </label>
            <button type="button" className="text-button" disabled={!foundry} onClick={() => void fetchModels(modelTarget(foundry))}>モデル候補を再取得</button>
          </details>
          <details className="add-load-controls add-create-controls">
            <summary>モック：作成結果の再現</summary>
            <label>作成結果の状態
              <select aria-label="作成結果の状態" value={createScenario} onChange={event => { const next = event.target.value as CreateResultScenario; setCreateScenario(next); onCreateScenarioChange(next); }}>
                <option value="success">通常成功</option>
                <option value="group-failure" disabled={!foundryIsNew}>RG作成後失敗（新規Foundry）</option>
                <option value="foundry-candidate-mismatch">Foundry作成後候補不一致</option>
                <option value="deployment-failure">デプロイ失敗</option>
                <option value="unknown">結果不明</option>
                <option value="conflict">同名競合</option>
                <option value="list-failure">作成成功・一覧更新失敗</option>
                <option value="interrupted">中断</option>
              </select>
            </label>
          </details>
          </>) : null}
          <div className="add-actions">
            <button type="button" className="back-button" onClick={onClose}>キャンセル</button>
            <button className="primary" disabled={!canReview}>確認</button>
          </div>
        </form>
      ) : null}

      {screen === "foundry" ? (
        <form onSubmit={handleFoundrySubmit}>
          <p className="add-context">{selectedSubscription?.name}</p>
          {groupPicker}
          <label className="add-field">新しいFoundry名
            <input required inputMode="url" autoCapitalize="none" autoCorrect="off" spellCheck={false} aria-describedby="foundry-validation" aria-invalid={Boolean(foundryError)} value={foundryName} onChange={event => setFoundryName(event.target.value)} />
          </label>
          <p id="foundry-validation" className={foundryError ? "add-validation add-error" : "add-validation"} aria-live="polite">{foundryError || FOUNDRY_RULE}</p>
          <label className="add-field">Foundryのリージョン
            <select aria-label="Foundryのリージョン" value={foundryRegion} onChange={event => setFoundryRegion(event.target.value)}>
              <option>japaneast</option><option>eastus</option><option>swedencentral</option>
            </select>
          </label>
          <div className="add-actions">
            <button type="button" className="back-button" onClick={() => setScreen("model")}>戻る</button>
            <button className="primary" disabled={!foundryName || Boolean(foundryError)}>使用</button>
          </div>
        </form>
      ) : null}

      {screen === "group" ? (
        <form onSubmit={handleGroupSubmit}>
          <p className="add-context">{selectedSubscription?.name}</p>
          <label className="add-field">新しいリソースグループ名
            <input required inputMode="url" autoCapitalize="none" autoCorrect="off" spellCheck={false} aria-describedby="group-validation" aria-invalid={Boolean(groupError)} value={groupName} onChange={event => setGroupName(event.target.value)} />
          </label>
          <p id="group-validation" className={groupError ? "add-validation add-error" : "add-validation"} aria-live="polite">{groupError || GROUP_RULE}</p>
          <label className="add-field">リソースグループのリージョン
            <select aria-label="リソースグループのリージョン" value={groupRegion} onChange={event => setGroupRegion(event.target.value)}>
              <option>japaneast</option><option>eastus</option><option>swedencentral</option>
            </select>
          </label>
          <div className="add-actions">
            <button type="button" className="back-button" onClick={() => setScreen(groupReturn)}>戻る</button>
            <button className="primary" disabled={!groupName || Boolean(groupError)}>使用</button>
          </div>
        </form>
      ) : null}

      {screen === "review" || screen === "done" ? (
        <>
          <dl className="add-summary">
            {[
              ["サブスクリプション", selectedSubscription?.name ?? ""],
              ["リソースグループ", groupSummary],
              ["Foundry", foundrySummary],
              ["形式 / モデル / バージョン", (selectedModel?.format ?? "") + " / " + (selectedModel?.name ?? "") + " / " + version],
              ["SKU / Capacity", sku + " / " + capacity],
              ["バージョン更新", "自動更新なし（モデル廃止時に停止）"],
              ["デプロイ名", name],
            ].map(([label, value]) => <div key={label}><dt>{label}</dt><dd>{value}</dd></div>)}
          </dl>
          {foundryIsNew ? (
            <details className="add-proposal">
              <summary>新規Foundryの設定</summary>
              <dl className="add-proposal-list">
                <div><dt>種類</dt><dd>AIServices</dd></div>
                <div><dt>アカウントSKU</dt><dd>S0</dd></div>
                <div><dt>公開ネットワーク</dt><dd>有効</dd></div>
                <div><dt>キー認証</dt><dd>無効（Entra ID）</dd></div>
                <div><dt>カスタムサブドメイン</dt><dd>{foundry}</dd></div>
                <div><dt>マネージドID</dt><dd>システム割り当て</dd></div>
                <div><dt>プロジェクト</dt><dd>作成しない</dd></div>
              </dl>
            </details>
          ) : null}
          {screen === "review" ? (
            <>
              <p className="add-plan-note">{foundryIsNew ? `${groupIsNew ? "リソースグループとFoundry" : "Foundry"}を作成し、モデルの設定を再確認してからデプロイを作成します。途中で中断・失敗しても、作成済みのリソースは削除されません。` : "モデルの設定を再確認し、同じFoundryに同名のデプロイがなければ作成します。"}</p>
              {mock ? null : <p className="add-result-followup">この操作でAzureにリソースを作成します。失敗しても作成済みのリソースは削除されません。</p>}
              <div className="add-actions">
                <button type="button" className="back-button" disabled={creating} onClick={() => setScreen("model")}>変更</button>
                <button type="button" className="primary" disabled={creating} onClick={() => void submitCreate()}>{creating ? "追加しています…" : "追加"}</button>
              </div>
            </>
          ) : (
            <>
              <p className="add-result-description">{resultDescription}</p>
              <p className="add-result-followup" aria-live="polite">{resultFollowup}</p>
              <div className="add-actions">
                {resultOutcome === "unknown" || resultOutcome === "interrupted"
                  ? <button type="button" className="back-button" disabled={checking} onClick={() => void checkResult()}>状態を確認</button>
                  : resultOutcome === "list-failure"
                    ? <button type="button" className="back-button" disabled={checking} onClick={() => void checkResult()}>一覧を再取得</button>
                    : null}
                <button type="button" className="primary" onClick={onClose}>閉じる</button>
              </div>
            </>
          )}
        </>
      ) : null}
    </dialog>
  );
}

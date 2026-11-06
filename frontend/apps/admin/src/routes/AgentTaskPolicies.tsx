import { useCallback, useEffect, useRef, useState } from "react";
import { isApiError, isNetworkError } from "@inklore/contracts";
import { useTranslation } from "@inklore/i18n";
import { Button, ErrorPresenter, useImeComposition } from "@inklore/ui";
import { adminSession, type LlmModel } from "../api/adminApi";
import { type AgentPolicy, type AgentPolicyInput, getAgentPolicy, listAgentPolicies, listPolicyModels, publishAgentPolicy } from "../api/agentPolicies";
import { STANDARD_TASK_KINDS } from "./AgentGovernanceModals";

const CAPABILITIES = ["text", "vision", "image", "embedding", "rerank", "video", "audio", "tools", "structured", "reasoning"];
type Draft = Omit<AgentPolicyInput, "max_output_tokens" | "temperature" | "timeout_seconds" | "max_retries"> & { max_output_tokens: string; temperature: string; timeout_seconds: string; max_retries: string };
const emptyDraft = (): Draft => ({ task_key: "", task_kinds: [], title: "", enabled: true, primary_model_id: "", fallback_model_ids: [], max_output_tokens: "", temperature: "", timeout_seconds: "", max_retries: "", required_capabilities: ["text"], output_mode: "text" });
const draftFrom = (value: AgentPolicy): Draft => ({ task_key: value.task_key, task_kinds: [...value.task_kinds], title: value.title, enabled: value.enabled, primary_model_id: value.primary_model_id, fallback_model_ids: [...value.fallback_model_ids], max_output_tokens: String(value.max_output_tokens), temperature: String(value.temperature), timeout_seconds: String(value.timeout_seconds), max_retries: String(value.max_retries), required_capabilities: [...value.required_capabilities], output_mode: value.output_mode });
const limits = { max_output_tokens: [1, 16384, 1], temperature: [0, 2, 0.01], timeout_seconds: [1, 180, 1], max_retries: [0, 3, 1] } as const;

export function AgentTaskPolicies() {
  const { t } = useTranslation(); const ime = useImeComposition();
  const [items, setItems] = useState<AgentPolicy[]>([]); const [models, setModels] = useState<LlmModel[]>([]);
  const [selected, setSelected] = useState<AgentPolicy | null>(null); const [draft, setDraft] = useState<Draft | null>(null);
  const [loading, setLoading] = useState(false); const [saving, setSaving] = useState(false); const [unknown, setUnknown] = useState(false);
  const [error, setError] = useState<unknown>(null); const [invalid, setInvalid] = useState(false); const [denied, setDenied] = useState(false);
  const [saved, setSaved] = useState<AgentPolicy | null>(null); const [operator, setOperator] = useState(() => adminSession.getSnapshot().principal?.roles.includes("operator") ?? false);
  const generation = useRef(0); const detailRequest = useRef(0); const locked = useRef(false);
  const intent = useRef<{ id: string | null; key: string; input: AgentPolicyInput & { expected_revision?: number } } | null>(null);
  const clear = useCallback(() => { generation.current++; detailRequest.current++; setItems([]); setModels([]); setSelected(null); setDraft(null); setSaved(null); intent.current = null; setUnknown(false); setDenied(true); setOperator(false); }, []);
  const fail = useCallback((cause: unknown) => { if (isApiError(cause) && [401, 403].includes(cause.httpStatus)) clear(); setError(cause); }, [clear]);
  const load = useCallback(async () => {
    const current = generation.current; setLoading(true);
    try { const [policies, available] = await Promise.all([listAgentPolicies(), listPolicyModels()]); if (current === generation.current) { setItems(policies); setModels(available); setError(null); } }
    catch (cause) { if (current === generation.current) fail(cause); }
    finally { if (current === generation.current) setLoading(false); }
  }, [fail]);
  useEffect(() => {
    const epoch = adminSession.getSessionEpoch(); const unsubscribe = adminSession.subscribe(value => { if (value.status !== "authenticated" || value.sessionEpoch !== epoch) clear(); else setOperator(value.principal?.roles.includes("operator") ?? false); });
    void load(); return () => { unsubscribe(); generation.current++; };
  }, [clear, load]);
  const edit = async (id: string) => {
    if (unknown || saving) return;
    const current = generation.current;
    const request = ++detailRequest.current;
    try { const value = await getAgentPolicy(id); if (current === generation.current && request === detailRequest.current) { setSelected(value); setDraft(draftFrom(value)); setError(null); setSaved(null); setInvalid(false); } }
    catch (cause) { if (current === generation.current && request === detailRequest.current) fail(cause); }
  };
  const update = <K extends keyof Draft>(key: K, value: Draft[K]) => setDraft(previous => previous ? { ...previous, [key]: value } : previous);
  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (locked.current || !draft || denied || !operator || ime.isComposingRef.current || ime.justEndedRef.current) return;
    if (!intent.current) {
      const body: AgentPolicyInput & { expected_revision?: number } = { ...draft, task_key: draft.task_key.trim(), title: draft.title.trim(), max_output_tokens: Number(draft.max_output_tokens), temperature: Number(draft.temperature), timeout_seconds: Number(draft.timeout_seconds), max_retries: Number(draft.max_retries), ...(selected ? { expected_revision: selected.revision } : {}) };
      if (!/^[a-z][a-z0-9_]{0,63}$/.test(body.task_key) || !body.title || [...body.title].length > 120 || body.task_kinds.length === 0 || body.required_capabilities.length === 0 || !body.primary_model_id || new Set([body.primary_model_id, ...body.fallback_model_ids]).size !== body.fallback_model_ids.length + 1 || Object.entries(limits).some(([key, [min, max, step]]) => { const value = body[key as keyof typeof limits]; return !draft[key as keyof typeof limits] || !Number.isFinite(value) || value < min || value > max || (step === 1 && !Number.isInteger(value)); })) { setInvalid(true); return; }
      intent.current = { id: selected?.id ?? null, key: crypto.randomUUID(), input: body };
    }
    const current = generation.current; locked.current = true; setSaving(true); setError(null); setInvalid(false);
    try {
      const value = await publishAgentPolicy(intent.current.id, intent.current.input, intent.current.key);
      if (current !== generation.current) return;
      intent.current = null; setUnknown(false); setSaved(value); setSelected(value); setDraft(null);
      setItems(previous => [value, ...previous.filter(item => item.id !== value.id)]);
    } catch (cause) {
      if (current !== generation.current) return;
      const uncertain = isNetworkError(cause) || (isApiError(cause) && (cause.httpStatus >= 500 || cause.code === "platform.operation_in_progress"));
      setUnknown(uncertain); if (!uncertain) intent.current = null; fail(cause);
    } finally { locked.current = false; if (current === generation.current) setSaving(false); }
  };
  const inputClass = "w-full min-w-0 rounded border border-[var(--border-decorative)] bg-[var(--bg-canvas)] p-2";
  return <section aria-label={t("agentPolicy.title")} className="flex-1 min-h-0 overflow-auto p-4 space-y-4">
    <header className="flex flex-wrap justify-between gap-2"><h2 className="font-semibold">{t("agentPolicy.title")}</h2><div className="flex gap-2"><Button variant="outline" disabled={loading || denied} onClick={() => void load()}>{t("common.refresh")}</Button>{operator && <Button disabled={saving || unknown} onClick={() => { detailRequest.current++; setSelected(null); setDraft(emptyDraft()); setSaved(null); setInvalid(false); setError(null); }}>{t("agentPolicy.create")}</Button>}</div></header>
    <p className="text-sm text-[var(--text-secondary)]">{t("agentPolicy.notice")}</p>
    {loading && <p role="status">{t("common.loading")}</p>}
    {error != null && <ErrorPresenter error={error} onRetry={denied ? undefined : () => void load()} />}
    {denied && <p role="alert">{t("agentQuestion.denied")}</p>}
    {saved && <p role="status">{t("agentPolicy.published", saved.version, saved.current_version_id)}</p>}
    {!loading && error == null && items.length === 0 && <p>{t("agentPolicy.empty")}</p>}
    <div className="space-y-2">{items.map(item => <article key={item.id} className="rounded border border-[var(--border-decorative)] p-3 space-y-2 text-sm break-words"><h3 className="font-medium">{item.title} · v{item.version}</h3><p>{item.task_key} · {item.task_kinds.join(", ")}</p><p>{t("agentQuestion.policyBudget", item.version, item.max_output_tokens, item.timeout_seconds)}</p><p>{t("agentPolicy.current", item.revision, item.current_version_id)}</p><p>{t(item.enabled ? "agentPolicy.enabled" : "agentPolicy.disabled")}</p>{operator && <Button size="sm" variant="outline" disabled={saving || unknown} onClick={() => void edit(item.id)}>{t("agentPolicy.edit")}</Button>}</article>)}</div>
    {draft && <form onSubmit={submit} className="rounded-xl border border-[var(--border-decorative)] p-4 space-y-4" onKeyDown={event => { if (event.key === "Enter" && ime.shouldBlockKeyAction(event)) { event.preventDefault(); event.stopPropagation(); } }}>
      <h3 className="font-semibold">{t(selected ? "agentPolicy.edit" : "agentPolicy.create")}</h3>
      {selected && <p className="text-xs break-words">{t("agentPolicy.current", selected.revision, selected.current_version_id)}</p>}
      <fieldset disabled={saving || unknown || !operator} className="grid grid-cols-1 min-[700px]:grid-cols-2 gap-4 min-w-0">
        <label className="space-y-1 text-sm"><span>{t("agentPolicy.key")}</span><input className={inputClass} required pattern="[a-z][a-z0-9_]{0,63}" readOnly={!!selected} value={draft.task_key} onChange={event => update("task_key", event.target.value)} /></label>
        <label className="space-y-1 text-sm"><span>{t("agentPolicy.name")}</span><input className={inputClass} required maxLength={120} value={draft.title} onChange={event => update("title", event.target.value)} onCompositionStart={ime.handleCompositionStart} onCompositionEnd={ime.handleCompositionEnd} /></label>
        <label className="space-y-1 text-sm"><span>{t("agentPolicy.model")}</span><select className={inputClass} required value={draft.primary_model_id} onChange={event => update("primary_model_id", event.target.value)}><option value="">{t("common.select")}</option>{models.map(model => <option key={model.id} value={model.id} disabled={!model.enabled}>{model.title} · {model.upstream_name}</option>)}</select></label>
        <label className="space-y-1 text-sm"><span>{t("agentPolicy.output")}</span><select className={inputClass} value={draft.output_mode} onChange={event => update("output_mode", event.target.value as Draft["output_mode"])}>{["text", "structured", "embedding"].map(mode => <option key={mode} value={mode}>{mode}</option>)}</select></label>
        {Object.entries(limits).map(([key, [min, max, step]]) => <label key={key} className="space-y-1 text-sm"><span>{t(`agentPolicy.${key}`)}</span><input aria-label={t(`agentPolicy.${key}`)} className={inputClass} type="number" min={min} max={max} step={step} required value={draft[key as keyof typeof limits]} onChange={event => update(key as keyof typeof limits, event.target.value)} />{selected && <small>{t("agentPolicy.previous", selected[key as keyof typeof limits])}</small>}</label>)}
        <fieldset className="min-w-0 space-y-2"><legend className="text-sm mb-2">{t("agentPolicy.tasks")}</legend>{Array.from(new Set([...STANDARD_TASK_KINDS.map(item => item.value), ...draft.task_kinds])).map(kind => <label key={kind} className="flex items-center gap-2 text-sm"><input type="checkbox" checked={draft.task_kinds.includes(kind)} onChange={event => update("task_kinds", event.target.checked ? [...draft.task_kinds, kind] : draft.task_kinds.filter(value => value !== kind))} /><span>{kind}</span></label>)}</fieldset>
        <fieldset className="min-w-0 space-y-2"><legend className="text-sm mb-2">{t("agentPolicy.capabilities")}</legend>{CAPABILITIES.map(capability => <label key={capability} className="flex items-center gap-2 text-sm"><input type="checkbox" checked={draft.required_capabilities.includes(capability)} onChange={event => update("required_capabilities", event.target.checked ? [...draft.required_capabilities, capability] : draft.required_capabilities.filter(value => value !== capability))} /><span>{capability}</span></label>)}</fieldset>
        <fieldset className="min-w-0 space-y-2"><legend className="text-sm mb-2">{t("agentPolicy.fallbacks")}</legend>{[0, 1, 2].map(index => <label key={index} className="block text-sm">{t("agentPolicy.fallback", index + 1)}<select className={inputClass} value={draft.fallback_model_ids[index] ?? ""} onChange={event => { const values = [...draft.fallback_model_ids]; values[index] = event.target.value; update("fallback_model_ids", values.filter(Boolean)); }}><option value="">{t("agentPolicy.none")}</option>{models.map(model => <option key={model.id} value={model.id} disabled={!model.enabled || model.id === draft.primary_model_id || (draft.fallback_model_ids.includes(model.id) && draft.fallback_model_ids[index] !== model.id)}>{model.title}</option>)}</select></label>)}</fieldset>
        <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={draft.enabled} onChange={event => update("enabled", event.target.checked)} />{t("agentPolicy.enabled")}</label>
      </fieldset>
      {invalid && <p role="alert">{t("agentPolicy.invalid")}</p>}
      {unknown && <p role="status">{t("agentQuestion.unknown")}</p>}
      <div className="flex flex-wrap gap-2"><Button type="submit" loading={saving} disabled={!operator || denied}>{t(unknown ? "agentQuestion.retry" : "agentPolicy.publish")}</Button>{!unknown && <Button variant="outline" disabled={saving} onClick={() => setDraft(null)}>{t("common.cancel")}</Button>}{selected && !unknown && <Button variant="ghost" disabled={saving} onClick={() => void edit(selected.id)}>{t("agentPolicy.reload")}</Button>}</div>
    </form>}
  </section>;
}

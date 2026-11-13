import { useEffect, useRef, useState } from "react";
import { type AgentQuestion, type AnswerQuestionInput, isApiError, isNetworkError } from "@inklore/contracts";
import { useTranslation } from "@inklore/i18n";
import { Button, ErrorPresenter, Textarea, useImeComposition } from "@inklore/ui";
import { getApiSDK } from "@inklore/transport";

/** Owner-only question interaction. Admin audit props never accept this data. */
export function QuestionCard({ question, disabled, sourceChanged, onRefresh, onDenied }: {
  question: AgentQuestion; disabled: boolean; sourceChanged: boolean;
  onRefresh: () => Promise<unknown>; onDenied: (error: unknown) => void;
}) {
  const { t } = useTranslation();
  const api = getApiSDK();
  const [current, setCurrent] = useState(question);
  const [text, setText] = useState("");
  const [choices, setChoices] = useState<string[]>([]);
  const [error, setError] = useState<unknown>(null);
  const [invalid, setInvalid] = useState(false);
  const [busy, setBusy] = useState(false);
  const [uncertain, setUncertain] = useState(false);
  const [now, setNow] = useState(Date.now);
  const locked = useRef(false);
  const intent = useRef<{ key: string; body: AnswerQuestionInput } | null>(null);
  const ime = useImeComposition();
  useEffect(() => { setCurrent(value => question.revision > value.revision ? question : value); }, [question]);
  useEffect(() => { const timer = setInterval(() => setNow(Date.now()), 15_000); return () => clearInterval(timer); }, []);
  const schema = current.response_schema;
  const multiple = schema.type === "array";
  const options = multiple ? schema.items.enum : "enum" in schema ? schema.enum : null;
  const expired = Date.parse(current.expires_at) <= now;
  const closed = current.status !== "open" || expired;
  const blocked = busy || disabled || sourceChanged || closed;
  const label = (key: string, fallback: string) => { const value = t(key); return value === key || !key ? fallback : value; };

  const reconcile = async () => {
    const fresh = await api.agent.getQuestion(current.run_id, current.id);
    setCurrent(fresh);
    if (fresh.status !== "open") { intent.current = null; setUncertain(false); setText(""); setChoices([]); }
    await onRefresh();
    return fresh;
  };
  const refresh = async () => {
    if (locked.current) return;
    locked.current = true; setBusy(true);
    try { await reconcile(); setError(null); }
    catch (cause) { setError(cause); if (isApiError(cause) && [401, 403, 404].includes(cause.httpStatus)) onDenied(cause); }
    finally { locked.current = false; setBusy(false); }
  };
  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (locked.current || blocked || ime.isComposingRef.current || ime.justEndedRef.current) return;
    if (!intent.current) {
      const answer = multiple ? options!.filter(value => choices.includes(value)) : text;
      const length = typeof answer === "string" ? [...answer].length : answer.length;
      const valid = multiple ? length >= (schema.minItems ?? 0) && length <= schema.maxItems :
        "enum" in schema ? schema.enum.includes(text) : length >= (schema.minLength ?? 0) && length <= schema.maxLength;
      if (!valid) { setInvalid(true); return; }
      intent.current = { key: crypto.randomUUID(), body: { expected_revision: current.revision, question_version: 1, answer } };
    }
    locked.current = true; setBusy(true); setError(null); setInvalid(false);
    try {
      const answered = await api.agent.answerQuestion(current.run_id, current.id, intent.current.body, intent.current.key);
      setCurrent(answered); setText(""); setChoices([]); intent.current = null; setUncertain(false);
      await onRefresh();
    } catch (cause) {
      setError(cause);
      if (isApiError(cause) && [401, 403, 404].includes(cause.httpStatus)) { onDenied(cause); return; }
      const unknown = isNetworkError(cause) || (isApiError(cause) && (cause.httpStatus >= 500 || cause.code === "platform.operation_in_progress"));
      setUncertain(unknown && !!intent.current);
      if (!unknown) intent.current = null;
      if (isApiError(cause) && cause.httpStatus === 409 && cause.code !== "platform.operation_in_progress") {
        try { await reconcile(); } catch (refreshError) {
          setError(refreshError);
          if (isApiError(refreshError) && [401, 403, 404].includes(refreshError.httpStatus)) onDenied(refreshError);
        }
      }
    } finally { locked.current = false; setBusy(false); }
  };
  return <section className="rounded-xl border border-[var(--status-warning)] p-4 space-y-3 break-words" aria-labelledby={`question-${current.id}`} data-testid="agent-question">
    <h3 id={`question-${current.id}`} className="font-semibold">{label(current.title_key, t("agentQuestion.title"))}</h3>
    <p className="text-sm text-[var(--text-secondary)]">{label(current.help_key, t("agentQuestion.help"))}</p>
    {current.context_summary && <p className="whitespace-pre-wrap text-sm">{current.context_summary}</p>}
    <p className="text-xs text-[var(--text-muted)]">{t("agentQuestion.bound", current.source_revision, current.expires_at)}</p>
    {sourceChanged && <p role="alert">{t("agentQuestion.sourceChanged")}</p>}
    {closed ? <p role="status">{t(`agentQuestion.${expired && current.status === "open" ? "expired" : current.status}`)}</p> :
      <form onSubmit={submit} className="space-y-3" aria-busy={busy} onKeyDown={event => {
        if (event.key === "Enter" && ime.shouldBlockKeyAction(event)) { event.preventDefault(); event.stopPropagation(); }
      }}>
        <fieldset disabled={blocked || uncertain} className="space-y-2 min-w-0">
          <legend className="text-sm mb-2">{t(multiple ? "agentQuestion.multiple" : options ? "agentQuestion.single" : "agentQuestion.text")}</legend>
          {options ? options.map(value => <label key={value} className="flex items-start gap-2 rounded border border-[var(--border-decorative)] p-2 text-sm">
            <input className="mt-1 shrink-0" type={multiple ? "checkbox" : "radio"} name={`answer-${current.id}`} value={value}
              checked={multiple ? choices.includes(value) : text === value} onChange={event => {
                if (multiple) setChoices(previous => event.target.checked ? [...previous, value] : previous.filter(item => item !== value));
                else setText(value);
              }} />
            <span>{label(current.options.find(option => option.value === value)?.label_key ?? "", value)}</span>
          </label>) : <Textarea aria-label={t("agentQuestion.text")} rows={4} value={text}
            onChange={event => setText(event.target.value)} onCompositionStart={ime.handleCompositionStart} onCompositionEnd={ime.handleCompositionEnd} />}
        </fieldset>
        {invalid && <p role="alert" className="text-sm text-[var(--status-danger)]">{t("agentQuestion.invalid")}</p>}
        {uncertain && <p role="status" className="text-sm">{t("agentQuestion.unknown")}</p>}
        <Button type="submit" loading={busy} disabled={blocked}>{t(uncertain ? "agentQuestion.retry" : "agentQuestion.submit")}</Button>
      </form>}
    {error != null && <ErrorPresenter error={error} />}
    <Button size="sm" variant="ghost" disabled={busy || disabled} onClick={() => void refresh()}>{t("agentQuestion.refresh")}</Button>
  </section>;
}

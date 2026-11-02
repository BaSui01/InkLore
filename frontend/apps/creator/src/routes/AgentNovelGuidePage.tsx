import { useEffect, useRef, useState } from "react";
import { ArrowLeft, ArrowRight, Feather } from "lucide-react";
import { ApiError, type CreateNovelInput, type Novel } from "@inklore/contracts";
import { type ApiClient } from "@inklore/transport";
import { Button, isImeComposing } from "@inklore/ui";
import { useI18n } from "@inklore/i18n";
import { HomeNotice } from "../components/home/HomeNotice";

type Guide = { session_id: string; revision: number; status: string; initial_idea: string; turns: Array<{ question: string; suggestions: string[]; answer: string }>; current_question?: { text: string; suggestions: string[] }; proposal?: { title: string; category: string; summary: string; target_words: number } };
function storageKeys(client: ApiClient) {
  const scope = client.getAuthSession().getScope();
  const owner = scope ? `${scope.tenantId}.${scope.userId}.${scope.audience}` : "anonymous";
  const prefix = `inklore.novel-guide.${owner}`;
  return { session: `${prefix}.session`, startKey: `${prefix}.start-key`, startIdea: `${prefix}.start-idea` };
}

export function AgentNovelGuidePage({ initialIdea, client, offline, onBack, onCreated }: {
  initialIdea: string; client: ApiClient; offline: boolean;
  onBack: () => void; onCreated: (novel: Novel) => void;
}) {
  const { t } = useI18n();
  const keys = storageKeys(client);
  const lock = useRef(false);
  const autoStarted = useRef(false);
  const startAttempt = useRef<{ idea: string; key: string } | null>(null);
  const answerAttempt = useRef<{ request: string; key: string } | null>(null);
  const createAttempt = useRef<{ request: string; key: string } | null>(null);
  const [idea, setIdea] = useState(initialIdea);
  const [answer, setAnswer] = useState("");
  const [guide, setGuide] = useState<Guide | null>(null);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [title, setTitle] = useState("");
  const [category, setCategory] = useState("");
  const [description, setDescription] = useState("");
  const [target, setTarget] = useState("0");
  const [restoring, setRestoring] = useState(true);
  const [restoreAttempt, setRestoreAttempt] = useState(0);

  const call = async <T,>(operationId: string, method: "GET" | "POST", path: string, body?: unknown, key?: string) => {
    const result = await client.request<unknown, T>({ operationId, method, path, body, idempotencyKey: key });
    return result.data;
  };
  const hydrate = (value: Guide) => {
    setGuide(value);
    if (value.proposal) {
      setTitle(value.proposal.title); setCategory(value.proposal.category);
      setDescription(value.proposal.summary); setTarget(String(value.proposal.target_words));
    }
  };

  useEffect(() => {
    let active = true;
    setRestoring(true);
    const restore = async () => {
      setError(null);
      try {
        const id = localStorage.getItem(keys.session);
        if (id) {
          try {
            const result = await call<Guide>("agent_get_novel_guide", "GET", `/v1/agents/novel-guide/${id}`);
            if (active) hydrate(result);
            return;
          } catch (cause) {
            if (!(cause instanceof ApiError && cause.httpStatus === 404)) { if (active) setError(cause); return; }
            if (!active) return;
            localStorage.removeItem(keys.session);
            localStorage.removeItem(keys.startKey);
            localStorage.removeItem(keys.startIdea);
          }
        }
        const savedKey = localStorage.getItem(keys.startKey);
        const savedIdea = localStorage.getItem(keys.startIdea);
        if (savedKey && savedIdea) {
          try {
            const result = await call<Guide>("agent_start_novel_guide", "POST", "/v1/agents/novel-guide", { initial_idea: savedIdea }, savedKey);
            if (active) { localStorage.setItem(keys.session, result.session_id); localStorage.removeItem(keys.startKey); localStorage.removeItem(keys.startIdea); hydrate(result); }
            return;
          } catch (cause) { if (active) setError(cause); return; }
        }
        if (initialIdea) setIdea(initialIdea);
      } finally {
        if (active) setRestoring(false);
      }
    };
    void restore();
    return () => { active = false; };
  }, [initialIdea, client, keys.session, keys.startKey, keys.startIdea, restoreAttempt]);

  useEffect(() => {
    if (guide?.status !== "processing") return;
    const poll = window.setInterval(() => {
      void call<Guide>("agent_get_novel_guide", "GET", `/v1/agents/novel-guide/${guide.session_id}`)
        .then(hydrate).catch(cause => setError(cause));
    }, 1200);
    return () => window.clearInterval(poll);
  }, [guide?.session_id, guide?.status, client]);

  const start = async (candidate = idea) => {
    if (lock.current || offline || !candidate.trim()) return;
    lock.current = true; setPending(true); setError(null);
    try {
      const content = candidate.trim();
      if (startAttempt.current?.idea !== content) startAttempt.current = { idea: content, key: crypto.randomUUID() };
      localStorage.setItem(keys.startKey, startAttempt.current.key);
      localStorage.setItem(keys.startIdea, content);
      const value = await call<Guide>("agent_start_novel_guide", "POST", "/v1/agents/novel-guide", { initial_idea: content }, startAttempt.current.key);
      startAttempt.current = null; localStorage.removeItem(keys.startKey); localStorage.removeItem(keys.startIdea);
      localStorage.setItem(keys.session, value.session_id); hydrate(value);
    } catch (cause) { setError(cause); } finally { lock.current = false; setPending(false); }
  };

  useEffect(() => {
    if (restoring || autoStarted.current || guide || !initialIdea.trim() || offline || localStorage.getItem(keys.session) || localStorage.getItem(keys.startKey)) return;
    autoStarted.current = true;
    void start(initialIdea);
  }, [restoring, guide, initialIdea, offline, keys.session, keys.startKey]);

  const submitAnswer = async (value = answer) => {
    if (!guide || lock.current || offline || !value.trim()) return;
    lock.current = true; setPending(true); setError(null);
    try {
      const request = JSON.stringify([guide.session_id, guide.revision, value.trim()]);
      if (answerAttempt.current?.request !== request) answerAttempt.current = { request, key: crypto.randomUUID() };
      const next = await call<Guide>("agent_answer_novel_guide", "POST", `/v1/agents/novel-guide/${guide.session_id}/answers`, { expected_revision: guide.revision, answer: value.trim() }, answerAttempt.current.key);
      answerAttempt.current = null; setAnswer(""); hydrate(next);
    } catch (cause) { setError(cause); } finally { lock.current = false; setPending(false); }
  };

  const confirm = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!guide || lock.current || offline) return;
    const input: CreateNovelInput = { title: title.trim(), category: category.trim(), summary: description.trim(), target_words: Number(target) };
    if (!input.title || [...input.title].length > 120 || [...input.category!].length > 60 || [...input.summary!].length > 2000 || !Number.isSafeInteger(input.target_words) || input.target_words! < 0 || input.target_words! > 100000000) { setError(new Error(t("home.create.validation"))); return; }
    lock.current = true; setPending(true); setError(null);
    try {
      const request = JSON.stringify(input);
      if (createAttempt.current?.request !== request) createAttempt.current = { request, key: crypto.randomUUID() };
      const created = await call<Novel>("content_createNovel", "POST", "/v1/content/novels", input, createAttempt.current.key);
      createAttempt.current = null;
      localStorage.removeItem(keys.session); localStorage.removeItem(keys.startKey); localStorage.removeItem(keys.startIdea); onCreated(created);
    } catch (cause) { setError(cause); } finally { lock.current = false; setPending(false); }
  };

  const hasPendingGuide = !guide && (Boolean(localStorage.getItem(keys.session)) || Boolean(localStorage.getItem(keys.startKey)));

  return <div className="creator-home novel-guide-page">
    <header className="novel-guide-header home-container"><Button type="button" className="home-button home-button-secondary" onClick={onBack} icon={<ArrowLeft size={17} />}>{t("home.guide.back")}</Button><span className="home-footer-brand"><Feather size={17} />{t("home.brand")}</span></header>
    <main className="novel-guide-main" aria-labelledby="home-create-title">
      <div className="novel-guide-heading"><span className="home-kicker">{t("home.guide.eyebrow")}</span><h1 id="home-create-title">{t("home.create.title")}</h1><p>{t("home.create.subtitle")}</p></div>
      <form className="novel-guide-form" onSubmit={confirm} aria-busy={pending}>
        {error != null && <HomeNotice error={error} retry={hasPendingGuide ? () => setRestoreAttempt(value => value + 1) : undefined} busy={restoring} />}
        {offline && <p className="home-form-error" role="status">{t("home.offline")}</p>}
        {restoring && <p role="status">{t("home.loading")}</p>}
        {pending && !guide && <div className="home-guide-thread" aria-live="polite"><div className="home-guide-turn"><p><strong>{t("home.guide.yourIdea")}</strong></p><p>{idea}</p></div><p role="status">{t("home.guide.thinking")}</p></div>}
        {!guide && !restoring && !pending && !hasPendingGuide && <><label htmlFor="home-guide-idea">{t("home.ideaLabel")}</label><textarea id="home-guide-idea" rows={5} value={idea} onChange={event => setIdea(event.target.value)} placeholder={t("home.ideaPlaceholder")} />
          <p className="home-form-help">{t("home.guide.startHint")}</p><Button type="button" className="home-button home-button-primary" onClick={() => void start()} loading={pending} disabled={offline || !idea.trim()}>{t("home.guide.start")}<ArrowRight size={17} /></Button></>}
        {guide && <div className="home-guide-thread" aria-live="polite">
          {guide.turns.map((turn,index)=><div className="home-guide-turn" key={`${guide.session_id}-${index}`}><p><strong>{turn.question}</strong></p><p>{turn.answer}</p></div>)}
          {guide.status === "processing" && <p role="status">{t("home.guide.thinking")}</p>}
          {guide.status === "failed" && <section role="alert"><p>{t("home.guide.failed")}</p><Button type="button" className="home-button home-button-secondary" onClick={()=>{localStorage.removeItem(keys.session);localStorage.removeItem(keys.startKey);localStorage.removeItem(keys.startIdea);setGuide(null);setError(null);}}>{t("home.guide.restart")}</Button></section>}
          {guide.current_question && <section className="home-guide-question"><h3>{guide.current_question.text}</h3><div className="home-guide-suggestions">{guide.current_question.suggestions.map(option=><Button key={option} type="button" className="home-button home-button-secondary" disabled={pending} onClick={()=>void submitAnswer(option)}>{option}</Button>)}</div>
            <label htmlFor="home-guide-answer">{t("home.guide.yourAnswer")}</label><textarea id="home-guide-answer" rows={3} value={answer} onChange={event=>setAnswer(event.target.value)} disabled={pending} placeholder={t("home.guide.answerPlaceholder")} onKeyDown={event=>{if(isImeComposing(event))return;if((event.ctrlKey||event.metaKey)&&event.key==="Enter"){event.preventDefault();void submitAnswer();}}}/>
            <Button type="button" className="home-button home-button-primary" onClick={()=>void submitAnswer()} loading={pending} disabled={offline||!answer.trim()}>{t("home.guide.answer")}</Button></section>}
          {guide.proposal && <section className="home-guide-proposal"><h3>{t("home.guide.proposal")}</h3>
            <label htmlFor="home-novel-title">{t("home.create.name")}</label><input id="home-novel-title" value={title} onChange={event=>setTitle(event.target.value)} disabled={pending}/>
            <div className="home-form-row"><div><label htmlFor="home-category">{t("home.create.category")}</label><input id="home-category" value={category} onChange={event=>setCategory(event.target.value)} disabled={pending}/></div><div><label htmlFor="home-target">{t("home.create.target")}</label><input id="home-target" type="number" min="0" max="100000000" step="1" value={target} onChange={event=>setTarget(event.target.value)} disabled={pending}/></div></div>
            <label htmlFor="home-summary">{t("home.create.summary")}</label><textarea id="home-summary" rows={5} value={description} onChange={event=>setDescription(event.target.value)} disabled={pending}/>
            <Button type="submit" className="home-button home-button-primary" loading={pending} disabled={offline}>{t("home.guide.confirm")}</Button></section>}
        </div>}
      </form>
    </main>
  </div>;
}

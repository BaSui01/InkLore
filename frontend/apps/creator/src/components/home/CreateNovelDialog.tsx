import { useEffect, useRef, useState } from "react";
import { Feather, X } from "lucide-react";
import { ApiError, type CreateNovelInput, type Novel } from "@inklore/contracts";
import { type ApiClient } from "@inklore/transport";
import { Button, isImeComposing } from "@inklore/ui";
import { useI18n } from "@inklore/i18n";
import { HomeNotice } from "./HomeNotice";

export function CreateNovelDialog({ open, summary, client, offline, onClose, onCreated }: {
  open: boolean; summary: string; client: ApiClient; offline: boolean;
  onClose: () => void; onCreated: (novel: Novel) => void;
}) {
  const { t } = useI18n();
  const dialog = useRef<HTMLDialogElement>(null);
  const titleInput = useRef<HTMLInputElement>(null);
  const lock = useRef(false);
  const attempt = useRef<{ serialized: string; key: string } | null>(null);
  const [title, setTitle] = useState("");
  const [category, setCategory] = useState("");
  const [description, setDescription] = useState(summary);
  const [target, setTarget] = useState("");
  const [completionDate, setCompletionDate] = useState("");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [invalid, setInvalid] = useState(false);

  useEffect(() => {
    if (open) {
      setDescription(summary);
      if (!dialog.current?.open) dialog.current?.showModal();
      titleInput.current?.focus();
    } else dialog.current?.close();
  }, [open, summary]);

  const handleFormKeyDown = (event: React.KeyboardEvent) => {
    if (isImeComposing(event)) {
      if (event.key === "Enter") {
        event.stopPropagation();
      }
      return;
    }
    if ((event.ctrlKey || event.metaKey) && event.key === "Enter") {
      event.preventDefault();
      void submit(event);
    }
  };

  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (lock.current || offline) return;
    const input: CreateNovelInput = {
      title: title.trim(), summary: description.trim(), category: category.trim(),
      target_words: target === "" ? 0 : Number(target),
      target_completion_date: completionDate || null,
    };
    if ([...input.title].length < 1 || [...input.title].length > 120 ||
        [...input.summary!].length > 2000 || [...input.category!].length > 60 ||
        !Number.isSafeInteger(input.target_words) || input.target_words! < 0 || input.target_words! > 100000000) {
      setInvalid(true); titleInput.current?.focus(); return;
    }
    setInvalid(false); setError(null); lock.current = true; setPending(true);
    const serialized = JSON.stringify(input);
    if (attempt.current?.serialized !== serialized) attempt.current = { serialized, key: crypto.randomUUID() };
    try {
      const response = await client.request<CreateNovelInput, Novel>({
        operationId: "content_createNovel", method: "POST", path: "/v1/content/novels",
        body: input, idempotencyKey: attempt.current!.key,
      });
      onCreated(response.data);
    } catch (cause) {
      setError(cause);
      if (cause instanceof ApiError && cause.httpStatus >= 400 && cause.httpStatus < 500 && cause.httpStatus !== 409) {
        attempt.current = null;
      }
    } finally { lock.current = false; setPending(false); }
  };

  return <dialog ref={dialog} className="home-create-dialog" aria-labelledby="home-create-title"
    onCancel={event => { event.preventDefault(); if (!lock.current) onClose(); }}>
    <form onSubmit={submit} onKeyDown={handleFormKeyDown} aria-busy={pending}>
      <div className="home-dialog-heading"><span className="home-dialog-icon"><Feather size={22} /></span>
        <div><h2 id="home-create-title">{t("home.create.title")}</h2><p>{t("home.create.subtitle")}</p></div>
        <Button className="home-icon-button" variant="ghost" type="button" onClick={onClose} disabled={pending} aria-label={t("home.close")}><X size={20} /></Button>
      </div>
      <div className="home-dialog-fields">
        {invalid && <p className="home-form-error" role="alert" id="home-create-error">{t("home.create.validation")}</p>}
        {error != null && <HomeNotice error={error} />}
        {offline && <p className="home-form-error" role="status">{t("home.offline")}</p>}
        <label htmlFor="home-novel-title">{t("home.create.name")}<span>{t("home.create.required")}</span></label>
        <input ref={titleInput} id="home-novel-title" value={title} onChange={e => setTitle(e.target.value)} required disabled={pending}
          aria-invalid={invalid || undefined} aria-describedby={invalid ? "home-create-error" : undefined} placeholder={t("home.create.namePlaceholder")} />
        <div className="home-form-row"><div><label htmlFor="home-category">{t("home.create.category")}</label>
          <input id="home-category" value={category} disabled={pending} onChange={e => setCategory(e.target.value)} placeholder={t("home.create.categoryPlaceholder")} /></div>
          <div><label htmlFor="home-target">{t("home.create.target")}</label><input id="home-target" type="number" min="0" max="100000000" step="1" disabled={pending}
            value={target} onChange={e => setTarget(e.target.value)} placeholder={t("home.create.targetPlaceholder")} /></div></div>
        <label htmlFor="home-completion-date">{t("writing.targetDate")}</label>
        <input id="home-completion-date" type="date" min="0001-01-01" max="9999-12-31" value={completionDate} disabled={pending} onChange={e => setCompletionDate(e.target.value)} />
        <label htmlFor="home-summary">{t("home.create.summary")}<span>{t("home.create.summaryLimit")}</span></label>
        <textarea id="home-summary" rows={5} value={description} disabled={pending} onChange={e => setDescription(e.target.value)} placeholder={t("home.ideaPlaceholder")} />
        <p className="home-form-help">{t("home.create.help")}</p>
      </div>
      <div className="home-dialog-footer"><Button type="button" className="home-button home-button-secondary" onClick={onClose} disabled={pending}>{t("home.cancel")}</Button>
        <Button type="submit" className="home-button home-button-primary" loading={pending} disabled={offline}>{t("home.create.submit")}</Button></div>
    </form>
  </dialog>;
}
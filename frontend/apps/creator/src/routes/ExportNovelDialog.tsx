import React, { useEffect, useRef, useState } from "react";
import { Button, ErrorPresenter, Modal } from "@inklore/ui";
import { useTranslation } from "@inklore/i18n";
import { ContentAPI, NetworkError, generateUUID, isApiError, isNetworkError } from "@inklore/contracts";
import { getApiSDK } from "@inklore/transport";
import { Download } from "lucide-react";

type ExportInput = ContentAPI.components["schemas"]["CreateExport"];
type ExportJob = ContentAPI.components["schemas"]["Job"];

export function ExportNovelDialog({ open, novelId, onClose }: { open: boolean; novelId: string; onClose: () => void }) {
  const { t } = useTranslation();
  const api = getApiSDK();
  const [format, setFormat] = useState<ExportInput["format"]>("txt");
  const [includeDrafts, setIncludeDrafts] = useState(false);
  const [revision, setRevision] = useState<number | null>(null);
  const [jobs, setJobs] = useState<ExportJob[]>([]);
  const [loading, setLoading] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [uncertain, setUncertain] = useState(false);
  const intent = useRef<{ key: string; body: ExportInput } | null>(null);
  const busyRef = useRef(false);

  useEffect(() => {
    if (!open) return;
    let active = true;
    setLoading(true);
    Promise.all([api.content.getNovel(novelId), api.content.listJobs(novelId)])
      .then(([novel, history]) => {
        if (!active) return;
        setRevision(novel.revision);
        setJobs(history.filter((job) => job.kind === "export"));
      })
      .catch((reason) => { if (active) setError(reason); })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [open, novelId]);

  const download = async (job: ExportJob) => {
    if (!job.output_file_id) throw new Error(t("export.missingFile"));
    const result = await api.content.downloadExport(job.output_file_id);
    const url = URL.createObjectURL(result.blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = result.filename;
    link.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  };

  const create = async () => {
    if (busyRef.current || revision === null) return;
    busyRef.current = true;
    setBusy(true);
    setError(null);
    if (!intent.current) {
      intent.current = { key: generateUUID(), body: { expected_revision: revision, format, include_drafts: includeDrafts, volume_ids: [], review_ticket: null } };
    }
    try {
      const job = await api.content.createExport(novelId, intent.current.body, intent.current.key);
      if (!job || typeof job.id !== "string" || job.kind !== "export" || job.novel_id !== novelId || job.status !== "succeeded" || typeof job.output_file_id !== "string" || !job.output_file_id) {
        throw new NetworkError(t("export.missingFile"));
      }
      intent.current = null;
      setUncertain(false);
      setJobs((items) => [job, ...items.filter((item) => item.id !== job.id)]);
      await download(job);
    } catch (reason) {
      const unknown = isNetworkError(reason) || (isApiError(reason) && (reason.httpStatus >= 500 || reason.code === "platform.operation_in_progress"));
      setUncertain(!!intent.current && unknown);
      if (!unknown) intent.current = null;
      setError(reason);
    } finally {
      busyRef.current = false;
      setBusy(false);
    }
  };

  const downloadPrevious = async (job: ExportJob) => {
    if (busyRef.current) return;
    busyRef.current = true;
    setBusy(true);
    setError(null);
    try { await download(job); } catch (reason) { setError(reason); }
    finally { busyRef.current = false; setBusy(false); }
  };

  return (
    <Modal open={open} onClose={() => { if (!busy) onClose(); }} title={t("export.title")} footer={
      <Button onClick={() => void create()} loading={busy} disabled={loading || revision === null} icon={<Download className="h-4 w-4" />}>
        {uncertain ? t("export.retrySame") : t("export.create")}
      </Button>
    }>
      <div className="space-y-4 text-sm">
        <p className="text-[var(--text-secondary)]">{t("export.savedOnly")}</p>
        <label className="block space-y-2">
          <span>{t("export.format")}</span>
          <select className="w-full rounded-lg border border-[var(--border-default)] bg-[var(--bg-surface)] p-2" value={format} disabled={busy || uncertain} onChange={(event) => setFormat(event.target.value as ExportInput["format"])}>
            <option value="txt">TXT</option><option value="markdown">Markdown</option><option value="docx">DOCX</option><option value="epub">EPUB</option>
          </select>
        </label>
        <label className="flex items-center gap-2">
          <input type="checkbox" checked={includeDrafts} disabled={busy || uncertain} onChange={(event) => setIncludeDrafts(event.target.checked)} />
          {t("export.includeDrafts")}
        </label>
        {loading && <p role="status">{t("common.loading")}</p>}
        {uncertain && <p role="status">{t("export.unknown")}</p>}
        {error != null && <ErrorPresenter error={error} context={{ operationLabel: t("export.title") }} />}
        <section aria-label={t("export.history")} className="space-y-2">
          <h3 className="font-medium">{t("export.history")}</h3>
          {!loading && jobs.length === 0 && <p className="text-[var(--text-secondary)]">{t("export.empty")}</p>}
          {jobs.map((job) => (
            <div key={job.id} className="flex items-center justify-between gap-3 rounded-lg border border-[var(--border-default)] p-2">
              <span>{job.format.toUpperCase()} · {t("export.chapters", job.result_chapter_count)} · {new Date(job.created_at).toLocaleString()}</span>
              <Button variant="outline" className="shrink-0 whitespace-nowrap" disabled={busy || job.status !== "succeeded" || !job.output_file_id || Date.parse(job.expires_at) <= Date.now()} onClick={() => void downloadPrevious(job)}>{t("export.download")}</Button>
            </div>
          ))}
        </section>
      </div>
    </Modal>
  );
}

import { useEffect, useMemo, useState } from "react";
import { Edit3, KeyRound, Plus, Trash2 } from "lucide-react";
import { Button, ConfirmDialog, Input, Modal } from "@inklore/ui";
import { safeErrorText } from "@inklore/transport";
import { useTranslation } from "@inklore/i18n";
import { StatusBadge } from "../components/StatusBadge";
import type { LlmChannel, LlmKey } from "../api/adminApi";
import { createLlmKey, deleteLlmKey, updateLlmKey } from "../api/adminApi";

export function ChannelKeysModal({ open, channel, keys, onClose, onSaved }: {
  open: boolean;
  channel: LlmChannel | null;
  keys: LlmKey[];
  onClose: () => void;
  onSaved: () => Promise<void>;
}) {
  const { t } = useTranslation();
  const [editing, setEditing] = useState<LlmKey | null>(null);
  const [formOpen, setFormOpen] = useState(false);
  const [title, setTitle] = useState("");
  const [locator, setLocator] = useState("");
  const [secretVersion, setSecretVersion] = useState("v1");
  const [weight, setWeight] = useState("50");
  const [rpm, setRpm] = useState("600");
  const [concurrency, setConcurrency] = useState("20");
  const [enabled, setEnabled] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<LlmKey | null>(null);
  const [deleteError, setDeleteError] = useState<string | null>(null);
  const channelKeys = useMemo(() => keys.filter(key => key.channel_id === channel?.id), [keys, channel?.id]);

  useEffect(() => {
    if (!open) return;
    setEditing(null);
    setFormOpen(false);
    setError(null);
    setDeleteTarget(null);
    setDeleteError(null);
  }, [open, channel?.id]);

  const beginCreate = () => {
    setEditing(null);
    setTitle(channel ? `${channel.title}-key` : "");
    setLocator("");
    setSecretVersion("v1");
    setWeight(String(channel?.weight ?? 50));
    setRpm(String(channel?.rpm ?? 600));
    setConcurrency(String(channel?.concurrency ?? 20));
    setEnabled(true);
    setError(null);
    setFormOpen(true);
  };

  const beginEdit = (key: LlmKey) => {
    setEditing(key);
    setTitle(key.title);
    setLocator(key.secret_ref.kind === "env" && /^[A-Z_][A-Z0-9_]*$/.test(key.secret_ref.locator) ? key.secret_ref.locator : "");
    setSecretVersion(key.secret_ref.version);
    setWeight(String(key.weight));
    setRpm(String(key.rpm));
    setConcurrency(String(key.concurrency));
    setEnabled(key.enabled);
    setError(null);
    setFormOpen(true);
  };

  const save = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!channel || busy) return;
    const parsedWeight = Number(weight);
    const parsedRpm = Number(rpm);
    const parsedConcurrency = Number(concurrency);
    const trimmedLocator = locator.trim();
    const isEnvLocator = /^[A-Z_][A-Z0-9_]*$/.test(trimmedLocator);
    if (!title.trim() || title.trim().length > 120 || !isEnvLocator || trimmedLocator.length > 256 ||
        !secretVersion.trim() || secretVersion.trim().length > 64 ||
        !Number.isInteger(parsedWeight) || parsedWeight < 1 || parsedWeight > 1000 ||
        !Number.isInteger(parsedRpm) || parsedRpm < 1 || parsedRpm > 100000 ||
        !Number.isInteger(parsedConcurrency) || parsedConcurrency < 1 || parsedConcurrency > 1000) {
      setError(t("admin.channel_keys.validation"));
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const payload = {
        channel_id: channel.id,
        title: title.trim(),
        secret_ref: { kind: "env" as const, locator: trimmedLocator, version: secretVersion.trim() },
        weight: parsedWeight,
        rpm: parsedRpm,
        concurrency: parsedConcurrency,
        enabled,
      };
      if (editing) await updateLlmKey(editing.id, { ...payload, expected_revision: editing.revision });
      else await createLlmKey(payload);
      await onSaved();
      setFormOpen(false);
      setEditing(null);
    } catch (cause) {
      setError(safeErrorText(cause, t));
    } finally {
      setBusy(false);
    }
  };

  const removeKey = async () => {
    if (!deleteTarget || busy) return;
    setBusy(true);
    setDeleteError(null);
    try {
      await deleteLlmKey(deleteTarget.id, deleteTarget.revision);
      setDeleteTarget(null);
      await onSaved();
    } catch (cause) {
      setDeleteError(safeErrorText(cause, t));
    } finally {
      setBusy(false);
    }
  };

  return <>
    <Modal open={open && !!channel} onClose={() => !busy && onClose()} maxWidth="2xl"
      title={<span className="inline-flex items-center gap-2"><KeyRound className="h-4 w-4" />{t("admin.channel_keys.title")} · {channel?.title}</span>}>
      <div className="space-y-4">
        {!formOpen && <div className="flex justify-end"><Button size="sm" variant="primary" icon={<Plus size={15} />} onClick={beginCreate}>{t("admin.channel_keys.add")}</Button></div>}
        {formOpen && <form onSubmit={save} className="space-y-3 rounded-lg border border-[var(--border-decorative)] bg-[var(--bg-subtle)]/40 p-4">
          <h3 className="font-semibold text-sm">{t(editing ? "admin.channel_keys.edit_title" : "admin.channel_keys.create_title")}</h3>
          {error && <p role="alert" className="rounded-md border border-[var(--status-danger)]/30 bg-[var(--status-danger)]/10 p-3 text-sm text-[var(--status-danger)]">{error}</p>}
          <div className="grid gap-3 sm:grid-cols-2">
            <label className="space-y-1 text-xs"><span>{t("admin.channel_keys.name")}</span><Input value={title} onChange={event => setTitle(event.target.value)} disabled={busy} /></label>
            <label className="space-y-1 text-xs"><span>{t("admin.channel_keys.env_ref")}</span><Input value={locator} onChange={event => setLocator(event.target.value)} disabled={busy} className="font-mono" /></label>
            <label className="space-y-1 text-xs"><span>{t("admin.channel_keys.version")}</span><Input value={secretVersion} onChange={event => setSecretVersion(event.target.value)} disabled={busy} /></label>
            <label className="space-y-1 text-xs"><span>{t("admin.channel_keys.weight")}</span><Input type="number" min="1" max="1000" value={weight} onChange={event => setWeight(event.target.value)} disabled={busy} /></label>
            <label className="space-y-1 text-xs"><span>{t("admin.channel_keys.rpm")}</span><Input type="number" min="1" value={rpm} onChange={event => setRpm(event.target.value)} disabled={busy} /></label>
            <label className="space-y-1 text-xs"><span>{t("admin.channel_keys.concurrency")}</span><Input type="number" min="1" value={concurrency} onChange={event => setConcurrency(event.target.value)} disabled={busy} /></label>
            <label className="flex items-center gap-2 pt-5 text-xs"><input type="checkbox" checked={enabled} onChange={event => setEnabled(event.target.checked)} disabled={busy} /><span>{t("admin.channel_keys.enabled")}</span></label>
          </div>
          <p className="text-xs text-[var(--text-muted)]">{t("admin.channel_keys.env_hint")}</p>
          <div className="flex justify-end gap-2 border-t border-[var(--border-decorative)] pt-3">
            <Button type="button" size="sm" variant="outline" disabled={busy} onClick={() => { setFormOpen(false); setEditing(null); }}>{t("common.cancel")}</Button>
            <Button type="submit" size="sm" variant="primary" loading={busy}>{t("admin.channel_keys.save")}</Button>
          </div>
        </form>}
        {!formOpen && channelKeys.length === 0 && <p className="py-8 text-center text-sm text-[var(--text-muted)]">{t("admin.channel_keys.empty")}</p>}
        {channelKeys.length > 0 && <div className="overflow-x-auto rounded-lg border border-[var(--border-decorative)]">
          <table className="w-full text-left text-xs">
            <thead className="bg-[var(--bg-subtle)] text-[var(--text-secondary)]"><tr><th className="p-3">{t("admin.channel_keys.name")}</th><th className="p-3">{t("admin.channel_keys.env_ref")}</th><th className="p-3">{t("admin.channel_keys.health")}</th><th className="p-3 text-right">{t("admin.channel_keys.actions")}</th></tr></thead>
            <tbody className="divide-y divide-[var(--border-decorative)]">{channelKeys.map(key => <tr key={key.id}>
              <td className="p-3 font-medium">{key.title}</td><td className="p-3 font-mono text-[var(--text-muted)]">{key.secret_ref.locator}</td><td className="p-3"><StatusBadge status={key.enabled ? (key.health === "healthy" ? "healthy" : key.health === "degraded" || key.health === "unknown" ? "cooling" : "open") : "disabled"} label={t(`admin.channel_keys.health_${key.health}`)} /></td>
              <td className="p-3"><div className="flex justify-end gap-1"><Button size="icon" variant="ghost" title={t("common.edit")} aria-label={t("common.edit")} onClick={() => beginEdit(key)}><Edit3 size={15} /></Button><Button size="icon" variant="ghost" title={t("common.delete")} aria-label={t("common.delete")} onClick={() => { setDeleteError(null); setDeleteTarget(key); }}><Trash2 size={15} /></Button></div></td>
            </tr>)}</tbody>
          </table>
        </div>}
      </div>
    </Modal>
    <ConfirmDialog open={!!deleteTarget} title={t("admin.channel_keys.delete_title")}
      description={<div className="space-y-2"><p>{t("admin.channel_keys.confirm_delete", deleteTarget?.title || "")}</p>{deleteError && <p role="alert" className="text-[var(--status-danger)]">{deleteError}</p>}</div>}
      confirmText={t("common.delete")} cancelText={t("common.cancel")} variant="danger" loading={busy}
      onClose={() => !busy && setDeleteTarget(null)} onConfirm={removeKey} />
  </>;
}

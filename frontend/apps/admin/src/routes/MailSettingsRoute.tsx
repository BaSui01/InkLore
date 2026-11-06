import { useCallback, useEffect, useRef, useState } from "react";
import { Mail, RefreshCw } from "lucide-react";
import { Button, Card, Input } from "@inklore/ui";
import { useTranslation } from "@inklore/i18n";
import { ApiError } from "@inklore/contracts";
import { useAdminSession } from "../context/AdminSessionContext";
import { AdminHeader } from "../components/AdminHeader";
import { AdminNotice } from "../components/AdminNotice";
import { getMailSettings, saveMailSettings, testMailSettings, MailSettings, MailSettingsInput } from "../api/mailApi";

const selectStyle = "w-full rounded-lg border border-[var(--border-control)] bg-[var(--bg-surface)] px-3 py-2 text-sm text-[var(--text-primary)]";

export function MailSettingsRoute() {
  const { t } = useTranslation();
  const { snapshot } = useAdminSession();
  const allowed = snapshot.principal?.global_roles?.includes("super_admin") === true;
  const [settings, setSettings] = useState<MailSettings | null>(null);
  const [form, setForm] = useState<MailSettingsInput | null>(null);
  const [credentialMode, setCredentialMode] = useState<"keep" | "encrypted" | "env">("encrypted");
  const [credential, setCredential] = useState("");
  const [busy, setBusy] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const saveKey = useRef<string | null>(null);
  const testKey = useRef<string | null>(null);
  const applySettings = useCallback((value: MailSettings) => {
    setSettings(value);
    setForm({ provider: value.provider, enabled: value.enabled, smtp_host: value.smtp_host,
      smtp_port: value.smtp_port || 465, smtp_security: value.smtp_security || "implicit_tls",
      smtp_username: value.smtp_username, sender_email: value.sender_email, sender_name: value.sender_name,
      expected_revision: value.revision });
    setCredentialMode(value.has_credential ? "keep" : "encrypted");
    setCredential("");
    saveKey.current = null;
  }, []);
  const load = useCallback(async () => {
    if (!allowed) return;
    setLoading(true); setError(null);
    try { applySettings(await getMailSettings()); }
    catch (err) { setError(err); }
    finally { setLoading(false); }
  }, [allowed, applySettings]);
  useEffect(() => {
    setSettings(null); setForm(null); setCredential(""); testKey.current = null;
    void load();
  }, [load, snapshot.sessionEpoch]);
  const dismiss = useCallback(() => { setError(null); setSuccess(null); }, []);
  const edit = (patch: Partial<MailSettingsInput>) => {
    setForm((previous) => previous ? { ...previous, ...patch } : previous);
    saveKey.current = null;
  };
  const save = async (event: React.FormEvent) => {
    event.preventDefault(); if (!form) return;
    setBusy(true); setError(null); setSuccess(null);
    const input: MailSettingsInput = { ...form };
    if (credentialMode === "encrypted" && credential) input.credential = { kind: "encrypted", value: credential };
    if (credentialMode === "env" && credential) input.credential = { kind: "env", locator: credential };
    saveKey.current ||= crypto.randomUUID();
    try { applySettings(await saveMailSettings(input, saveKey.current)); testKey.current = null; setSuccess(t("mail.saved")); }
    catch (err) { setError(err); }
    finally { setBusy(false); }
  };
  const test = async () => {
    if (!settings) return;
    setBusy(true); setError(null); setSuccess(null); testKey.current ||= crypto.randomUUID();
    try {
      const result = await testMailSettings(settings.revision, testKey.current);
      if (!result.accepted) throw new Error(t("mail.not_accepted"));
      testKey.current = null; setSuccess(t("mail.test_accepted"));
    } catch (err) {
      if (err instanceof ApiError && err.httpStatus === 503) testKey.current = null;
      setError(err);
    } finally { setBusy(false); }
  };

  return <div className="flex h-full min-h-0 flex-col">
    <AdminHeader title={t("mail.title")} subtitle={t("mail.subtitle")} actions={allowed && <Button variant="outline" onClick={() => void load()} disabled={busy || loading}><RefreshCw className="mr-2 h-4 w-4" />{t("common.refresh")}</Button>} />
    <AdminNotice error={error} success={success} onDismiss={dismiss} />
    {!allowed ? <p className="p-6 text-sm text-[var(--text-secondary)]" role="alert">{t("mail.super_admin_only")}</p> : <div className="flex-1 overflow-y-auto p-6">
      {loading && <p className="text-sm text-[var(--text-muted)]">{t("common.loading")}</p>}
      {form && settings && <form onSubmit={save} className="mx-auto max-w-3xl space-y-5">
        <Card className="space-y-5 p-5">
          <h3 className="flex items-center gap-2 font-semibold"><Mail className="h-5 w-5" />{t("mail.transport")}</h3>
          <p className="text-sm text-[var(--text-secondary)]">{t("mail.scope_hint")}</p>
          <fieldset disabled={busy} className="space-y-4">
            <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={form.enabled} onChange={(e) => edit({ enabled: e.target.checked })} />{t("mail.enabled")}</label>
            <label className="block space-y-1 text-sm">{t("mail.provider")}
              <select className={selectStyle} value={form.provider} onChange={(e) => { edit({ provider: e.target.value as MailSettingsInput["provider"] }); setCredentialMode("encrypted"); setCredential(""); }}>
                <option value="disabled" disabled>{t("mail.unconfigured")}</option><option value="smtp">SMTP</option>
              </select>
            </label>
            {form.provider === "smtp" && <div className="grid gap-4 sm:grid-cols-2">
              <label className="space-y-1 text-sm">{t("mail.smtp_host")}<Input value={form.smtp_host} maxLength={253} required={form.enabled} onChange={(e) => edit({ smtp_host: e.target.value })} /></label>
              <label className="space-y-1 text-sm">{t("mail.security")}<select className={selectStyle} value={form.smtp_security} onChange={(e) => edit({ smtp_security: e.target.value as "implicit_tls" | "starttls", smtp_port: e.target.value === "implicit_tls" ? 465 : 587 })}><option value="implicit_tls">{t("mail.implicit_tls")}</option><option value="starttls">{t("mail.starttls")}</option></select></label>
              <label className="space-y-1 text-sm sm:col-span-2">{t("mail.smtp_username")}<Input value={form.smtp_username} autoComplete="off" maxLength={254} required={form.enabled} onChange={(e) => edit({ smtp_username: e.target.value })} /></label>
            </div>}
            <div className="grid gap-4 sm:grid-cols-2">
              <label className="space-y-1 text-sm">{t("mail.sender_email")}<Input type="email" value={form.sender_email} maxLength={254} required={form.enabled} onChange={(e) => edit({ sender_email: e.target.value })} /></label>
              <label className="space-y-1 text-sm">{t("mail.sender_name")}<Input value={form.sender_name} maxLength={120} onChange={(e) => edit({ sender_name: e.target.value })} /></label>
            </div>
          </fieldset>
        </Card>
        <Card className="space-y-4 p-5">
          <h3 className="font-semibold">{t("mail.credential")}</h3>
          <p className="text-sm text-[var(--text-secondary)]">{settings.has_credential ? t("mail.credential_present") : t("mail.credential_absent")}</p>
          <fieldset disabled={busy} className="space-y-4">
            <label className="block space-y-1 text-sm">{t("mail.credential_action")}<select className={selectStyle} value={credentialMode} onChange={(e) => { setCredentialMode(e.target.value as typeof credentialMode); setCredential(""); saveKey.current = null; }}>
              {settings.has_credential && form.provider === settings.provider && <option value="keep">{t("mail.keep_credential")}</option>}<option value="encrypted">{t("mail.encrypted_credential")}</option><option value="env">{t("mail.env_credential")}</option>
            </select></label>
            {credentialMode !== "keep" && <label className="block space-y-1 text-sm">{credentialMode === "env" ? t("mail.env_locator") : t("mail.new_secret")}<Input type={credentialMode === "env" ? "text" : "password"} value={credential} autoComplete="new-password" required={form.enabled} maxLength={credentialMode === "env" ? 96 : 8192} onChange={(e) => { setCredential(e.target.value); saveKey.current = null; }} /></label>}
            <p className="text-xs text-[var(--text-muted)]">{t("mail.secret_hint")}</p>
          </fieldset>
          <div className="flex flex-wrap items-center gap-3"><Button type="submit" disabled={busy || loading}>{busy ? t("auth.submitting") : t("common.save")}</Button><span className="text-xs text-[var(--text-muted)]">{t("mail.revision", settings.revision)}</span></div>
        </Card>
        <Card className="space-y-3 p-5"><h3 className="font-semibold">{t("mail.test_title")}</h3><p className="text-sm text-[var(--text-secondary)]">{t("mail.test_hint")}</p><Button type="button" variant="outline" disabled={!settings.enabled || busy} onClick={() => void test()}>{t("mail.test_send")}</Button></Card>
      </form>}
    </div>}
  </div>;
}

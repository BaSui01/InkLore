import { adminClient } from "./adminApi";

export interface MailSettings {
  provider: "disabled" | "smtp" | "resend";
  enabled: boolean;
  smtp_host: string;
  smtp_port: number;
  smtp_security: "" | "implicit_tls" | "starttls";
  smtp_username: string;
  sender_email: string;
  sender_name: string;
  has_credential: boolean;
  credential_kind: "" | "encrypted" | "env";
  revision: number;
  updated_at: string;
}
export type MailSettingsInput = Omit<MailSettings, "has_credential" | "credential_kind" | "revision" | "updated_at"> & {
  expected_revision: number;
  credential?: { kind: "encrypted"; value: string } | { kind: "env"; locator: string };
};
export async function getMailSettings(): Promise<MailSettings> {
  return (await adminClient.request<undefined, MailSettings>({ operationId: "platformGetMailSettings", method: "GET", path: "/v1/platform/mail-settings" })).data;
}
export async function saveMailSettings(input: MailSettingsInput, key: string): Promise<MailSettings> {
  return (await adminClient.request<MailSettingsInput, MailSettings>({ operationId: "platformUpdateMailSettings", method: "PUT", path: "/v1/platform/mail-settings", body: input, idempotencyKey: key })).data;
}
export async function testMailSettings(revision: number, key: string): Promise<{ accepted: boolean; delivery_id: string }> {
  return (await adminClient.request<{ expected_revision: number }, { accepted: boolean; delivery_id: string }>({ operationId: "platformTestMailSettings", method: "POST", path: "/v1/platform/mail-settings/test", body: { expected_revision: revision }, idempotencyKey: key })).data;
}

/**
 * @inklore/admin-console - 成员治理相关模态框集合 (UserManagementModals)
 * 严格遵循 Zero Hardcoding Contract、No Emoji Policy 与零硬编码色彩主题
 */

import React, { useState } from "react";
import { Check, Shield, Layers } from "lucide-react";
import { Modal, Button, Input } from "@inklore/ui";
import { useTranslation } from "@inklore/i18n";
import { PlatformMember, createPlatformMember, updatePlatformMember, resetMemberPassword } from "../api/adminApi";

interface CreateMemberModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: (msg: string) => void;
}

export function CreateMemberModal({ open, onClose, onSuccess }: CreateMemberModalProps) {
  const { t } = useTranslation();
  const [email, setEmail] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [password, setPassword] = useState("");
  const [roles, setRoles] = useState<string[]>(["creator"]);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const toggleRole = (roleKey: string) => {
    setRoles((prev) => {
      if (prev.includes(roleKey)) {
        if (prev.length <= 1) {
          setError("至少需要分配一个系统角色。");
          return prev;
        }
        setError(null);
        return prev.filter((r) => r !== roleKey);
      } else {
        setError(null);
        return [...prev, roleKey];
      }
    });
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!email.trim() || !displayName.trim() || !password.trim()) {
      setError("请完整填写所有必填字段。");
      return;
    }
    if (password.length < 8) {
      setError(t("admin.users.form_password_hint"));
      return;
    }
    if (roles.length === 0) {
      setError("至少需要分配一个系统角色。");
      return;
    }

    setSubmitting(true);
    setError(null);
    try {
      await createPlatformMember({
        email: email.trim(),
        display_name: displayName.trim(),
        password,
        roles,
      });
      onSuccess(t("admin.users.success_created"));
      onClose();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal open={open} onClose={() => !submitting && onClose()} title={t("admin.users.modal_create_title")}>
      <form onSubmit={handleSubmit} className="space-y-4 text-xs">
        {error && (
          <div className="p-2.5 rounded-lg border border-red-500/30 bg-red-500/10 text-red-500 text-xs">
            {error}
          </div>
        )}

        <div className="space-y-1">
          <label className="font-semibold text-[var(--text-primary)]">
            {t("admin.users.form_email")} *
          </label>
          <Input
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            placeholder="author@example.com"
            required
            className="w-full text-xs"
          />
        </div>

        <div className="space-y-1">
          <label className="font-semibold text-[var(--text-primary)]">
            {t("admin.users.form_display_name")} *
          </label>
          <Input
            type="text"
            value={displayName}
            onChange={(e) => setDisplayName(e.target.value)}
            placeholder="例如: 签约作者-墨白"
            required
            className="w-full text-xs"
          />
        </div>

        <div className="space-y-1">
          <label className="font-semibold text-[var(--text-primary)]">
            {t("admin.users.form_password")} *
          </label>
          <Input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder={t("admin.users.form_password_hint")}
            required
            className="w-full text-xs"
          />
        </div>

        <div className="space-y-1.5">
          <label className="font-semibold text-[var(--text-primary)]">
            {t("admin.users.form_roles")} *
          </label>
          <div className="grid grid-cols-2 gap-2 pt-1">
            <button
              type="button"
              onClick={() => toggleRole("creator")}
              className={`p-2.5 rounded-lg border text-xs font-medium text-center transition-all flex items-center justify-center gap-1.5 ${
                roles.includes("creator")
                  ? "bg-[var(--action-primary)] border-[var(--action-primary)] text-[var(--action-on-primary)] shadow-xs"
                  : "bg-[var(--bg-canvas)] border-[var(--border-decorative)] text-[var(--text-secondary)] hover:border-[var(--text-muted)]"
              }`}
            >
              <Check className={`w-3.5 h-3.5 ${roles.includes("creator") ? "opacity-100" : "opacity-0"}`} />
              <Layers className="w-3.5 h-3.5" />
              <span>{t("admin.users.role_creator")}</span>
            </button>
            <button
              type="button"
              onClick={() => toggleRole("operator")}
              className={`p-2.5 rounded-lg border text-xs font-medium text-center transition-all flex items-center justify-center gap-1.5 ${
                roles.includes("operator")
                  ? "bg-[var(--action-primary)] border-[var(--action-primary)] text-[var(--action-on-primary)] shadow-xs"
                  : "bg-[var(--bg-canvas)] border-[var(--border-decorative)] text-[var(--text-secondary)] hover:border-[var(--text-muted)]"
              }`}
            >
              <Check className={`w-3.5 h-3.5 ${roles.includes("operator") ? "opacity-100" : "opacity-0"}`} />
              <Shield className="w-3.5 h-3.5" />
              <span>{t("admin.users.role_operator")}</span>
            </button>
          </div>
        </div>

        <div className="flex items-center justify-end gap-2 pt-3 border-t border-[var(--border-decorative)]">
          <Button type="button" variant="outline" size="sm" onClick={onClose} disabled={submitting}>
            {t("common.cancel")}
          </Button>
          <Button type="submit" variant="primary" size="sm" loading={submitting}>
            {t("common.confirm")}
          </Button>
        </div>
      </form>
    </Modal>
  );
}

interface EditMemberModalProps {
  open: boolean;
  member: PlatformMember | null;
  isSelf?: boolean;
  isLastOperator?: boolean;
  onClose: () => void;
  onSuccess: (msg: string) => void;
}

export function EditMemberModal({
  open,
  member,
  isSelf = false,
  isLastOperator = false,
  onClose,
  onSuccess,
}: EditMemberModalProps) {
  const { t } = useTranslation();
  const [roles, setRoles] = useState<string[]>(() => (member ? [...member.roles] : []));
  const [status, setStatus] = useState<"active" | "disabled">(() => (member ? member.status : "active"));
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  React.useEffect(() => {
    if (member) {
      setRoles([...member.roles]);
      setStatus(member.status);
      setError(null);
    }
  }, [member]);

  if (!member) return null;

  const cannotDisable = isSelf || isLastOperator;
  const disableTooltip = isSelf
    ? t("admin.users.cannot_disable_self")
    : isLastOperator
    ? t("admin.users.last_operator_disable_forbidden")
    : undefined;

  const toggleRole = (roleKey: string) => {
    if (roleKey === "operator" && roles.includes("operator")) {
      if (isLastOperator) {
        setError(t("admin.users.last_operator_role_forbidden"));
        return;
      }
      if (isSelf) {
        setError(t("admin.users.cannot_remove_self_operator"));
        return;
      }
    }
    setRoles((prev) => {
      if (prev.includes(roleKey)) {
        if (prev.length <= 1) {
          setError("至少需要保留一个有效系统角色。");
          return prev;
        }
        setError(null);
        return prev.filter((r) => r !== roleKey);
      } else {
        setError(null);
        return [...prev, roleKey];
      }
    });
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (roles.length === 0) {
      setError("至少需要保留一个有效系统角色。");
      return;
    }
    if (cannotDisable && status === "disabled") {
      setError(disableTooltip || "无法停用该账号。");
      return;
    }
    if (isLastOperator && !roles.includes("operator")) {
      setError(t("admin.users.last_operator_role_forbidden"));
      return;
    }
    if (isSelf && !roles.includes("operator")) {
      setError(t("admin.users.cannot_remove_self_operator"));
      return;
    }

    setSubmitting(true);
    setError(null);
    try {
      await updatePlatformMember(member.id, {
        expected_revision: member.revision,
        roles,
        status,
      });
      onSuccess(t("admin.users.success_updated"));
      onClose();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal open={open} onClose={() => !submitting && onClose()} title={t("admin.users.modal_edit_title")}>
      <form onSubmit={handleSubmit} className="space-y-4 text-xs">
        {error && (
          <div className="p-2.5 rounded-lg border border-red-500/30 bg-red-500/10 text-red-500 text-xs">
            {error}
          </div>
        )}

        <div className="p-3 rounded-lg bg-[var(--bg-subtle)] space-y-1">
          <div className="font-semibold text-[var(--text-primary)]">{member.display_name}</div>
          <div className="text-[11px] font-mono text-[var(--text-muted)]">{member.email}</div>
        </div>

        <div className="space-y-1.5">
          <label className="font-semibold text-[var(--text-primary)]">
            {t("admin.users.form_roles")}
          </label>
          <div className="grid grid-cols-2 gap-2 pt-1">
            <button
              type="button"
              onClick={() => toggleRole("creator")}
              className={`p-2.5 rounded-lg border text-xs font-medium text-center transition-all flex items-center justify-center gap-1.5 ${
                roles.includes("creator")
                  ? "bg-[var(--action-primary)] border-[var(--action-primary)] text-[var(--action-on-primary)] shadow-xs"
                  : "bg-[var(--bg-canvas)] border-[var(--border-decorative)] text-[var(--text-secondary)] hover:border-[var(--text-muted)]"
              }`}
            >
              <Check className={`w-3.5 h-3.5 ${roles.includes("creator") ? "opacity-100" : "opacity-0"}`} />
              <Layers className="w-3.5 h-3.5" />
              <span>{t("admin.users.role_creator")}</span>
            </button>
            <button
              type="button"
              onClick={() => toggleRole("operator")}
              className={`p-2.5 rounded-lg border text-xs font-medium text-center transition-all flex items-center justify-center gap-1.5 ${
                roles.includes("operator")
                  ? "bg-[var(--action-primary)] border-[var(--action-primary)] text-[var(--action-on-primary)] shadow-xs"
                  : "bg-[var(--bg-canvas)] border-[var(--border-decorative)] text-[var(--text-secondary)] hover:border-[var(--text-muted)]"
              }`}
              title={
                isLastOperator
                  ? t("admin.users.last_operator_role_forbidden")
                  : isSelf
                  ? t("admin.users.cannot_remove_self_operator")
                  : undefined
              }
            >
              <Check className={`w-3.5 h-3.5 ${roles.includes("operator") ? "opacity-100" : "opacity-0"}`} />
              <Shield className="w-3.5 h-3.5" />
              <span>{t("admin.users.role_operator")}</span>
            </button>
          </div>
        </div>

        <div className="space-y-1.5">
          <label className="font-semibold text-[var(--text-primary)]">
            {t("admin.users.form_status")}
          </label>
          <div className="flex items-center gap-4">
            <label className="flex items-center gap-1.5 cursor-pointer">
              <input
                type="radio"
                name="status"
                value="active"
                checked={status === "active"}
                onChange={() => setStatus("active")}
              />
              <span>{t("admin.users.status_active")}</span>
            </label>
            <label
              className={`flex items-center gap-1.5 ${cannotDisable ? "cursor-not-allowed opacity-50" : "cursor-pointer"}`}
              title={disableTooltip}
            >
              <input
                type="radio"
                name="status"
                value="disabled"
                checked={status === "disabled"}
                disabled={cannotDisable}
                onChange={() => !cannotDisable && setStatus("disabled")}
              />
              <span>{t("admin.users.status_disabled")}</span>
              {cannotDisable && (
                <span className="text-[10px] text-amber-600 dark:text-amber-400 font-mono">
                  ({disableTooltip})
                </span>
              )}
            </label>
          </div>
        </div>

        <div className="flex items-center justify-end gap-2 pt-3 border-t border-[var(--border-decorative)]">
          <Button type="button" variant="outline" size="sm" onClick={onClose} disabled={submitting}>
            {t("common.cancel")}
          </Button>
          <Button type="submit" variant="primary" size="sm" loading={submitting}>
            {t("common.save")}
          </Button>
        </div>
      </form>
    </Modal>
  );
}

interface ResetPasswordModalProps {
  open: boolean;
  member: PlatformMember | null;
  onClose: () => void;
  onSuccess: (msg: string) => void;
}

export function ResetPasswordModal({ open, member, onClose, onSuccess }: ResetPasswordModalProps) {
  const { t } = useTranslation();
  const [newPassword, setNewPassword] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (!member) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (newPassword.length < 8) {
      setError("新密码长度必须至少为 8 位字符。");
      return;
    }

    setSubmitting(true);
    setError(null);
    try {
      await resetMemberPassword(member.id, {
        expected_revision: member.revision,
        new_password: newPassword,
      });
      onSuccess(t("admin.users.success_pwd_reset"));
      onClose();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal open={open} onClose={() => !submitting && onClose()} title={t("admin.users.modal_reset_pwd_title")}>
      <form onSubmit={handleSubmit} className="space-y-4 text-xs">
        {error && (
          <div className="p-2.5 rounded-lg border border-red-500/30 bg-red-500/10 text-red-500 text-xs">
            {error}
          </div>
        )}

        <p className="text-[var(--text-secondary)]">
          {t("admin.users.modal_reset_pwd_desc")} ({member.display_name} - {member.email})
        </p>

        <div className="space-y-1">
          <label className="font-semibold text-[var(--text-primary)]">
            {t("admin.users.form_new_password")}
          </label>
          <Input
            type="password"
            value={newPassword}
            onChange={(e) => setNewPassword(e.target.value)}
            placeholder={t("admin.users.form_password_hint")}
            required
            className="w-full text-xs"
          />
        </div>

        <div className="flex items-center justify-end gap-2 pt-3 border-t border-[var(--border-decorative)]">
          <Button type="button" variant="outline" size="sm" onClick={onClose} disabled={submitting}>
            {t("common.cancel")}
          </Button>
          <Button type="submit" variant="primary" size="sm" loading={submitting}>
            {t("common.confirm")}
          </Button>
        </div>
      </form>
    </Modal>
  );
}

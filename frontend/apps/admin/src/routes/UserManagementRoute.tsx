/**
 * @inklore/admin-console - 用户治理与计费规则管理路由 (UserManagementRoute)
 * 严格遵循 Zero Hardcoding Contract、No Emoji Policy 与零硬编码色彩主题
 * 涵盖租户成员 CRUD (增删改查/角色配置/状态切换/重置密码/会话吊销) 与大模型计费规则/日预算配额
 */

import React, { useState, useEffect, useCallback, useMemo } from "react";
import {
  Users,
  UserPlus,
  RefreshCw,
  KeyRound,
  Ban,
  CheckCircle2,
  Layers,
  Edit2,
  UserCheck,
  Shield,
  Trash2,
} from "lucide-react";
import { Button, Card, Badge, Input, ConfirmDialog } from "@inklore/ui";
import { safeErrorText } from "@inklore/transport";
import { useTranslation } from "@inklore/i18n";
import { useAdminSession } from "../context/AdminSessionContext";
import { AdminHeader } from "../components/AdminHeader";
import { AdminPagination } from "../components/AdminPagination";
import { AdminNotice } from "../components/AdminNotice";
import {
  PlatformMember,
  LlmBudget,
  LlmPricing,
  LlmModel,
  fetchPlatformMembers,
  updatePlatformMember,
  deletePlatformMember,
  revokeMemberSessions,
  fetchLlmBudget,
  fetchLlmPricings,
  fetchLlmModels,
} from "../api/adminApi";
import { CreateMemberModal, EditMemberModal, ResetPasswordModal } from "./UserManagementModals";
import { BillingRulesPanel } from "./BillingRulesPanel";

export function UserManagementRoute() {
  const { t } = useTranslation();
  const { isAuthenticated, snapshot } = useAdminSession();

  // 页面 Tab: users(租户成员) | billing(计费规则与配额)
  const [activeTab, setActiveTab] = useState<"users" | "billing">("users");

  // 数据加载状态
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<unknown>(null);
  const [actionError, setActionError] = useState<unknown>(null);
  const [hasLoaded, setHasLoaded] = useState(false);
  const [members, setMembers] = useState<PlatformMember[]>([]);
  const [budget, setBudget] = useState<LlmBudget | null>(null);
  const [pricings, setPricings] = useState<LlmPricing[]>([]);
  const [models, setModels] = useState<LlmModel[]>([]);

  // 搜索与过滤
  const [searchQuery, setSearchQuery] = useState("");
  const [roleFilter, setRoleFilter] = useState("all");
  const [statusFilter, setStatusFilter] = useState("all");

  // 分页状态
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);

  // 弹窗状态
  const [createModalOpen, setCreateModalOpen] = useState(false);
  const [editModalOpen, setEditModalOpen] = useState(false);
  const [editingMember, setEditingMember] = useState<PlatformMember | null>(null);
  const [resetPwdModalOpen, setResetPwdModalOpen] = useState(false);
  const [resetPwdMember, setResetPwdMember] = useState<PlatformMember | null>(null);

  // 确认对话框
  const [revokeConfirmOpen, setRevokeConfirmOpen] = useState(false);
  const [revokeMemberTarget, setRevokeMemberTarget] = useState<PlatformMember | null>(null);
  const [revoking, setRevoking] = useState(false);

  const [disableConfirmOpen, setDisableConfirmOpen] = useState(false);
  const [disableMemberTarget, setDisableMemberTarget] = useState<PlatformMember | null>(null);
  const [disabling, setDisabling] = useState(false);

  // 确认对话框：彻底删除
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [deleteMemberTarget, setDeleteMemberTarget] = useState<PlatformMember | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [deleteMemberError, setDeleteMemberError] = useState<string | null>(null);

  // 成功提示
  const [actionSuccess, setActionSuccess] = useState<string | null>(null);

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      const [membersPage, b, pList, mList] = await Promise.all([
        fetchPlatformMembers(100),
        fetchLlmBudget(),
        fetchLlmPricings(),
        fetchLlmModels(),
      ]);
      setMembers(membersPage.items || []);
      setBudget(b);
      setPricings(pList || []);
      setModels(mList || []);
      setHasLoaded(true);
      setLoadError(null);
    } catch (err) {
      setLoadError(err);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (isAuthenticated) {
      loadData();
    }
  }, [loadData, isAuthenticated]);

  const dismissNotice = useCallback(() => {
    setLoadError(null);
    setActionError(null);
    setActionSuccess(null);
  }, []);

  // 统计数据
  const totalCount = members.length;
  const activeCount = members.filter((m) => m.status === "active").length;
  const creatorCount = members.filter((m) => m.roles.includes("creator")).length;
  const operatorCount = members.filter((m) => m.roles.includes("operator")).length;

  // 活跃系统管理员与总系统管理员列表（用于防御性编程：必须保留至少一个系统管理员）
  const activeOperators = useMemo(() => {
    return members.filter((m) => m.status === "active" && m.roles.includes("operator"));
  }, [members]);
  const totalOperators = useMemo(() => {
    return members.filter((m) => m.roles.includes("operator"));
  }, [members]);

  const isLastOperator = useCallback(
    (m: PlatformMember) => {
      return (
        m.roles.includes("operator") &&
        (activeOperators.length <= 1 || totalOperators.length <= 1)
      );
    },
    [activeOperators.length, totalOperators.length]
  );

  // 过滤成员列表
  const filteredMembers = useMemo(() => {
    return members.filter((m) => {
      const q = searchQuery.toLowerCase().trim();
      const matchQuery =
        !q ||
        m.display_name.toLowerCase().includes(q) ||
        m.email.toLowerCase().includes(q) ||
        m.id.toLowerCase().includes(q);
      const matchRole = roleFilter === "all" || m.roles.includes(roleFilter);
      const matchStatus = statusFilter === "all" || m.status === statusFilter;
      return matchQuery && matchRole && matchStatus;
    });
  }, [members, searchQuery, roleFilter, statusFilter]);

  // 分页列表切片
  const paginatedMembers = useMemo(() => {
    const start = (page - 1) * pageSize;
    return filteredMembers.slice(start, start + pageSize);
  }, [filteredMembers, page, pageSize]);

  // 吊销会话处理
  const handleRevokeSessions = async () => {
    if (!revokeMemberTarget) return;
    setActionError(null);
    setRevoking(true);
    try {
      await revokeMemberSessions(revokeMemberTarget.id, {
        expected_revision: revokeMemberTarget.revision,
      });
      setRevokeConfirmOpen(false);
      setRevokeMemberTarget(null);
      setActionSuccess(t("admin.users.success_sessions_revoked"));
      await loadData();
    } catch (err: unknown) {
      setActionError(err);
    } finally {
      setRevoking(false);
    }
  };

  // 停用/启用状态处理（带全方位防御性校验）
  const handleToggleStatus = (m: PlatformMember) => {
    if (m.status === "active") {
      if (snapshot.principal?.user_id === m.user_id) {
        setActionError(new Error(t("admin.users.cannot_disable_self")));
        return;
      }
      if (isLastOperator(m)) {
        setActionError(new Error(t("admin.users.last_operator_disable_forbidden")));
        return;
      }
      setDisableMemberTarget(m);
      setDisableConfirmOpen(true);
    } else {
      void executeStatusChange(m, "active");
    }
  };

  const executeStatusChange = async (m: PlatformMember, targetStatus: "active" | "disabled") => {
    if (targetStatus === "disabled") {
      if (snapshot.principal?.user_id === m.user_id) {
        setActionError(new Error(t("admin.users.cannot_disable_self")));
        return;
      }
      if (isLastOperator(m)) {
        setActionError(new Error(t("admin.users.last_operator_disable_forbidden")));
        return;
      }
    }
    setActionError(null);
    setDisabling(true);
    try {
      await updatePlatformMember(m.id, {
        expected_revision: m.revision,
        roles: m.roles,
        status: targetStatus,
      });
      setDisableConfirmOpen(false);
      setDisableMemberTarget(null);
      setActionSuccess(t("admin.users.success_updated"));
      await loadData();
    } catch (err: unknown) {
      setActionError(err);
    } finally {
      setDisabling(false);
    }
  };

  // 彻底删除用户处理（带全方位防御性校验）
  const handleDeleteMember = async () => {
    if (!deleteMemberTarget) return;
    if (snapshot.principal?.user_id === deleteMemberTarget.user_id) {
      setActionError(new Error(t("admin.users.cannot_delete_self")));
      setDeleteConfirmOpen(false);
      return;
    }
    if (isLastOperator(deleteMemberTarget)) {
      setActionError(new Error(t("admin.users.last_operator_delete_forbidden")));
      setDeleteConfirmOpen(false);
      return;
    }
    setActionError(null);
    setDeleteMemberError(null);
    setDeleting(true);
    try {
      await deletePlatformMember(deleteMemberTarget.id);
      setDeleteConfirmOpen(false);
      setDeleteMemberTarget(null);
      setDeleteMemberError(null);
      setActionSuccess(t("admin.users.success_deleted"));
      await loadData();
    } catch (err: unknown) {
      setDeleteMemberError(safeErrorText(err, t));
      setActionError(err);
    } finally {
      setDeleting(false);
    }
  };

  // 多角色展示：一个用户可同时展示多个角色徽章
  const roleBadges = (roles: string[]) => {
    return (
      <div className="flex items-center gap-1.5 flex-wrap">
        {roles.includes("operator") && (
          <Badge variant="success" size="sm" className="font-mono flex items-center gap-1">
            <Shield className="w-3 h-3" />
            <span>{t("admin.users.role_operator")}</span>
          </Badge>
        )}
        {roles.includes("creator") && (
          <Badge variant="info" size="sm" className="font-mono flex items-center gap-1">
            <Layers className="w-3 h-3" />
            <span>{t("admin.users.role_creator")}</span>
          </Badge>
        )}
        {roles.includes("auditor") && (
          <Badge variant="secondary" size="sm" className="font-mono">
            <span>{t("admin.users.role_auditor")}</span>
          </Badge>
        )}
      </div>
    );
  };

  const formatDate = (isoStr: string) => {
    if (!isoStr) return "-";
    try {
      const d = new Date(isoStr);
      return d.toLocaleDateString() + " " + d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
    } catch {
      return isoStr;
    }
  };

  return (
    <div className="flex-1 flex flex-col h-full min-h-0 w-full overflow-hidden bg-[var(--bg-surface)] text-[var(--text-primary)]">
      <AdminNotice
        error={actionError ?? loadError}
        success={actionSuccess}
        onRetry={actionError == null && loadError != null ? () => void loadData() : undefined}
        onDismiss={dismissNotice}
      />

      {/* 顶部 Header 与功能选项卡 */}
      <AdminHeader
        title={t("admin.users.title")}
        actions={
          <div className="flex items-center gap-3 flex-wrap">
            <div className="inline-flex p-1 bg-[var(--bg-subtle)] border border-[var(--border-decorative)] rounded-lg text-xs font-medium shrink-0">
              <button
                type="button"
                onClick={() => setActiveTab("users")}
                className={`px-3 py-1.5 rounded-md transition-colors whitespace-nowrap shrink-0 ${
                  activeTab === "users"
                    ? "bg-[var(--bg-surface)] text-[var(--text-primary)] shadow-xs font-semibold"
                    : "text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
                }`}
              >
                {t("admin.users.tab_users")}
              </button>
              <button
                type="button"
                onClick={() => setActiveTab("billing")}
                className={`px-3 py-1.5 rounded-md transition-colors whitespace-nowrap shrink-0 ${
                  activeTab === "billing"
                    ? "bg-[var(--bg-surface)] text-[var(--text-primary)] shadow-xs font-semibold"
                    : "text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
                }`}
              >
                {t("admin.users.tab_billing")}
              </button>
            </div>

            <Button
              size="sm"
              variant="outline"
              icon={<RefreshCw className={`w-3.5 h-3.5 ${loading ? "animate-spin" : ""}`} />}
              onClick={loadData}
              title={t("common.refresh")}
              aria-label={t("common.refresh")}
            >
              {t("common.refresh")}
            </Button>

            {activeTab === "users" && (
              <Button
                size="sm"
                variant="primary"
                icon={<UserPlus className="w-4 h-4" />}
                onClick={() => setCreateModalOpen(true)}
              >
                {t("admin.users.btn_create_user")}
              </Button>
            )}
          </div>
        }
        className="px-6 py-3.5 bg-[var(--bg-surface)] border-b border-[var(--border-decorative)] shrink-0"
      />

      {/* Tab 1: 租户成员 CRUD 治理 */}
      {(loading || hasLoaded) && activeTab === "users" && (
        <div className="flex-1 min-h-0 flex flex-col">
          {/* 统计概览条 */}
          <div className="shrink-0 px-6 py-3 border-b border-[var(--border-decorative)] bg-[var(--bg-subtle)]/30 grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3.5">
            <Card className="p-3 bg-[var(--bg-surface)] border-[var(--border-decorative)] rounded-lg shadow-xs space-y-1">
              <div className="flex items-center justify-between">
                <span className="text-xs font-medium text-[var(--text-secondary)]">{t("admin.users.stat_total")}</span>
                <div className="w-6 h-6 rounded-md bg-[var(--action-primary)]/10 text-[var(--action-primary)] flex items-center justify-center">
                  <Users className="w-3.5 h-3.5" />
                </div>
              </div>
              <div className="text-lg font-bold font-mono text-[var(--text-primary)]">{totalCount}</div>
            </Card>

            <Card className="p-3 bg-[var(--bg-surface)] border-[var(--border-decorative)] rounded-lg shadow-xs space-y-1">
              <div className="flex items-center justify-between">
                <span className="text-xs font-medium text-[var(--text-secondary)]">{t("admin.users.stat_active")}</span>
                <div className="w-6 h-6 rounded-md bg-[var(--status-success)]/10 text-[var(--status-success)] flex items-center justify-center">
                  <UserCheck className="w-3.5 h-3.5" />
                </div>
              </div>
              <div className="text-lg font-bold font-mono text-[var(--text-primary)]">{activeCount}</div>
            </Card>

            <Card className="p-3 bg-[var(--bg-surface)] border-[var(--border-decorative)] rounded-lg shadow-xs space-y-1">
              <div className="flex items-center justify-between">
                <span className="text-xs font-medium text-[var(--text-secondary)]">{t("admin.users.stat_creators")}</span>
                <div className="w-6 h-6 rounded-md bg-indigo-500/10 text-indigo-500 flex items-center justify-center">
                  <Layers className="w-3.5 h-3.5" />
                </div>
              </div>
              <div className="text-lg font-bold font-mono text-[var(--text-primary)]">{creatorCount}</div>
            </Card>

            <Card className="p-3 bg-[var(--bg-surface)] border-[var(--border-decorative)] rounded-lg shadow-xs space-y-1">
              <div className="flex items-center justify-between">
                <span className="text-xs font-medium text-[var(--text-secondary)]">{t("admin.users.stat_operators")}</span>
                <div className="w-6 h-6 rounded-md bg-emerald-500/10 text-emerald-500 flex items-center justify-center">
                  <Shield className="w-3.5 h-3.5" />
                </div>
              </div>
              <div className="text-lg font-bold font-mono text-[var(--text-primary)]">{operatorCount}</div>
            </Card>
          </div>

          {/* 搜索与多维度筛选条 */}
          <div className="shrink-0 px-6 py-2.5 border-b border-[var(--border-decorative)] bg-[var(--bg-subtle)]/40 flex flex-wrap items-center justify-between gap-3">
            <div className="flex-1 min-w-[240px] max-w-md">
              <Input
                placeholder={t("admin.users.search_placeholder")}
                value={searchQuery}
                onChange={(e) => {
                  setSearchQuery(e.target.value);
                  setPage(1);
                }}
                className="w-full text-xs"
              />
            </div>
            <div className="flex items-center gap-3">
              <select
                value={roleFilter}
                onChange={(e) => {
                  setRoleFilter(e.target.value);
                  setPage(1);
                }}
                className="h-8 px-2.5 text-xs rounded-lg border border-[var(--border-decorative)] bg-[var(--bg-canvas)] text-[var(--text-primary)] cursor-pointer"
              >
                <option value="all">{t("admin.users.filter_role_all")}</option>
                <option value="creator">{t("admin.users.role_creator")}</option>
                <option value="operator">{t("admin.users.role_operator")}</option>
              </select>

              <select
                value={statusFilter}
                onChange={(e) => {
                  setStatusFilter(e.target.value);
                  setPage(1);
                }}
                className="h-8 px-2.5 text-xs rounded-lg border border-[var(--border-decorative)] bg-[var(--bg-canvas)] text-[var(--text-primary)] cursor-pointer"
              >
                <option value="all">{t("admin.users.filter_status_all")}</option>
                <option value="active">{t("admin.users.status_active")}</option>
                <option value="disabled">{t("admin.users.status_disabled")}</option>
              </select>
            </div>
          </div>

          {/* 成员数据列表表格 */}
          <div className="flex-1 min-h-0 flex flex-col bg-[var(--bg-surface)] overflow-hidden">
            <div className="flex-1 min-h-0 overflow-auto">
              <table className="w-full min-w-[960px] text-xs text-left">
                <thead className="bg-[var(--bg-subtle)] text-[var(--text-secondary)] border-b border-[var(--border-decorative)] font-mono uppercase tracking-wider sticky top-0 z-10">
                  <tr>
                    <th className="p-3.5 min-w-[200px]">{t("admin.users.col_user")}</th>
                    <th className="p-3.5 min-w-[180px]">{t("admin.users.col_email")}</th>
                    <th className="p-3.5 min-w-[180px]">{t("admin.users.col_roles")}</th>
                    <th className="p-3.5 min-w-[100px]">{t("admin.users.col_status")}</th>
                    <th className="p-3.5 min-w-[140px]">{t("admin.users.col_updated_at")}</th>
                    <th className="p-3.5 min-w-[260px] text-right">{t("admin.users.col_actions")}</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-[var(--border-decorative)] text-[var(--text-secondary)]">
                  {filteredMembers.length === 0 ? (
                    <tr>
                      <td colSpan={6} className="p-16 text-center text-[var(--text-muted)] font-sans">
                        {loading ? t("common.loading") : t("admin.users.empty")}
                      </td>
                    </tr>
                  ) : (
                    paginatedMembers.map((m) => {
                      const isActive = m.status === "active";
                      const isSelf = snapshot.principal?.user_id === m.user_id;
                      const isLastOp = isLastOperator(m);

                      // 防御性判断：不能删除当前登录用户，且必须保留至少一个系统管理员
                      const cannotDelete = isSelf || isLastOp;
                      const deleteDisabledReason = isSelf
                        ? t("admin.users.cannot_delete_self")
                        : isLastOp
                        ? t("admin.users.last_operator_delete_forbidden")
                        : undefined;

                      // 防御性判断：不能停用当前登录用户，且必须保留至少一个系统管理员
                      const cannotDisable = isActive && (isSelf || isLastOp);
                      const disableDisabledReason = isSelf
                        ? t("admin.users.cannot_disable_self")
                        : isLastOp
                        ? t("admin.users.last_operator_disable_forbidden")
                        : undefined;

                      return (
                        <tr key={m.id} className="hover:bg-[var(--bg-subtle)]/50 transition-colors">
                          <td className="p-3.5">
                            <div className="flex items-center gap-2.5">
                              <div className="w-7 h-7 rounded-full bg-[var(--bg-subtle)] border border-[var(--border-decorative)] text-[var(--text-primary)] font-bold text-xs flex items-center justify-center shrink-0">
                                {m.display_name.slice(0, 1).toUpperCase()}
                              </div>
                              <div className="min-w-0">
                                <div className="font-semibold text-[var(--text-primary)] flex items-center gap-1.5 whitespace-nowrap">
                                  <span>{m.display_name}</span>
                                  {isSelf && (
                                    <span className="text-[10px] px-1 py-0.2 rounded bg-amber-500/10 text-amber-600 dark:text-amber-400 font-mono">
                                      当前会话
                                    </span>
                                  )}
                                </div>
                                <div className="text-[10px] font-mono text-[var(--text-muted)] truncate max-w-xs">
                                  ID: {m.id}
                                </div>
                              </div>
                            </div>
                          </td>
                          <td className="p-3.5 font-mono text-[var(--text-primary)] font-medium whitespace-nowrap">
                            {m.email}
                          </td>
                          <td className="p-3.5 whitespace-nowrap">
                            {roleBadges(m.roles)}
                          </td>
                          <td className="p-3.5 whitespace-nowrap">
                            <Badge variant={isActive ? "success" : "secondary"} size="sm">
                              {isActive ? t("admin.users.status_active") : t("admin.users.status_disabled")}
                            </Badge>
                          </td>
                          <td className="p-3.5 font-mono text-[var(--text-muted)] whitespace-nowrap">
                            {formatDate(m.updated_at || m.created_at)}
                          </td>
                          <td className="p-3.5 text-right whitespace-nowrap">
                            <div className="flex items-center justify-end gap-1.5">
                              <Button
                                size="sm"
                                variant="outline"
                                icon={<Edit2 className="w-3.5 h-3.5" />}
                                onClick={() => {
                                  setEditingMember(m);
                                  setEditModalOpen(true);
                                }}
                                title={t("admin.users.action_edit")}
                              >
                                {t("admin.users.action_edit")}
                              </Button>

                              <Button
                                size="sm"
                                variant="ghost"
                                icon={<KeyRound className="w-3.5 h-3.5 text-amber-500" />}
                                onClick={() => {
                                  setResetPwdMember(m);
                                  setResetPwdModalOpen(true);
                                }}
                                title={t("admin.users.action_reset_pwd")}
                              >
                                {t("admin.users.action_reset_pwd")}
                              </Button>

                              <Button
                                size="sm"
                                variant="ghost"
                                icon={<RefreshCw className="w-3.5 h-3.5 text-[var(--action-primary)]" />}
                                onClick={() => {
                                  setRevokeMemberTarget(m);
                                  setRevokeConfirmOpen(true);
                                }}
                                title={t("admin.users.action_revoke_sessions")}
                              >
                                {t("admin.users.action_revoke_sessions")}
                              </Button>

                              <Button
                                size="sm"
                                variant="ghost"
                                icon={
                                  isActive ? (
                                    <Ban className={`w-3.5 h-3.5 ${cannotDisable ? "text-[var(--text-muted)] opacity-40" : "text-amber-500"}`} />
                                  ) : (
                                    <CheckCircle2 className="w-3.5 h-3.5 text-emerald-500" />
                                  )
                                }
                                onClick={() => !cannotDisable && handleToggleStatus(m)}
                                disabled={cannotDisable}
                                title={disableDisabledReason || (isActive ? t("admin.users.action_disable") : t("admin.users.action_enable"))}
                                className={
                                  cannotDisable
                                    ? "cursor-not-allowed opacity-40 hover:bg-transparent"
                                    : isActive
                                    ? "hover:text-amber-500 hover:bg-amber-500/10"
                                    : "hover:text-emerald-500 hover:bg-emerald-500/10"
                                }
                              >
                                {isActive ? t("admin.users.action_disable") : t("admin.users.action_enable")}
                              </Button>

                              <Button
                                size="sm"
                                variant="ghost"
                                icon={<Trash2 className={`w-3.5 h-3.5 ${cannotDelete ? "text-[var(--text-muted)] opacity-40" : "text-red-500"}`} />}
                                onClick={() => {
                                  if (cannotDelete) return;
                                  setDeleteMemberTarget(m);
                                  setDeleteConfirmOpen(true);
                                }}
                                disabled={cannotDelete}
                                title={deleteDisabledReason || t("admin.users.action_delete")}
                                className={
                                  cannotDelete
                                    ? "cursor-not-allowed opacity-40 hover:bg-transparent"
                                    : "hover:text-red-600 hover:bg-red-500/10 text-red-500"
                                }
                              >
                                {t("admin.users.action_delete")}
                              </Button>
                            </div>
                          </td>
                        </tr>
                      );
                    })
                  )}
                </tbody>
              </table>
            </div>

            {/* 统一底部分页组件 */}
            <AdminPagination
              total={filteredMembers.length}
              page={page}
              pageSize={pageSize}
              onPageChange={setPage}
              onPageSizeChange={(newSize) => {
                setPageSize(newSize);
                setPage(1);
              }}
              pageSizeOptions={[10, 20, 50, 100]}
            />
          </div>
        </div>
      )}

      {/* Tab 2: 计费规则与配额管理 */}
      {(loading || hasLoaded) && activeTab === "billing" && (
        <div className="flex-1 min-h-0 overflow-auto">
          <BillingRulesPanel
            budget={budget}
            pricings={pricings}
            models={models}
            onBudgetUpdated={(updated) => setBudget(updated)}
            onSuccess={(msg) => {
              setActionError(null);
              setActionSuccess(msg);
            }}
          />
        </div>
      )}

      {/* 弹窗组件挂载 */}
      <CreateMemberModal
        open={createModalOpen}
        onClose={() => setCreateModalOpen(false)}
        onSuccess={(msg) => {
          setActionError(null);
          setActionSuccess(msg);
          void loadData();
        }}
      />

      <EditMemberModal
        open={editModalOpen}
        member={editingMember}
        isSelf={editingMember ? snapshot.principal?.user_id === editingMember.user_id : false}
        isLastOperator={editingMember ? isLastOperator(editingMember) : false}
        onClose={() => {
          setEditModalOpen(false);
          setEditingMember(null);
        }}
        onSuccess={(msg) => {
          setActionError(null);
          setActionSuccess(msg);
          void loadData();
        }}
      />

      <ResetPasswordModal
        open={resetPwdModalOpen}
        member={resetPwdMember}
        onClose={() => {
          setResetPwdModalOpen(false);
          setResetPwdMember(null);
        }}
        onSuccess={(msg) => {
          setActionError(null);
          setActionSuccess(msg);
          void loadData();
        }}
      />

      {/* 吊销会话确认对话框 */}
      {revokeConfirmOpen && revokeMemberTarget && (
        <ConfirmDialog
          open={revokeConfirmOpen}
          title={t("admin.users.confirm_revoke_sessions_title")}
          description={t("admin.users.confirm_revoke_sessions_desc")}
          confirmText={t("common.confirm")}
          cancelText={t("common.cancel")}
          variant="danger"
          onConfirm={handleRevokeSessions}
          onClose={() => setRevokeConfirmOpen(false)}
          loading={revoking}
        />
      )}

      {/* 停用账号确认对话框 */}
      {disableConfirmOpen && disableMemberTarget && (
        <ConfirmDialog
          open={disableConfirmOpen}
          title={t("admin.users.confirm_disable_title")}
          description={
            disableMemberTarget && (snapshot.principal?.user_id === disableMemberTarget.user_id || isLastOperator(disableMemberTarget))
              ? snapshot.principal?.user_id === disableMemberTarget.user_id
                ? t("admin.users.cannot_disable_self")
                : t("admin.users.last_operator_disable_forbidden")
              : t("admin.users.confirm_disable_desc")
          }
          confirmText={t("admin.users.action_disable")}
          cancelText={t("common.cancel")}
          variant="danger"
          onConfirm={() => disableMemberTarget && executeStatusChange(disableMemberTarget, "disabled")}
          onClose={() => setDisableConfirmOpen(false)}
          loading={disabling}
          confirmDisabled={disableMemberTarget ? snapshot.principal?.user_id === disableMemberTarget.user_id || isLastOperator(disableMemberTarget) : false}
        />
      )}

      {/* 彻底删除账号确认对话框 */}
      {deleteConfirmOpen && deleteMemberTarget && (
        <ConfirmDialog
          open={deleteConfirmOpen}
          title={t("admin.users.confirm_delete_title")}
          description={
            <div className="space-y-2">
              <p>
                {deleteMemberTarget && (snapshot.principal?.user_id === deleteMemberTarget.user_id || isLastOperator(deleteMemberTarget))
                  ? snapshot.principal?.user_id === deleteMemberTarget.user_id
                    ? t("admin.users.cannot_delete_self")
                    : t("admin.users.last_operator_delete_forbidden")
                  : t("admin.users.confirm_delete_desc")}
              </p>
              {deleteMemberError && (
                <div role="alert" className="p-3 rounded-lg bg-[var(--status-danger)]/10 border border-[var(--status-danger)]/30 text-[var(--status-danger)] text-xs font-medium">
                  {deleteMemberError}
                </div>
              )}
            </div>
          }
          confirmText={t("admin.users.btn_confirm_delete")}
          cancelText={t("common.cancel")}
          variant="danger"
          onConfirm={handleDeleteMember}
          onClose={() => { setDeleteConfirmOpen(false); setDeleteMemberError(null); }}
          loading={deleting}
          confirmDisabled={deleteMemberTarget ? snapshot.principal?.user_id === deleteMemberTarget.user_id || isLastOperator(deleteMemberTarget) : false}
        />
      )}
    </div>
  );
}

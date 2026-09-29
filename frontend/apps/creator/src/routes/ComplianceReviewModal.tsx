import React, { useState, useEffect, useCallback } from "react";
import {
  ShieldAlert,
  ShieldCheck,
  AlertTriangle,
  RefreshCw,
  Check,
  EyeOff,
  Sparkles,
  Info,
} from "lucide-react";
import { Button, Badge, Modal, ErrorPresenter } from "@inklore/ui";
import type { ApiSDK } from "@inklore/transport";
import type { ComplianceReport, ComplianceIssue } from "@inklore/contracts";
import { useTranslation } from "@inklore/i18n";

export interface ComplianceReviewModalProps {
  open: boolean;
  onClose: () => void;
  api: ApiSDK;
  novelId: string;
  chapterId: string;
  chapterRevision: number;
  chapterTitle: string;
  bodyText: string;
  onAdoptSuggestion: (startOffset: number, endOffset: number, replacement: string) => void;
}

/** 异步计算 UTF-8 文本的 SHA-256 十六进制摘要 */
async function computeSha256(text: string): Promise<string> {
  const encoder = new TextEncoder();
  const data = encoder.encode(text);
  const hashBuffer = await crypto.subtle.digest("SHA-256", data);
  const hashArray = Array.from(new Uint8Array(hashBuffer));
  return hashArray.map((b) => b.toString(16).padStart(2, "0")).join("");
}

export function ComplianceReviewModal({
  open,
  onClose,
  api,
  novelId,
  chapterId,
  chapterRevision,
  chapterTitle,
  bodyText,
  onAdoptSuggestion,
}: ComplianceReviewModalProps) {
  const { t } = useTranslation();
  const [scanning, setScanning] = useState(false);
  const [report, setReport] = useState<ComplianceReport | null>(null);
  const [issues, setIssues] = useState<ComplianceIssue[]>([]);
  const [error, setError] = useState<unknown>(null);
  const [adoptedIssueIds, setAdoptedIssueIds] = useState<Set<string>>(new Set());

  const runScan = useCallback(async () => {
    if (!bodyText.trim()) {
      setError(new Error(t("compliance.err_empty_content")));
      return;
    }
    setScanning(true);
    setError(null);
    setAdoptedIssueIds(new Set());
    try {
      const contentHash = await computeSha256(bodyText);
      const rep = await api.compliance.scan({
        novel_id: novelId,
        chapter_id: chapterId,
        chapter_revision: chapterRevision,
        content_hash: contentHash,
        text: bodyText,
      });
      setReport(rep);
      const issueList = await api.compliance.listIssues(rep.id, { limit: 100 });
      setIssues(issueList);
    } catch (err: unknown) {
      setError(err);
    } finally {
      setScanning(false);
    }
  }, [api, bodyText, chapterId, chapterRevision, novelId, t]);

  useEffect(() => {
    if (open) {
      void runScan();
    } else {
      setReport(null);
      setIssues([]);
      setError(null);
    }
  }, [open, runScan]);

  const handleAdopt = (issue: ComplianceIssue) => {
    if (!issue.suggestion || !issue.suggestion.trim()) return;
    onAdoptSuggestion(issue.start_offset, issue.end_offset, issue.suggestion.trim());
    setAdoptedIssueIds((prev) => new Set(prev).add(issue.id));
  };

  const handleIgnore = async (issue: ComplianceIssue) => {
    if (!report) return;
    try {
      await api.compliance.ignoreIssue(report.id, issue.id);
      setIssues((prev) =>
        prev.map((item) => (item.id === issue.id ? { ...item, state: "ignored" } : item))
      );
    } catch (err) {
      setError(err);
    }
  };

  const aiScore = report?.ai_flavor_score ?? null;
  const scoreColor =
    aiScore === null
      ? "text-stone-500"
      : aiScore < 30
      ? "text-emerald-600 dark:text-emerald-400"
      : aiScore < 60
      ? "text-amber-600 dark:text-amber-400"
      : "text-rose-600 dark:text-rose-400";

  const scoreBadgeBg =
    aiScore === null
      ? "bg-stone-100 text-stone-600 dark:bg-stone-800 dark:text-stone-300"
      : aiScore < 30
      ? "bg-emerald-50 text-emerald-700 border-emerald-200 dark:bg-emerald-950/40 dark:text-emerald-300"
      : aiScore < 60
      ? "bg-amber-50 text-amber-700 border-amber-200 dark:bg-amber-950/40 dark:text-amber-300"
      : "bg-rose-50 text-rose-700 border-rose-200 dark:bg-rose-950/40 dark:text-rose-300";

  const activeIssues = issues.filter((i) => i.state !== "ignored");

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={t("compliance.modal_title")}
      className="max-w-3xl"
    >
      <div className="space-y-4">
        {/* 顶部概览指标栏 */}
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 p-3.5 rounded-lg bg-stone-50 dark:bg-stone-900 border border-stone-200/80 dark:border-stone-800 text-xs">
          <div>
            <span className="text-stone-500">{t("compliance.col_chapter")}:</span>
            <p className="font-medium text-stone-800 dark:text-stone-200 truncate mt-0.5">
              {chapterTitle || t("editor.untitled")}
            </p>
          </div>
          <div>
            <span className="text-stone-500">{t("compliance.col_ai_score")}:</span>
            <div className="flex items-center gap-1.5 mt-0.5">
              <span className={`text-base font-bold font-mono ${scoreColor}`}>
                {aiScore !== null ? aiScore : "—"}
              </span>
              <span className="text-stone-400">/ 100</span>
              <span className={`px-1.5 py-0.5 rounded text-[10px] font-semibold border ${scoreBadgeBg}`}>
                {aiScore === null
                  ? t("common.loading")
                  : aiScore < 30
                  ? t("compliance.score_level_low")
                  : aiScore < 60
                  ? t("compliance.score_level_mid")
                  : t("compliance.score_level_high")}
              </span>
            </div>
          </div>
          <div>
            <span className="text-stone-500">{t("compliance.col_risk_count")}:</span>
            <div className="flex items-center gap-2 mt-0.5 font-mono text-sm font-semibold">
              <span className="text-rose-600 dark:text-rose-400">
                {report ? report.block_count : 0} {t("compliance.risk_block")}
              </span>
              <span className="text-stone-300 dark:text-stone-700">|</span>
              <span className="text-amber-600 dark:text-amber-400">
                {activeIssues.length} {t("compliance.risk_pending")}
              </span>
            </div>
          </div>
        </div>

        {error != null && (
          <div className="p-3 rounded-md bg-rose-50 dark:bg-rose-950/40 border border-rose-200 dark:border-rose-900/60">
            <ErrorPresenter error={error} />
          </div>
        )}

        {/* 扫描等待状态 */}
        {scanning ? (
          <div className="py-12 flex flex-col items-center justify-center space-y-3 text-stone-500">
            <RefreshCw className="w-6 h-6 animate-spin text-amber-600" />
            <p className="text-xs font-medium">{t("compliance.scanning_hint")}</p>
          </div>
        ) : activeIssues.length === 0 ? (
          <div className="py-12 flex flex-col items-center justify-center space-y-2 text-stone-500">
            <ShieldCheck className="w-10 h-10 text-emerald-600" />
            <p className="text-sm font-semibold text-stone-700 dark:text-stone-300">
              {t("compliance.all_passed")}
            </p>
            <p className="text-xs text-stone-400 max-w-sm text-center">
              {t("compliance.all_passed_desc")}
            </p>
          </div>
        ) : (
          <div className="space-y-3 max-h-[380px] overflow-y-auto pr-1 text-xs">
            {activeIssues.map((issue) => {
              const isAdopted = adoptedIssueIds.has(issue.id);
              const isBlock = issue.severity === "block";
              return (
                <div
                  key={issue.id}
                  className={`p-3 rounded-lg border transition-all ${
                    isAdopted
                      ? "bg-emerald-50/40 border-emerald-300/60 dark:bg-emerald-950/20 dark:border-emerald-800/40 opacity-75"
                      : isBlock
                      ? "bg-rose-50/50 border-rose-200 dark:bg-rose-950/20 dark:border-rose-900/40"
                      : "bg-white dark:bg-stone-900 border-stone-200 dark:border-stone-800 shadow-2xs"
                  }`}
                >
                  <div className="flex items-center justify-between gap-2 mb-1.5">
                    <div className="flex items-center gap-1.5">
                      {isBlock ? (
                        <ShieldAlert className="w-3.5 h-3.5 text-rose-600 shrink-0" />
                      ) : (
                        <AlertTriangle className="w-3.5 h-3.5 text-amber-600 shrink-0" />
                      )}
                      <span className="font-semibold text-stone-800 dark:text-stone-200">
                        {issue.kind === "sensitive"
                          ? t("compliance.kind_sensitive")
                          : t("compliance.kind_ai_flavor")}
                      </span>
                      <Badge variant={isBlock ? "danger" : "warning"}>
                        {isBlock ? t("compliance.severity_block") : t("compliance.severity_warning")}
                      </Badge>
                      <span className="text-[11px] font-mono text-stone-400">
                        [{issue.start_offset}, {issue.end_offset}]
                      </span>
                    </div>

                    <div className="flex items-center gap-1">
                      <Button
                        size="sm"
                        variant="ghost"
                        onClick={() => void handleIgnore(issue)}
                        icon={<EyeOff className="w-3 h-3" />}
                        title={t("compliance.btn_ignore")}
                      >
                        {t("compliance.btn_ignore")}
                      </Button>
                      {issue.suggestion && (
                        <Button
                          size="sm"
                          variant={isAdopted ? "outline" : "primary"}
                          disabled={isAdopted}
                          onClick={() => handleAdopt(issue)}
                          icon={isAdopted ? <Check className="w-3 h-3" /> : <Sparkles className="w-3 h-3" />}
                        >
                          {isAdopted ? t("compliance.btn_adopted") : t("compliance.btn_adopt")}
                        </Button>
                      )}
                    </div>
                  </div>

                  {/* 原文摘录与建议修改 */}
                  <div className="space-y-1.5 bg-stone-50 dark:bg-stone-950/50 p-2.5 rounded border border-stone-200/60 dark:border-stone-800/60 text-xs font-mono">
                    <div className="flex items-start gap-1.5">
                      <span className="text-rose-600 dark:text-rose-400 shrink-0 font-bold font-sans">-</span>
                      <span className="text-stone-700 dark:text-stone-300 break-all select-all">
                        {issue.excerpt}
                      </span>
                    </div>
                    {issue.suggestion && (
                      <div className="flex items-start gap-1.5 text-emerald-700 dark:text-emerald-400">
                        <span className="shrink-0 font-bold font-sans">+</span>
                        <span className="break-all select-all font-semibold">
                          {issue.suggestion}
                        </span>
                      </div>
                    )}
                  </div>

                  {issue.message && (
                    <div className="flex items-center gap-1 text-[11px] text-stone-500 mt-1.5">
                      <Info className="w-3 h-3 shrink-0 text-stone-400" />
                      <span>{issue.message}</span>
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        )}

        {/* 底部操作区 */}
        <div className="flex items-center justify-between pt-3 border-t border-stone-200/80 dark:border-stone-800 text-xs">
          <Button
            size="sm"
            variant="outline"
            onClick={() => void runScan()}
            disabled={scanning}
            icon={<RefreshCw className={`w-3.5 h-3.5 ${scanning ? "animate-spin" : ""}`} />}
          >
            {t("compliance.btn_rescan")}
          </Button>

          <Button size="sm" variant="primary" onClick={onClose}>
            {t("common.close")}
          </Button>
        </div>
      </div>
    </Modal>
  );
}

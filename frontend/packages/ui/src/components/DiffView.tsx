/**
 * @inklore/ui - 文本差异比对组件 (DiffView)
 * 严格遵循 20-FRONTEND-REUSE-AND-STATE-CONTRACTS.md §3.7
 * 
 * 核心契约与红线：
 * 1. 差异计算纯函数，行级与块级高亮。
 * 2. 新增除底色外必须显式标注「+ 新增」，删除除底色外必须显式标注「- 删除」，符合 WCAG 无障碍规范。
 * 3. 严格禁止出现任何 Unicode Emoji。
 * 4. 任何一侧未 loaded 禁止当做空字符串计算。
 */

import React, { useMemo } from "react";
import { useTranslation } from "@inklore/i18n";

export interface TextSnapshot {
  resourceId: string;
  revision: number;
  contentHash: string;
  text: string;
  status: "loaded" | "error" | "loading";
}

export interface DiffHunk {
  type: "added" | "removed" | "unchanged";
  line: string;
  leftLineNumber?: number;
  rightLineNumber?: number;
}

export interface DiffViewProps {
  left: TextSnapshot;
  right: TextSnapshot;
  leftLabel?: string;
  rightLabel?: string;
  onSelectResolution?: (resolvedText: string) => void;
  readOnly?: boolean;
}

/** 纯函数：逐行简单 LCS 差异比较算法 */
export function computeLineDiff(original: string, modified: string): DiffHunk[] {
  const origLines = original.split(/\r?\n/);
  const modLines = modified.split(/\r?\n/);
  const hunks: DiffHunk[] = [];

  let i = 0;
  let j = 0;
  let origLineNum = 1;
  let modLineNum = 1;

  while (i < origLines.length || j < modLines.length) {
    if (i < origLines.length && j < modLines.length && origLines[i] === modLines[j]) {
      hunks.push({
        type: "unchanged",
        line: origLines[i],
        leftLineNumber: origLineNum++,
        rightLineNumber: modLineNum++,
      });
      i++;
      j++;
    } else if (j < modLines.length && (i >= origLines.length || !origLines.slice(i).includes(modLines[j]))) {
      hunks.push({
        type: "added",
        line: modLines[j],
        rightLineNumber: modLineNum++,
      });
      j++;
    } else {
      hunks.push({
        type: "removed",
        line: origLines[i],
        leftLineNumber: origLineNum++,
      });
      i++;
    }
  }

  return hunks;
}

export function DiffView({
  left,
  right,
  leftLabel,
  rightLabel,
  onSelectResolution,
  readOnly = true,
}: DiffViewProps) {
  const { t } = useTranslation();

  const isReady = left.status === "loaded" && right.status === "loaded";

  const diffHunks = useMemo(() => {
    if (!isReady) return [];
    return computeLineDiff(left.text, right.text);
  }, [left.text, right.text, isReady]);

  if (!isReady) {
    return (
      <div style={{ padding: "32px", textAlign: "center", color: "var(--text-secondary)" }}>
        {t("common.loading")}
      </div>
    );
  }

  return (
    <div
      className="diff-view-container"
      style={{
        border: "1px solid var(--border-decorative)",
        borderRadius: "8px",
        backgroundColor: "var(--bg-surface)",
        overflow: "hidden",
      }}
    >
      <div
        style={{
          display: "flex",
          justifyContent: "space-between",
          padding: "10px 16px",
          backgroundColor: "var(--bg-subtle)",
          borderBottom: "1px solid var(--border-decorative)",
          fontSize: "13px",
          fontWeight: 600,
        }}
      >
        <span>
          {leftLabel || t("diff.base")} (rev: {left.revision})
        </span>
        <span>
          {rightLabel || t("diff.proposed")} (rev: {right.revision})
        </span>
      </div>

      <div style={{ maxHeight: "600px", overflowY: "auto", fontFamily: "monospace", fontSize: "13px" }}>
        {diffHunks.map((hunk, idx) => {
          let bg = "transparent";
          let badge = "";
          let badgeColor = "transparent";

          if (hunk.type === "added") {
            bg = "var(--diff-add-bg)";
            badge = "+ [新增]";
            badgeColor = "var(--status-success)";
          } else if (hunk.type === "removed") {
            bg = "var(--diff-remove-bg)";
            badge = "- [删除]";
            badgeColor = "var(--status-danger)";
          }

          return (
            <div
              key={idx}
              style={{
                display: "flex",
                backgroundColor: bg,
                padding: "2px 8px",
                lineHeight: "22px",
                borderBottom: "1px solid rgba(0,0,0,0.03)",
              }}
            >
              <div
                style={{
                  width: "70px",
                  color: "var(--text-muted)",
                  userSelect: "none",
                  display: "flex",
                  justifyContent: "space-between",
                  paddingRight: "8px",
                }}
              >
                <span>{hunk.leftLineNumber || ""}</span>
                <span>{hunk.rightLineNumber || ""}</span>
              </div>

              {badge && (
                <span
                  style={{
                    color: badgeColor,
                    fontWeight: "bold",
                    marginRight: "8px",
                    userSelect: "none",
                  }}
                >
                  {badge}
                </span>
              )}

              <div style={{ flex: 1, whiteSpace: "pre-wrap", wordBreak: "break-all" }}>
                {hunk.line || " "}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

import React from "react";
import { ChevronLeft, ChevronRight, BookOpen, Feather, Maximize2, Sparkles, CheckCircle2, CloudUpload, User } from "lucide-react";
import { Button } from "@inklore/ui";
import { useCreatorSession } from "../context/CreatorSessionContext";

export interface TopNavbarProps {
  novelTitle?: string;
  chapterTitle?: string;
  wordCount?: number;
  isSaved?: boolean;
  activeMode: "outline" | "writing" | "zen";
  onChangeMode: (mode: "outline" | "writing" | "zen") => void;
  onPrevChapter?: () => void;
  onNextChapter?: () => void;
  onToggleAiDrawer?: () => void;
}

export function TopNavbar({
  novelTitle = "未命名作品",
  chapterTitle = "第1章 · 归途初显",
  wordCount = 0,
  isSaved = true,
  activeMode,
  onChangeMode,
  onPrevChapter,
  onNextChapter,
  onToggleAiDrawer,
}: TopNavbarProps) {
  const { isAuthenticated, principal, openAuthPage } = useCreatorSession();

  return (
    <header className="h-14 border-b border-neutral-200 dark:border-neutral-800 bg-white dark:bg-neutral-900 px-4 flex items-center justify-between z-20 shrink-0 select-none">
      {/* Left: Novel Title & Back */}
      <div className="flex items-center gap-3">
        <div className="flex items-center gap-1 text-sm font-semibold text-neutral-900 dark:text-neutral-100">
          <span className="truncate max-w-[160px]">{novelTitle}</span>
        </div>
      </div>

      {/* Middle: Mode Switchers & Chapter Stepper */}
      <div className="flex items-center gap-3">
        {/* Mode Segments */}
        <div className="bg-neutral-100 dark:bg-neutral-800 p-0.5 rounded-lg flex items-center text-xs font-medium">
          <button
            onClick={() => onChangeMode("outline")}
            className={`flex items-center gap-1.5 px-3 py-1.5 rounded-md transition-all ${
              activeMode === "outline"
                ? "bg-white dark:bg-neutral-700 text-neutral-900 dark:text-neutral-100 shadow-xs"
                : "text-neutral-500 hover:text-neutral-800 dark:hover:text-neutral-200"
            }`}
          >
            <BookOpen className="w-3.5 h-3.5" />
            <span>大纲</span>
          </button>
          <button
            onClick={() => onChangeMode("writing")}
            className={`flex items-center gap-1.5 px-3 py-1.5 rounded-md transition-all ${
              activeMode === "writing"
                ? "bg-white dark:bg-neutral-700 text-neutral-900 dark:text-neutral-100 shadow-xs"
                : "text-neutral-500 hover:text-neutral-800 dark:hover:text-neutral-200"
            }`}
          >
            <Feather className="w-3.5 h-3.5" />
            <span>写作</span>
          </button>
          <button
            onClick={() => onChangeMode("zen")}
            className={`flex items-center gap-1.5 px-3 py-1.5 rounded-md transition-all ${
              activeMode === "zen"
                ? "bg-white dark:bg-neutral-700 text-neutral-900 dark:text-neutral-100 shadow-xs"
                : "text-neutral-500 hover:text-neutral-800 dark:hover:text-neutral-200"
            }`}
          >
            <Maximize2 className="w-3.5 h-3.5" />
            <span>沉浸</span>
          </button>
        </div>

        {/* Chapter Stepper */}
        {activeMode === "writing" && (
          <div className="flex items-center border border-neutral-200 dark:border-neutral-700 rounded-lg px-2 py-1 text-xs text-neutral-700 dark:text-neutral-300 bg-neutral-50 dark:bg-neutral-800/60">
            <button
              onClick={onPrevChapter}
              className="p-0.5 hover:text-amber-600 transition-colors"
              title="上一章"
            >
              <ChevronLeft className="w-4 h-4" />
            </button>
            <span className="px-2 font-medium truncate max-w-[140px]">{chapterTitle}</span>
            <button
              onClick={onNextChapter}
              className="p-0.5 hover:text-amber-600 transition-colors"
              title="下一章"
            >
              <ChevronRight className="w-4 h-4" />
            </button>
          </div>
        )}
      </div>

      {/* Right: AI Copilot Button, Words counter, Save Status */}
      <div className="flex items-center gap-3">
        {onToggleAiDrawer && (
          <Button
            size="sm"
            variant="outline"
            onClick={onToggleAiDrawer}
            icon={<Sparkles className="w-3.5 h-3.5 text-amber-500" />}
            className="text-xs"
          >
            AI 创作副驾
          </Button>
        )}

        <div className="h-4 w-px bg-neutral-200 dark:bg-neutral-700" />

        {/* Word count badge */}
        <span className="text-xs text-neutral-500 font-mono">
          {wordCount.toLocaleString()} 字
        </span>

        {/* Save status */}
        <div className="flex items-center gap-1 text-xs text-neutral-400" title={isSaved ? "已实时保存到本地与云端" : "保存中..."}>
          {isSaved ? (
            <CheckCircle2 className="w-4 h-4 text-emerald-500" />
          ) : (
            <CloudUpload className="w-4 h-4 text-amber-500 animate-pulse" />
          )}
          <span className="hidden sm:inline">{isSaved ? "已保存" : "暂存中"}</span>
        </div>

        <div className="h-4 w-px bg-neutral-200 dark:bg-neutral-700" />

        {/* User Account / Login Button */}
        {isAuthenticated ? (
          <div className="flex items-center gap-1.5 text-xs text-neutral-700 dark:text-neutral-300 font-medium">
            <div className="w-6 h-6 rounded-full bg-amber-500/20 text-amber-600 dark:text-amber-400 flex items-center justify-center">
              <User className="w-3.5 h-3.5" />
            </div>
            <span className="hidden sm:inline truncate max-w-[100px]">{principal?.display_name || "墨境作者"}</span>
          </div>
        ) : (
          <Button
            size="sm"
            variant="ghost"
            onClick={() => openAuthPage("login")}
            className="text-xs text-amber-600 dark:text-amber-400 hover:text-amber-700 h-8 px-2.5"
            icon={<User className="w-3.5 h-3.5" />}
          >
            登录
          </Button>
        )}
      </div>
    </header>
  );
}

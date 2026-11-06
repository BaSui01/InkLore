import React, { useState, useEffect, useRef } from "react";
import {
  BookOpen,
  Settings,
  PanelLeft,
  ArrowLeft,
  Edit3,
  Layers,
  SlidersHorizontal,
  Search,
  Monitor,
  Bot,
  Save,
  ChevronDown,
  Trash2,
  HelpCircle,
  ShieldCheck,
  Sparkles,
  RefreshCw,
  MoreVertical,
  AlertTriangle,
  FileText,
  CheckCircle2,
  CloudUpload,
  Plus,
  Download,
} from "lucide-react";
import { Button, Badge, Modal, Input, ErrorPresenter } from "@inklore/ui";
import { getApiSDK, reportSafeDiagnostic, safeErrorText } from "@inklore/transport";
import { useTranslation } from "@inklore/i18n";
import {
  ArtifactView,
  Volume,
  Chapter,
  Character,
  LoreEntry,
  Novel,
  LlmModel,
  isApiError,
  isValidRevision,
} from "@inklore/contracts";
import { countCodePoints, normalizeLf, downloadEditorDraft } from "./editorText";
import { AgentWorkbench } from "./AgentWorkbench";
import { PlainTextEditor } from "./PlainTextEditor";
import { ConflictResolutionModal } from "./ConflictResolutionModal";
import { ConfirmChapterModal } from "./ConfirmChapterModal";
import { ComplianceReviewModal } from "./ComplianceReviewModal";
import { ExportNovelDialog } from "./ExportNovelDialog";

export interface EditorRouteProps {
  novelId: string;
  onBack: () => void;
}

export function EditorRoute({ novelId, onBack }: EditorRouteProps) {
  const { t } = useTranslation();
  const [sidebarOpen, setSidebarOpen] = useState(true);
  const [activeTab, setActiveTab] = useState<"settings" | "outline" | "catalog" | "docs">("catalog");
  const [fontSize, setFontSize] = useState(18);
  const [workbenchOpen, setWorkbenchOpen] = useState(false);
  const [workbenchSelection, setWorkbenchSelection] = useState<{ text: string; startOffset: number; endOffset: number } | null>(null);
  const [availableModels, setAvailableModels] = useState<LlmModel[]>([]);
  const [selectedModelId, setSelectedModelId] = useState<string>("");

  // 纯文本正文内容（遵循 00基线§2 与 03分册§2）
  const [bodyText, setBodyText] = useState("");

  // 真实后端实体状态
  const [novel, setNovel] = useState<Novel | null>(null);
  const [volumes, setVolumes] = useState<Volume[]>([]);
  const [chapters, setChapters] = useState<Chapter[]>([]);
  const [activeChapter, setActiveChapter] = useState<Chapter | null>(null);
  const [characters, setCharacters] = useState<Character[]>([]);
  const [lore, setLore] = useState<LoreEntry[]>([]);
  const [currentRevision, setCurrentRevision] = useState<number | null>(null);
  const [saveStatus, setSaveStatus] = useState<"saved" | "saving" | "unsaved">("unsaved");
  const [lastSaveTime, setLastSaveTime] = useState("");
  const [loadError, setLoadError] = useState<unknown>(null);
  const [bodyLoadError, setBodyLoadError] = useState<unknown>(null);
  const [saveError, setSaveError] = useState<unknown>(null);
  const [accessDenied, setAccessDenied] = useState(false);
  const accessDeniedRef = useRef(false);
  const [chaptersLoaded, setChaptersLoaded] = useState(false);
  const [charactersLoaded, setCharactersLoaded] = useState(false);
  const [bodyLoading, setBodyLoading] = useState(false);
  const [loadedChapterId, setLoadedChapterId] = useState<string | null>(null);
  const [switchBlocked, setSwitchBlocked] = useState(false);

  // 409 版本冲突处理状态
  const [conflictModalOpen, setConflictModalOpen] = useState(false);
  // 章节确认入账弹窗状态
  const [confirmModalOpen, setConfirmModalOpen] = useState(false);
  const [complianceModalOpen, setComplianceModalOpen] = useState(false);
  const [exportOpen, setExportOpen] = useState(false);

  const [createChapterModalOpen, setCreateChapterModalOpen] = useState(false);
  const [newChapterTitle, setNewChapterTitle] = useState("");
  const [creatingChapter, setCreatingChapter] = useState(false);

  const api = getApiSDK();
  const saveTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const isComposingRef = useRef(false);
  const applyingServerBodyRef = useRef(false);
  const activeChapterRef = useRef<Chapter | null>(null);
  const loadedChapterIdRef = useRef<string | null>(null);
  const revisionRef = useRef<number | null>(null);
  const dirtyRef = useRef(false);
  const editGenerationRef = useRef(0);
  const saveInFlightRef = useRef(false);
  const saveErrorRef = useRef<unknown>(null);
  const listLoadTokenRef = useRef(0);
  const bodyLoadTokenRef = useRef(0);
  const bodyTextRef = useRef(bodyText);
  bodyTextRef.current = bodyText;

  const saveIntentRef = useRef<{
    chapterId: string;
    body: string;
    expectedRevision: number;
    idempotencyKey: string;
    generation: number;
  } | null>(null);

  // Unicode 码点字数统计（不含空白字符）
  const wordCount = countCodePoints(bodyText) || (activeChapter?.word_count || 0);

  // 正文变动处理
  const handleBodyChange = (newText: string) => {
    setBodyText(newText);
    if (
      !loadedChapterIdRef.current ||
      applyingServerBodyRef.current ||
      loadedChapterIdRef.current !== activeChapterRef.current?.id
    ) {
      return;
    }
    dirtyRef.current = true;
    editGenerationRef.current += 1;
    setSaveStatus("unsaved");

    if (isApiError(saveErrorRef.current) && [400, 422].includes(saveErrorRef.current.httpStatus)) {
      saveIntentRef.current = null;
      saveErrorRef.current = null;
      setSaveError(null);
    }

    if (isComposingRef.current || saveErrorRef.current) return;
    if (saveTimerRef.current) clearTimeout(saveTimerRef.current);
    saveTimerRef.current = setTimeout(() => {
      if (!isComposingRef.current) void handleAutoSave(newText);
    }, 1200);
  };

  // 只读资源独立落位；失败只保留旧快照，绝不将错误转换成业务空列表。
  const loadNovelData = async () => {
    if (!novelId) return;
    const token = ++listLoadTokenRef.current;
    const [novelResult, volumesResult, chaptersResult, charactersResult, loreResult] =
      await Promise.allSettled([
        api.content.getNovel(novelId),
        api.content.listVolumes(novelId),
        api.content.listChapters(novelId),
        api.worldview.listCharacters(novelId),
        api.worldview.listLore(novelId),
      ]);
    if (token !== listLoadTokenRef.current) return;

    const failures = [novelResult, volumesResult, chaptersResult, charactersResult, loreResult].filter(
      (result): result is PromiseRejectedResult => result.status === "rejected"
    );
    const denied = failures.find(
      ({ reason }) => isApiError(reason) && [403, 404].includes(reason.httpStatus)
    );
    if (denied) {
      accessDeniedRef.current = true;
      setAccessDenied(true);
    }
    const invalidNovel = novelResult.status === "fulfilled" && novelResult.value?.id !== novelId;
    if (novelResult.status === "fulfilled" && !denied && !invalidNovel) {
      setNovel(novelResult.value);
      if (volumesResult.status === "fulfilled") setVolumes(volumesResult.value);
      if (charactersResult.status === "fulfilled") {
        setCharacters(charactersResult.value);
        setCharactersLoaded(true);
      }
      if (loreResult.status === "fulfilled") setLore(loreResult.value);
      if (chaptersResult.status === "fulfilled") {
        const items = chaptersResult.value;
        const current = activeChapterRef.current;
        const stillPresent = items.find((chapter) => chapter.id === current?.id);
        if (current && !stillPresent && (dirtyRef.current || saveInFlightRef.current || saveIntentRef.current)) {
          setSwitchBlocked(true);
        } else {
          setChapters(items);
          setChaptersLoaded(true);
          if (stillPresent) {
            activeChapterRef.current = stillPresent;
            setActiveChapter(stillPresent);
          } else if (items.length > 0) {
            selectChapter(items[0]);
          } else {
            bodyLoadTokenRef.current += 1;
            activeChapterRef.current = null;
            loadedChapterIdRef.current = null;
            revisionRef.current = null;
            setActiveChapter(null);
            setLoadedChapterId(null);
            setCurrentRevision(null);
            applyingServerBodyRef.current = true;
            try {
              setBodyText("");
            } finally {
              applyingServerBodyRef.current = false;
            }
          }
        }
      }
      if (failures.length === 0) {
        accessDeniedRef.current = false;
        setAccessDenied(false);
      }
    }
    setLoadError(
      denied?.reason ?? failures[0]?.reason ?? (invalidNovel ? new Error("Invalid novel response") : null)
    );
  };

  useEffect(() => {
    void loadNovelData();
    return () => {
      listLoadTokenRef.current += 1;
      bodyLoadTokenRef.current += 1;
      if (saveTimerRef.current) clearTimeout(saveTimerRef.current);
    };
  }, [novelId]);

  useEffect(() => {
    api.llm
      .listModels({ limit: 50 })
      .then((models) => {
        const active = (models || []).filter((m) => m.status !== "offline");
        setAvailableModels(active);
        if (active.length > 0) {
          setSelectedModelId((prev) => prev || active[0].id);
        }
      })
      .catch((err) => {
        reportSafeDiagnostic("creator.models_load_failed", err);
      });
  }, [api]);

  // 切换章节前先保住当前草稿；新章节正文未读取成功时绝不展示旧章正文。
  const selectChapter = (chapter: Chapter) => {
    if (
      chapter.id !== activeChapterRef.current?.id &&
      (dirtyRef.current || saveInFlightRef.current || saveIntentRef.current)
    ) {
      setSwitchBlocked(true);
      return;
    }
    if (chapter.id === loadedChapterIdRef.current && !bodyLoadError) return;
    activeChapterRef.current = chapter;
    setActiveChapter(chapter);
    setSwitchBlocked(false);
    void loadChapterBody(chapter.id);
  };

  const loadChapterBody = async (chapterId: string) => {
    if (dirtyRef.current || saveInFlightRef.current || saveIntentRef.current || accessDeniedRef.current) return;
    const token = ++bodyLoadTokenRef.current;
    setBodyLoading(true);
    setBodyLoadError(null);
    try {
      const res = await api.content.getChapterBody(chapterId);
      if (token !== bodyLoadTokenRef.current || activeChapterRef.current?.id !== chapterId) return;
      if (res.chapter?.id !== chapterId || !isValidRevision(res.chapter.revision) || typeof res.body !== "string") {
        throw new Error("Invalid chapter body response");
      }
      if (dirtyRef.current || saveInFlightRef.current || accessDeniedRef.current) return;
      applyingServerBodyRef.current = true;
      try {
        setBodyText(res.body);
      } finally {
        applyingServerBodyRef.current = false;
      }
      loadedChapterIdRef.current = chapterId;
      revisionRef.current = res.chapter.revision;
      setLoadedChapterId(chapterId);
      setCurrentRevision(res.chapter.revision);
      setLastSaveTime("");
      setSaveStatus("saved");
      setBodyLoadError(null);
    } catch (err) {
      if (token !== bodyLoadTokenRef.current || activeChapterRef.current?.id !== chapterId) return;
      reportSafeDiagnostic("creator.body_load_failed", err);
      setBodyLoadError(err);
      if (isApiError(err) && [403, 404].includes(err.httpStatus)) {
        accessDeniedRef.current = true;
        setAccessDenied(true);
      }
    } finally {
      if (token === bodyLoadTokenRef.current) setBodyLoading(false);
    }
  };

  useEffect(() => {
    const warnBeforeLeave = (event: BeforeUnloadEvent) => {
      if (!dirtyRef.current && !saveInFlightRef.current && !saveIntentRef.current) return;
      event.preventDefault();
      event.returnValue = "";
    };
    window.addEventListener("beforeunload", warnBeforeLeave);
    return () => window.removeEventListener("beforeunload", warnBeforeLeave);
  }, []);

  // 每次写只冻结一份 payload/key。规范化为 LF 换行，支持 409 冲突核对。
  const handleAutoSave = async (rawText?: string, retry = false) => {
    const chapter = activeChapterRef.current;
    const textToSave = rawText !== undefined ? rawText : bodyTextRef.current;
    if (
      !chapter ||
      loadedChapterIdRef.current !== chapter.id ||
      !isValidRevision(revisionRef.current) ||
      isComposingRef.current ||
      saveInFlightRef.current ||
      accessDeniedRef.current ||
      (saveErrorRef.current && !retry)
    ) {
      return;
    }
    if (!dirtyRef.current && !saveIntentRef.current) return;

    const normalizedBody = normalizeLf(textToSave);
    const intent = saveIntentRef.current ?? {
      chapterId: chapter.id,
      body: normalizedBody,
      expectedRevision: revisionRef.current,
      idempotencyKey: crypto.randomUUID(),
      generation: editGenerationRef.current,
    };
    if (intent.chapterId !== chapter.id) return;
    saveIntentRef.current = intent;
    saveInFlightRef.current = true;
    setSaveStatus("saving");
    try {
      const res = await api.content.saveChapterBody(
        intent.chapterId,
        {
          body: intent.body,
          expected_revision: intent.expectedRevision,
        },
        intent.idempotencyKey
      );
      if (
        res.chapter?.id !== intent.chapterId ||
        !isValidRevision(res.chapter.revision) ||
        normalizeLf(res.body) !== intent.body
      ) {
        throw new Error("Invalid save response");
      }
      revisionRef.current = res.chapter.revision;
      setCurrentRevision(res.chapter.revision);
      saveIntentRef.current = null;
      saveErrorRef.current = null;
      setSaveError(null);
      if (editGenerationRef.current === intent.generation) {
        dirtyRef.current = false;
        setSaveStatus("saved");
      } else {
        setSaveStatus("unsaved");
        if (saveTimerRef.current) clearTimeout(saveTimerRef.current);
        saveTimerRef.current = setTimeout(() => {
          if (!isComposingRef.current) void handleAutoSave(bodyTextRef.current);
        }, 1200);
      }
      const d = new Date();
      setLastSaveTime(
        `${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}:${String(d.getSeconds()).padStart(2, "0")}`
      );
    } catch (err) {
      reportSafeDiagnostic("creator.body_save_failed", err);
      saveErrorRef.current = err;
      setSaveError(err);
      setSaveStatus("unsaved");
      if (isApiError(err)) {
        if (err.httpStatus === 409) {
          // 409 版本冲突：弹出三方核对弹窗，不自动丢弃本地修改
          setConflictModalOpen(true);
        } else if ([403, 404].includes(err.httpStatus)) {
          accessDeniedRef.current = true;
          setAccessDenied(true);
        }
      }
    } finally {
      saveInFlightRef.current = false;
    }
  };

  // 409 冲突解决回调：采用服务器版本
  const handleUseServer = (serverBody: string, serverRevision: number) => {
    applyingServerBodyRef.current = true;
    try {
      setBodyText(serverBody);
    } finally {
      applyingServerBodyRef.current = false;
    }
    revisionRef.current = serverRevision;
    setCurrentRevision(serverRevision);
    dirtyRef.current = false;
    saveIntentRef.current = null;
    saveErrorRef.current = null;
    setSaveError(null);
    setSaveStatus("saved");
    setConflictModalOpen(false);
  };

  // 409 冲突解决回调：以最新版本为基准覆盖保存本地草稿
  const handleOverwriteWithLocal = (serverRevision: number) => {
    revisionRef.current = serverRevision;
    setCurrentRevision(serverRevision);
    saveIntentRef.current = null;
    saveErrorRef.current = null;
    setSaveError(null);
    setConflictModalOpen(false);
    void handleAutoSave(bodyTextRef.current);
  };

  // 新建章节到后端
  const handleCreateChapter = async () => {
    if (
      !newChapterTitle.trim() ||
      !novelId ||
      !chaptersLoaded ||
      loadError ||
      dirtyRef.current ||
      saveInFlightRef.current ||
      saveIntentRef.current
    ) {
      setSwitchBlocked(true);
      return;
    }
    setCreatingChapter(true);
    try {
      let volId = volumes[0]?.id;
      if (!volId) {
        const createdVol = await api.content.createVolume(novelId, {
          title: "第一卷 正文卷",
          summary: "长篇主线第一卷",
        });
        setVolumes([createdVol]);
        volId = createdVol.id;
      }

      const createdChap = await api.content.createChapter(novelId, {
        volume_id: volId,
        title: newChapterTitle.trim(),
      });

      if (!isValidRevision(createdChap.revision)) throw new Error("Invalid chapter revision");
      setChapters([...chapters, createdChap]);
      activeChapterRef.current = createdChap;
      setActiveChapter(createdChap);
      setNewChapterTitle("");
      setCreateChapterModalOpen(false);
      applyingServerBodyRef.current = true;
      try {
        setBodyText("");
      } finally {
        applyingServerBodyRef.current = false;
      }
      loadedChapterIdRef.current = createdChap.id;
      revisionRef.current = createdChap.revision;
      dirtyRef.current = false;
      setLoadedChapterId(createdChap.id);
      setCurrentRevision(createdChap.revision);
      setSaveError(null);
      saveErrorRef.current = null;
      setSaveStatus("saved");
    } catch (err: any) {
      alert(safeErrorText(err, t));
    } finally {
      setCreatingChapter(false);
    }
  };

  const openAgentWorkbench = (startOffset = 0, endOffset = 0) => {
    if (!activeChapterRef.current || !isValidRevision(revisionRef.current) || loadedChapterIdRef.current !== activeChapterRef.current.id) return;
    if (dirtyRef.current || saveInFlightRef.current || saveIntentRef.current || saveErrorRef.current) {
      setSwitchBlocked(true);
      return;
    }
    const source = normalizeLf(bodyTextRef.current);
    const codepoints = Array.from(source);
    const start = Math.max(0, Math.min(startOffset, codepoints.length));
    const end = Math.max(start, Math.min(endOffset, codepoints.length));
    const selection = end > start ? { text: codepoints.slice(start, end).join(""), startOffset: start, endOffset: end } : null;
    setWorkbenchSelection(selection);
    setWorkbenchOpen(true);
  };

  const handleAgentAdopt = (artifact: ArtifactView, selection: { text: string; startOffset: number; endOffset: number } | null | undefined, sourceSnapshot: string) => {
    if (normalizeLf(bodyTextRef.current) !== sourceSnapshot || artifact.is_partial || !artifact.text_content || artifact.target_resource_id !== activeChapterRef.current?.id || artifact.source_revision !== revisionRef.current) return;
    const codepoints = Array.from(normalizeLf(bodyTextRef.current));
    let next: string;
    if (selection) {
      if (codepoints.slice(selection.startOffset, selection.endOffset).join("") !== selection.text) return;
      next = [...codepoints.slice(0, selection.startOffset), ...Array.from(artifact.text_content), ...codepoints.slice(selection.endOffset)].join("");
    } else {
      next = artifact.text_content;
    }
    handleBodyChange(normalizeLf(next));
    setWorkbenchOpen(false);
  };

  // Compliance suggestions are code-point anchored and still require author adoption.
  const handleAdoptComplianceSuggestion = (startOffset: number, endOffset: number, replacement: string) => {
    const runes = Array.from(bodyText);
    if (startOffset < 0 || endOffset > runes.length || startOffset > endOffset) return;
    const newRunes = [...runes.slice(0, startOffset), ...Array.from(replacement), ...runes.slice(endOffset)];
    handleBodyChange(normalizeLf(newRunes.join("")));
  };

  const handleOpenWorkbench = (_selectedText: string, startUtf16: number, endUtf16: number) => {
    const source = normalizeLf(bodyTextRef.current);
    const start = countCodePoints(source.slice(0, startUtf16));
    const end = countCodePoints(source.slice(0, endUtf16));
    openAgentWorkbench(start, end);
  };


  const downloadDraft = () => {
    downloadEditorDraft(bodyText, activeChapterRef.current?.id || "draft");
  };

  if (accessDenied) {
    return (
      <div className="min-h-screen bg-[var(--bg-canvas)] p-6" role="alert">
        <ErrorPresenter error={loadError ?? bodyLoadError ?? saveError} />
        {dirtyRef.current && <Button onClick={downloadDraft}>{t("editor.downloadDraft")}</Button>}
        <Button variant="outline" onClick={onBack}>
          {t("common.back")}
        </Button>
      </div>
    );
  }

  return (
    <div className="h-screen w-screen flex flex-col overflow-hidden novel-paper-bg text-stone-900 dark:text-stone-100 select-none">
      {/* 1. Top Bar */}
      <header className="h-11 border-b border-stone-200/80 dark:border-stone-800/80 px-3 flex items-center justify-between text-xs shrink-0 bg-[#FAF7EE] dark:bg-[#181613]">
        <div className="flex items-center gap-2">
          <div className="flex items-center gap-1.5 px-2 py-1 rounded hover:bg-stone-200/60 dark:hover:bg-stone-800 cursor-pointer font-medium max-w-[200px] truncate">
            <BookOpen className="w-3.5 h-3.5 text-stone-600 dark:text-stone-400" />
            <span className="truncate">{novel?.title || "未命名作品"}</span>
            <ChevronDown className="w-3 h-3 text-stone-400" />
          </div>

          <button className="p-1 rounded hover:bg-stone-200/60 dark:hover:bg-stone-800 text-stone-500">
            <Settings className="w-3.5 h-3.5" />
          </button>
          <button
            onClick={() => setSidebarOpen(!sidebarOpen)}
            className="p-1 rounded hover:bg-stone-200/60 dark:hover:bg-stone-800 text-stone-500"
            title="折叠/展开章节侧栏"
          >
            <PanelLeft className="w-3.5 h-3.5" />
          </button>

          <div className="h-3.5 w-px bg-stone-300 dark:bg-stone-700 mx-1" />

          <button
            onClick={() => {
              if (dirtyRef.current || saveInFlightRef.current || saveIntentRef.current) {
                setSwitchBlocked(true);
              } else {
                onBack();
              }
            }}
            className="p-1 rounded hover:bg-stone-200/60 dark:hover:bg-stone-800 text-stone-600 dark:text-stone-300 flex items-center gap-1 font-medium"
            title="返回书架与首页"
          >
            <ArrowLeft className="w-4 h-4" />
          </button>

          <div className="flex items-center gap-1 ml-1">
            <button className="flex items-center gap-1 px-2.5 py-1 rounded bg-[#EAE2D0] dark:bg-stone-800 text-[#5C4A32] dark:text-stone-200 font-semibold shadow-2xs">
              <Edit3 className="w-3 h-3 text-amber-700 dark:text-amber-400" />
              <span>写作</span>
            </button>
            <button className="flex items-center gap-1 px-2 py-1 rounded hover:bg-stone-200/50 dark:hover:bg-stone-800 text-stone-600 dark:text-stone-400">
              <Layers className="w-3 h-3" />
              <span>小说架构</span>
            </button>
            <button className="flex items-center gap-1 px-2 py-1 rounded hover:bg-stone-200/50 dark:hover:bg-stone-800 text-stone-600 dark:text-stone-400">
              <SlidersHorizontal className="w-3 h-3" />
              <span>工具</span>
              <ChevronDown className="w-2.5 h-2.5 text-stone-400" />
            </button>
            <button className="flex items-center gap-1 px-2 py-1 rounded hover:bg-stone-200/50 dark:hover:bg-stone-800 text-stone-600 dark:text-stone-400">
              <Search className="w-3 h-3" />
              <span>搜索</span>
            </button>
            <button className="flex items-center gap-1 px-2 py-1 rounded hover:bg-stone-200/50 dark:hover:bg-stone-800 text-stone-600 dark:text-stone-400">
              <Monitor className="w-3 h-3" />
              <span>模式</span>
            </button>
          </div>
        </div>

        {/* Right Section */}
        <div className="flex items-center gap-3 text-stone-600 dark:text-stone-300">
          <Button variant="ghost" disabled={!novel || saveStatus === "saving"} onClick={() => setExportOpen(true)} icon={<Download className="h-4 w-4" />}>{t("export.title")}</Button>
          <span className="font-mono text-stone-700 dark:text-stone-300 font-medium">
            {loadedChapterId === activeChapter?.id ? wordCount.toLocaleString() : "—"} 字
          </span>
          <span className="flex items-center gap-0.5 text-stone-500 font-mono">
            <Edit3 className="w-3 h-3" />
            <span>rev.{loadedChapterId === activeChapter?.id ? currentRevision ?? "—" : "—"}</span>
          </span>
          <div className="flex items-center gap-1 text-emerald-700 dark:text-emerald-400 font-medium">
            {loadedChapterId !== activeChapter?.id ? (
              <span role="status">{bodyLoading ? t("editor.bodyLoading") : t("editor.bodyUnavailable")}</span>
            ) : saveStatus === "saving" ? (
              <>
                <CloudUpload className="w-3.5 h-3.5 animate-pulse text-amber-500" />
                <span>{t("common.saving")}</span>
              </>
            ) : saveStatus === "saved" ? (
              <>
                <CheckCircle2 className="w-3.5 h-3.5" />
                <span>{lastSaveTime ? t("editor.savedAt", lastSaveTime) : t("ws.saved")}</span>
              </>
            ) : (
              <span className="text-amber-700 dark:text-amber-300" role="status">
                {t("editor.unsaved")}
              </span>
            )}
          </div>

          <div className="h-3.5 w-px bg-stone-300 dark:bg-stone-700" />

          {/* 字号微调 */}
          <div className="flex items-center gap-1 border border-stone-300/80 dark:border-stone-700 rounded px-1.5 py-0.5">
            <button onClick={() => setFontSize(Math.max(14, fontSize - 1))} className="px-1 hover:text-amber-600 font-bold">
              -
            </button>
            <span className="font-mono px-1">{fontSize}</span>
            <button onClick={() => setFontSize(Math.min(24, fontSize + 1))} className="px-1 hover:text-amber-600 font-bold">
              +
            </button>
          </div>

          <button className="flex items-center gap-1 px-2 py-1 rounded hover:bg-stone-200/50 dark:hover:bg-stone-800">
            <Bot className="w-3.5 h-3.5 text-amber-600" />
            <span>AI</span>
          </button>
          <button className="flex items-center gap-1 px-2 py-1 rounded hover:bg-stone-200/50 dark:hover:bg-stone-800">
            <Settings className="w-3.5 h-3.5" />
            <span>设置</span>
          </button>
          <button
            onClick={() => void handleAutoSave(bodyText)}
            disabled={
              !activeChapter ||
              loadedChapterId !== activeChapter.id ||
              !!saveError ||
              bodyLoading ||
              saveStatus === "saving" ||
              saveStatus === "saved"
            }
            className="flex items-center gap-1 px-2.5 py-1 rounded bg-amber-600 hover:bg-amber-700 text-white font-semibold transition-colors shadow-2xs disabled:opacity-50"
          >
            <Save className="w-3.5 h-3.5" />
            <span>保存</span>
          </button>
          {activeChapter && (
            activeChapter.status === "confirmed" ? (
              <span className="flex items-center gap-1 px-2 py-1 rounded bg-emerald-100 dark:bg-emerald-950/40 text-emerald-800 dark:text-emerald-300 font-semibold text-xs">
                <CheckCircle2 className="w-3.5 h-3.5 text-emerald-600" />
                <span>已定稿</span>
              </span>
            ) : (
              <button
                onClick={() => setConfirmModalOpen(true)}
                disabled={!activeChapter || loadedChapterId !== activeChapter.id || dirtyRef.current || saveStatus !== "saved"}
                className="flex items-center gap-1 px-2.5 py-1 rounded border border-emerald-600/80 text-emerald-700 dark:text-emerald-300 hover:bg-emerald-50 dark:hover:bg-emerald-950/30 font-semibold transition-colors disabled:opacity-50"
                title="确认本章正文并写入世界观事实账本"
              >
                <CheckCircle2 className="w-3.5 h-3.5 text-emerald-600" />
                <span>定稿入账</span>
              </button>
            )
          )}
        </div>
      </header>

      {(loadError != null || bodyLoadError != null || saveError != null || switchBlocked) && (
        <div className="max-h-[40vh] overflow-y-auto px-4 py-2 bg-[#FAF7EE] dark:bg-[#181613]" role="region" aria-label={t("common.details")}>
          {loadError != null && (
            <ErrorPresenter error={loadError} context={{ operationLabel: t("editor.loadFailed") }} onRetry={() => void loadNovelData()} />
          )}
          {bodyLoadError != null && (
            <ErrorPresenter
              error={bodyLoadError}
              context={{ operationLabel: t("editor.bodyUnavailable") }}
              onRetry={() => activeChapterRef.current && void loadChapterBody(activeChapterRef.current.id)}
            />
          )}
          {saveError != null && (
            <>
              <ErrorPresenter
                error={saveError}
                context={{ operationLabel: t("editor.saveFailed"), hasDraft: true }}
                onRetry={
                  saveIntentRef.current &&
                  (!isApiError(saveError) || saveError.httpStatus >= 500 || saveError.code === "platform.operation_in_progress")
                    ? () => void handleAutoSave(undefined, true)
                    : undefined
                }
              />
              {(!isApiError(saveError) || saveError.httpStatus >= 500 || saveError.code === "platform.operation_in_progress") && (
                <p className="text-sm text-amber-800 dark:text-amber-300">{t("editor.saveUnknown")}</p>
              )}
            </>
          )}
          {switchBlocked && <p role="alert" className="text-sm text-amber-800 dark:text-amber-300">{t("editor.switchBlocked")}</p>}
          {(dirtyRef.current || saveIntentRef.current) && (
            <Button variant="outline" onClick={downloadDraft}>{t("editor.downloadDraft")}</Button>
          )}
        </div>
      )}

      {/* 2. Workspace Body */}
      <div className="flex-1 flex overflow-hidden">
        {/* Left Sidebar */}
        {sidebarOpen && (
          <aside className="w-64 border-r border-stone-200/80 dark:border-stone-800/80 flex flex-col bg-[#FAF7EE] dark:bg-[#181613] shrink-0">
            <div className="h-9 border-b border-stone-200/80 dark:border-stone-800 flex items-center px-2 text-xs">
              <button
                onClick={() => setActiveTab("settings")}
                className={`flex-1 py-1.5 text-center transition-colors ${
                  activeTab === "settings" ? "font-bold text-stone-900 border-b-2 border-amber-600" : "text-stone-500 hover:text-stone-800"
                }`}
              >
                设定 ({characters.length})
              </button>
              <button
                onClick={() => setActiveTab("outline")}
                className={`flex-1 py-1.5 text-center transition-colors ${
                  activeTab === "outline" ? "font-bold text-stone-900 border-b-2 border-amber-600" : "text-stone-500 hover:text-stone-800"
                }`}
              >
                章纲
              </button>
              <button
                onClick={() => setActiveTab("catalog")}
                className={`flex-1 py-1.5 text-center transition-colors ${
                  activeTab === "catalog" ? "font-bold text-stone-900 border-b-2 border-amber-600" : "text-stone-500 hover:text-stone-800"
                }`}
              >
                目录 ({chapters.length})
              </button>
              <button
                onClick={() => setActiveTab("docs")}
                className={`flex-1 py-1.5 text-center transition-colors ${
                  activeTab === "docs" ? "font-bold text-stone-900 border-b-2 border-amber-600" : "text-stone-500 hover:text-stone-800"
                }`}
              >
                ··· 文档
              </button>
            </div>

            <div className="p-2.5 pb-1">
              <div className="relative">
                <Search className="w-3.5 h-3.5 absolute left-2.5 top-1/2 -translate-y-1/2 text-stone-400" />
                <input
                  type="text"
                  placeholder="搜索章节与设定..."
                  className="w-full h-7 rounded-md pl-8 pr-2 text-xs bg-stone-200/50 dark:bg-stone-800/80 border-none placeholder:text-stone-400 focus:outline-none focus:ring-1 focus:ring-amber-500"
                />
              </div>
            </div>

            <div className="px-3 py-1.5 flex items-center justify-between text-[11px] text-stone-500 border-b border-stone-200/60 dark:border-stone-800/60">
              <span>{chapters.length} 章节 · {loadedChapterId === activeChapter?.id ? wordCount.toLocaleString() : "—"} 字</span>
              <div className="flex items-center gap-2">
                <button
                  disabled={!chaptersLoaded || loadError != null || dirtyRef.current || saveInFlightRef.current}
                  onClick={() => setCreateChapterModalOpen(true)}
                  title="添加新章节"
                  className="hover:text-amber-600 flex items-center gap-0.5 text-amber-600 font-semibold"
                >
                  <Plus className="w-3 h-3" />
                  <span>加章</span>
                </button>
                <button title="帮助" className="hover:text-stone-800"><HelpCircle className="w-3 h-3" /></button>
              </div>
            </div>

            <div className="flex-1 overflow-y-auto p-2 space-y-1 text-xs">
              {activeTab === "catalog" && (
                <>
                  {!chaptersLoaded ? (
                    <div className="p-4 text-stone-500">{loadError != null ? t("editor.loadFailed") : t("common.loading")}</div>
                  ) : chapters.length === 0 ? (
                    <div className="p-6 text-center text-xs text-stone-400 space-y-2">
                      <FileText className="w-6 h-6 mx-auto text-stone-300" />
                      <p>暂无章节</p>
                      <Button
                        size="sm"
                        variant="outline"
                        icon={<Plus className="w-3 h-3" />}
                        disabled={loadError != null || dirtyRef.current || saveInFlightRef.current}
                        onClick={() => setCreateChapterModalOpen(true)}
                      >
                        新建第一章
                      </Button>
                    </div>
                  ) : (
                    <>
                      <div className="flex items-center justify-between px-2 py-1 text-stone-600 dark:text-stone-400 font-semibold cursor-pointer">
                        <div className="flex items-center gap-1">
                          <ChevronDown className="w-3.5 h-3.5" />
                          <span>{volumes[0]?.title || t("ws.unassigned")}</span>
                        </div>
                        <span className="text-[11px] text-stone-400 font-mono">{chapters.length} 章</span>
                      </div>

                      {chapters.map((chap, idx) => {
                        const isSelected = activeChapter?.id === chap.id;
                        return (
                          <div
                            key={chap.id}
                            onClick={() => selectChapter(chap)}
                            className={`p-2 rounded-lg border space-y-1 cursor-pointer transition-all ${
                              isSelected
                                ? "bg-[#FAF2DC] dark:bg-amber-950/40 border-amber-300/80 dark:border-amber-700/60"
                                : "border-transparent hover:bg-stone-200/40 dark:hover:bg-stone-800/50"
                            }`}
                          >
                            <div className="flex items-start gap-2">
                              <span className="px-1.5 py-0.5 rounded bg-amber-200/80 dark:bg-amber-800 text-amber-900 dark:text-amber-200 font-mono font-bold text-[10px]">
                                {idx + 1}
                              </span>
                              <div className="flex-1 font-medium text-stone-900 dark:text-stone-100 line-clamp-1 leading-snug">
                                {chap.title}
                              </div>
                            </div>
                            <div className="text-[11px] text-stone-500 font-mono pl-6">
                              {chap.word_count.toLocaleString()}字
                            </div>
                          </div>
                        );
                      })}
                    </>
                  )}
                </>
              )}

              {activeTab === "settings" && (
                <div className="space-y-2 p-1">
                  <div className="font-semibold text-stone-500 text-[11px]">登场人物 ({characters.length})</div>
                  {!charactersLoaded ? (
                    <div className="p-4 text-center text-xs text-stone-400">{loadError != null ? t("editor.loadFailed") : t("common.loading")}</div>
                  ) : characters.length === 0 ? (
                    <div className="p-4 text-center text-xs text-stone-400">暂无角色记录</div>
                  ) : (
                    characters.map((c) => (
                      <div key={c.id} className="p-2 rounded-lg bg-stone-100 dark:bg-stone-800/60 border border-stone-200/60 dark:border-stone-700/60 space-y-1">
                        <div className="flex items-center justify-between">
                          <span className="font-bold text-stone-900 dark:text-stone-100">{c.name}</span>
                          <Badge size="sm" variant={c.role_type === "protagonist" ? "success" : "secondary"}>
                            {c.role_type === "protagonist" ? "主角" : "配角"}
                          </Badge>
                        </div>
                        <p className="text-[11px] text-stone-500 line-clamp-2">{c.summary || "暂无描述"}</p>
                      </div>
                    ))
                  )}
                </div>
              )}
            </div>

            <div className="h-8 border-t border-stone-200/80 dark:border-stone-800 px-3 flex items-center justify-between text-[11px] text-stone-400">
              <span className="font-mono truncate max-w-[120px]">ID: {activeChapter?.id || novelId}</span>
              <div className="flex items-center gap-2">
                <ShieldCheck className="w-3.5 h-3.5 hover:text-stone-600" />
                <HelpCircle className="w-3.5 h-3.5 hover:text-stone-600" />
                <Trash2 className="w-3.5 h-3.5 hover:text-stone-600" />
                <Sparkles className="w-3.5 h-3.5 hover:text-amber-600" />
              </div>
            </div>
          </aside>
        )}

        {/* Center Main Writing Area */}
        <main className="flex-1 flex flex-col overflow-y-auto bg-[#FAF7EE] dark:bg-[#181613]">
          <div className="w-full max-w-4xl mx-auto px-8 py-6 space-y-4">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <FileText className="w-5 h-5 text-stone-600 dark:text-stone-400" />
                <h1 className="text-xl font-bold text-stone-900 dark:text-stone-100 tracking-tight">
                  {activeChapter?.title || (novel ? `《${novel.title}》正文创作` : "正文写作工作台")}
                </h1>
              </div>

              <div className="flex items-center gap-2 text-xs">
                {/* 动态推理模型自选下拉器 */}
                <div className="flex items-center gap-1 px-2 py-1 rounded-md bg-white dark:bg-stone-800 border border-stone-300/80 dark:border-stone-700 text-stone-700 dark:text-stone-300 shadow-2xs font-medium">
                  <Bot className="w-3.5 h-3.5 text-stone-500 shrink-0" />
                  <span className="text-stone-500 shrink-0">{t("editor.modelLabel")}:</span>
                  <select
                    value={selectedModelId}
                    onChange={(e) => setSelectedModelId(e.target.value)}
                    className="bg-transparent font-medium text-stone-700 dark:text-stone-200 outline-none cursor-pointer max-w-[130px] truncate text-xs"
                    title={t("editor.modelSelectorTip")}
                  >
                    {availableModels.length === 0 ? (
                      <option value="">{t("editor.modelLoading")}</option>
                    ) : (
                      availableModels.map((m) => (
                        <option key={m.id} value={m.id}>
                          {m.name || m.id}
                        </option>
                      ))
                    )}
                  </select>
                </div>

                <button
                  onClick={() => openAgentWorkbench()}
                  className="flex items-center gap-1 rounded-md border border-stone-300/80 bg-white px-2.5 py-1 font-medium text-stone-700 shadow-2xs hover:bg-stone-50 dark:border-stone-700 dark:bg-stone-800 dark:text-stone-300"
                  disabled={!activeChapter || saveStatus !== "saved"}
                >
                  <Bot className="h-3.5 w-3.5 text-amber-600" />
                  <span>{t("agentWorkbench.open")}</span>
                </button>
                <button
                  onClick={() => setComplianceModalOpen(true)}
                  className="flex items-center gap-1 px-2.5 py-1 rounded-md bg-white dark:bg-stone-800 border border-stone-300/80 dark:border-stone-700 text-stone-700 dark:text-stone-300 hover:bg-stone-50 shadow-2xs font-medium"
                  title={t("editor.complianceCheckTip")}
                >
                  <ShieldCheck className="w-3.5 h-3.5 text-emerald-600" />
                  <span>{t("editor.complianceCheck")}</span>
                </button>
              </div>
            </div>

            {/* Atmosphere Callout */}
            {activeChapter && (
              <div className="px-4 py-3 rounded-lg bg-[#EEF4FB] dark:bg-sky-950/30 border border-[#DDE8F4] dark:border-sky-900/40 text-xs font-semibold text-stone-800 dark:text-stone-200">
                {activeChapter.title}
              </div>
            )}

            {/* Body Text Editor Area (原生纯文本多行文本框) */}
            <div className="novel-reading-font min-h-[500px] text-stone-800 dark:text-stone-200 focus:outline-none">
              {!chaptersLoaded ? (
                <div className="p-12 text-center text-stone-500" role="status">
                  {loadError != null ? t("editor.loadFailed") : t("common.loading")}
                </div>
              ) : chapters.length === 0 ? (
                <div className="p-12 text-center text-stone-400 space-y-3 font-sans">
                  <FileText className="w-8 h-8 mx-auto text-stone-300 dark:text-stone-700" />
                  <p className="text-sm font-medium text-stone-600 dark:text-stone-400">当前作品暂无章节正文</p>
                  <p className="text-xs text-stone-400">点击下方按钮在数据库中创建第一章</p>
                  <Button
                    variant="primary"
                    size="sm"
                    icon={<Plus className="w-3.5 h-3.5" />}
                    disabled={loadError != null || dirtyRef.current || saveInFlightRef.current}
                    onClick={() => setCreateChapterModalOpen(true)}
                  >
                    新建第一章
                  </Button>
                </div>
              ) : loadedChapterId === activeChapter?.id ? (
                <PlainTextEditor
                  value={bodyText}
                  onChange={handleBodyChange}
                  onSave={() => void handleAutoSave(bodyText)}
                  onOpenWorkbench={handleOpenWorkbench}
                  fontSize={fontSize}
                  disabled={bodyLoading}
                />
              ) : (
                <div className="p-12 text-center text-stone-500" role="status">
                  {bodyLoading ? t("editor.bodyLoading") : t("editor.bodyUnavailable")}
                </div>
              )}
            </div>
          </div>
        </main>
      </div>

      {/* 3. Bottom Status Bar */}
      <footer className="h-7 border-t border-stone-200/80 dark:border-stone-800 px-4 flex items-center justify-between text-xs text-stone-500 shrink-0 bg-[#FAF7EE] dark:bg-[#181613]">
        <div className="flex items-center gap-2">
          <span className="font-medium text-stone-700 dark:text-stone-300">
            {activeChapter?.title || "暂无章节"} · {loadedChapterId === activeChapter?.id ? wordCount.toLocaleString() : "—"}字
          </span>
        </div>

        <div className="flex items-center gap-2">
          <div className="flex items-center gap-1 px-1.5 py-0.5 rounded bg-amber-100 dark:bg-amber-950/40 text-amber-800 dark:text-amber-300 text-[11px] font-semibold cursor-pointer">
            <AlertTriangle className="w-3 h-3 text-amber-600" />
            <span>体检</span>
          </div>
        </div>
      </footer>

      {/* Create Chapter Modal */}
      <Modal
        open={createChapterModalOpen}
        onClose={() => setCreateChapterModalOpen(false)}
        title="在数据库中创建新章节"
        footer={
          <>
            <Button variant="ghost" onClick={() => setCreateChapterModalOpen(false)}>
              取消
            </Button>
            <Button variant="primary" loading={creatingChapter} onClick={handleCreateChapter}>
              确认创建
            </Button>
          </>
        }
      >
        <div className="space-y-3">
          <label className="block text-xs font-medium text-stone-700 dark:text-stone-300">章节标题</label>
          <Input
            value={newChapterTitle}
            onChange={(e) => setNewChapterTitle(e.target.value)}
            onPressEnter={() => {
              if (!creatingChapter && newChapterTitle.trim()) {
                handleCreateChapter();
              }
            }}
            placeholder="例如：第1章 诸神审判，我以魔血染青天"
          />
        </div>
      </Modal>

      {/* 409 版本冲突解决弹窗 */}
      {activeChapter && (
        <ConflictResolutionModal
          open={conflictModalOpen}
          chapterId={activeChapter.id}
          localBody={bodyText}
          onClose={() => setConflictModalOpen(false)}
          onUseServer={handleUseServer}
          onOverwriteWithLocal={handleOverwriteWithLocal}
        />
      )}

      {/* 章节正式确认与事实入账弹窗 */}
      {activeChapter && (
        <ConfirmChapterModal
          open={confirmModalOpen}
          novelId={novelId}
          chapter={activeChapter}
          onClose={() => setConfirmModalOpen(false)}
          onSuccess={(confirmation) => {
            const updated = { ...activeChapter, status: "confirmed" as const };
            setActiveChapter(updated);
            activeChapterRef.current = updated;
            setChapters(chapters.map((c) => (c.id === updated.id ? updated : c)));
          }}
        />
      )}

      <AgentWorkbench
        open={workbenchOpen}
        novelId={novelId}
        chapterId={activeChapter?.id || ""}
        chapterRevision={currentRevision}
        chapterText={bodyText}
        models={availableModels}
        selectedModelId={selectedModelId}
        initialSelection={workbenchSelection}
        onClose={() => setWorkbenchOpen(false)}
        onAdopt={handleAgentAdopt}
      />

      {/* Compliance review suggestions require the author to apply them. */}
      <ExportNovelDialog key={novelId} open={exportOpen} novelId={novelId} onClose={() => setExportOpen(false)} />
      <ComplianceReviewModal
        open={complianceModalOpen}
        onClose={() => setComplianceModalOpen(false)}
        api={api}
        novelId={novelId}
        chapterId={activeChapter?.id || ""}
        chapterRevision={currentRevision || 1}
        chapterTitle={activeChapter?.title || ""}
        bodyText={bodyText}
        onAdoptSuggestion={handleAdoptComplianceSuggestion}
      />
    </div>
  );
}

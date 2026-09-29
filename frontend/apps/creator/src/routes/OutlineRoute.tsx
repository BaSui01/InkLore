import React, { useState, useEffect, useRef } from "react";
import { Plus, BookOpen, Sparkles, ChevronRight, FileText, Layers } from "lucide-react";
import { Button, Card, Badge, Modal, Input, Textarea, ErrorPresenter } from "@inklore/ui";
import { getApiSDK, reportSafeDiagnostic, safeErrorText } from "@inklore/transport";
import { useTranslation } from "@inklore/i18n";
import { Chapter, Novel, Scene, Volume, OutlineNode } from "@inklore/contracts";

export interface OutlineRouteProps {
  novelId?: string;
  onOpenChapter?: (chapterId: string) => void;
}

export function OutlineRoute({ novelId: initialNovelId, onOpenChapter }: OutlineRouteProps) {
  const { t } = useTranslation();
  const api = getApiSDK();

  const [currentNovelId, setCurrentNovelId] = useState<string | null>(initialNovelId || null);
  const [novel, setNovel] = useState<Novel | null>(null);
  const [volumes, setVolumes] = useState<Volume[]>([]);
  const [chapters, setChapters] = useState<Chapter[]>([]);
  const [scenes, setScenes] = useState<Scene[]>([]);
  const [nodes, setNodes] = useState<OutlineNode[]>([]);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState<unknown>(null);

  // 模态框状态
  const [createChapterModalOpen, setCreateChapterModalOpen] = useState(false);
  const [newChapterTitle, setNewChapterTitle] = useState("");
  const [creatingChapter, setCreatingChapter] = useState(false);

  const [createSceneModalOpen, setCreateSceneModalOpen] = useState(false);
  const [activeChapterForScene, setActiveChapterForScene] = useState<string | null>(null);
  const [newSceneTitle, setNewSceneTitle] = useState("");
  const [newSceneSummary, setNewSceneSummary] = useState("");
  const [creatingScene, setCreatingScene] = useState(false);

  // 1. 如果没有传入 novelId，先从书架读取最近一部作品
  useEffect(() => {
    if (initialNovelId) {
      setCurrentNovelId(initialNovelId);
      return;
    }
    let active = true;
    api.content
      .listNovels({ limit: 1 })
      .then((res) => {
        if (!active) return;
        if (res && res.length > 0) {
          setCurrentNovelId(res[0].id);
        }
      })
      .catch((err) => {
        reportSafeDiagnostic("creator.outline_fetch_novels_failed", err);
      });
    return () => {
      active = false;
    };
  }, [initialNovelId]);

  // 2. 加载大纲核心数据
  const loadOutlineData = async () => {
    if (!currentNovelId) return;
    setLoading(true);
    setLoadError(null);
    try {
      const [novelRes, volumesRes, chaptersRes, scenesRes, nodesRes] = await Promise.allSettled([
        api.content.getNovel(currentNovelId),
        api.content.listVolumes(currentNovelId),
        api.content.listChapters(currentNovelId),
        api.outline.listScenes(currentNovelId),
        api.outline.listNodes(currentNovelId),
      ]);

      if (novelRes.status === "fulfilled") setNovel(novelRes.value);
      if (volumesRes.status === "fulfilled") setVolumes(volumesRes.value);
      if (chaptersRes.status === "fulfilled") setChapters(chaptersRes.value);
      if (scenesRes.status === "fulfilled") setScenes(scenesRes.value);
      if (nodesRes.status === "fulfilled") setNodes(nodesRes.value);

      const firstFail = [novelRes, volumesRes, chaptersRes, scenesRes, nodesRes].find(
        (r): r is PromiseRejectedResult => r.status === "rejected"
      );
      if (firstFail && chaptersRes.status === "rejected") {
        setLoadError(firstFail.reason);
      }
    } catch (err) {
      reportSafeDiagnostic("creator.outline_load_failed", err);
      setLoadError(err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (currentNovelId) {
      void loadOutlineData();
    }
  }, [currentNovelId]);

  // 创建章节
  const handleCreateChapter = async () => {
    if (!currentNovelId || !newChapterTitle.trim()) return;
    setCreatingChapter(true);
    try {
      let volId = volumes[0]?.id;
      if (!volId) {
        const createdVol = await api.content.createVolume(currentNovelId, {
          title: "第一卷 正文卷",
          summary: "长篇主线第一卷",
        });
        setVolumes([createdVol]);
        volId = createdVol.id;
      }
      const created = await api.content.createChapter(currentNovelId, {
        volume_id: volId,
        title: newChapterTitle.trim(),
      });
      setChapters([...chapters, created]);
      setNewChapterTitle("");
      setCreateChapterModalOpen(false);
    } catch (err) {
      reportSafeDiagnostic("creator.create_chapter_failed", err);
      alert(safeErrorText(err, t));
    } finally {
      setCreatingChapter(false);
    }
  };

  // 创建场景
  const handleCreateScene = async () => {
    if (!currentNovelId || !activeChapterForScene || !newSceneTitle.trim()) return;
    setCreatingScene(true);
    try {
      const created = await api.outline.createScene(currentNovelId, {
        chapter_id: activeChapterForScene,
        title: newSceneTitle.trim(),
        summary: newSceneSummary.trim() || undefined,
        order_index: scenes.filter((s) => s.chapter_id === activeChapterForScene).length + 1,
      });
      setScenes([...scenes, created]);
      setNewSceneTitle("");
      setNewSceneSummary("");
      setCreateSceneModalOpen(false);
    } catch (err) {
      reportSafeDiagnostic("creator.create_scene_failed", err);
      alert(safeErrorText(err, t));
    } finally {
      setCreatingScene(false);
    }
  };

  if (!currentNovelId && !loading) {
    return (
      <div className="flex-1 flex flex-col items-center justify-center p-8 bg-neutral-50 dark:bg-neutral-950 text-neutral-500">
        <BookOpen className="w-12 h-12 text-neutral-300 dark:text-neutral-700 mb-4" />
        <h3 className="text-base font-semibold text-neutral-800 dark:text-neutral-200 mb-1">
          暂无选中的创作作品
        </h3>
        <p className="text-xs text-neutral-400 mb-4">请先在书架创建或打开一部作品进行大纲规划</p>
      </div>
    );
  }

  return (
    <div className="flex-1 flex flex-col overflow-hidden bg-neutral-50/50 dark:bg-neutral-950 p-6">
      {/* Header bar */}
      <div className="flex items-center justify-between pb-4 border-b border-neutral-200 dark:border-neutral-800 shrink-0">
        <div>
          <div className="flex items-center gap-2">
            <h2 className="text-xl font-bold text-neutral-900 dark:text-neutral-100">
              {novel ? `《${novel.title}》大纲蓝图` : "大纲与剧情节拍"}
            </h2>
            <Badge variant="outline" size="sm">
              共 {chapters.length} 章 · {scenes.length} 场景
            </Badge>
          </div>
          <p className="text-xs text-neutral-500 mt-0.5">
            三级大纲架构：总纲 (全书策略) → 卷纲 (阶段冲突) → 章节蓝图与微观场景规划
          </p>
        </div>

        <div className="flex items-center gap-2">
          <Button
            size="sm"
            variant="outline"
            icon={<Sparkles className="w-3.5 h-3.5 text-amber-500" />}
            onClick={() => alert("正在连接 Agent 大纲推演服务...")}
          >
            AI 蓝图推演
          </Button>
          <Button
            size="sm"
            variant="primary"
            icon={<Plus className="w-3.5 h-3.5" />}
            onClick={() => setCreateChapterModalOpen(true)}
          >
            新建章节卡片
          </Button>
        </div>
      </div>

      {/* 错误提示 */}
      {loadError != null && (
        <div className="pt-3">
          <ErrorPresenter error={loadError} onRetry={() => void loadOutlineData()} />
        </div>
      )}

      {/* Horizontal Scrolling Card Board (真实后端数据驱动) */}
      <div className="flex-1 overflow-x-auto overflow-y-hidden py-6 flex items-start gap-4">
        {chapters.length === 0 && !loading ? (
          <div className="w-full flex flex-col items-center justify-center p-12 text-center text-neutral-400 border-2 border-dashed border-neutral-200 dark:border-neutral-800 rounded-2xl">
            <Layers className="w-8 h-8 text-neutral-300 dark:text-neutral-700 mb-2" />
            <p className="text-sm font-medium text-neutral-600 dark:text-neutral-400 mb-1">
              当前作品尚未创建章节与大纲场景
            </p>
            <p className="text-xs text-neutral-400 mb-4">
              点击下方按钮建立第一章剧情卡片
            </p>
            <Button
              size="sm"
              variant="primary"
              icon={<Plus className="w-3.5 h-3.5" />}
              onClick={() => setCreateChapterModalOpen(true)}
            >
              新建第一章
            </Button>
          </div>
        ) : (
          chapters.map((chap, idx) => {
            const chapScenes = scenes.filter((s) => s.chapter_id === chap.id);
            const totalWords = chapScenes.reduce((acc, s) => acc + (s.word_count || 0), 0);
            return (
              <div
                key={chap.id}
                className="w-80 shrink-0 bg-white dark:bg-neutral-900 border border-neutral-200 dark:border-neutral-800 rounded-2xl shadow-xs flex flex-col max-h-full overflow-hidden"
              >
                {/* Chapter Card Header */}
                <div className="p-3.5 border-b border-neutral-100 dark:border-neutral-800 flex items-center justify-between bg-neutral-50/40 dark:bg-neutral-850">
                  <div className="flex items-center gap-1.5 font-semibold text-sm text-neutral-900 dark:text-neutral-100 truncate pr-2">
                    <span>
                      第 {idx + 1} 章 · {chap.title}
                    </span>
                  </div>
                  <span className="text-[11px] font-mono text-neutral-400 shrink-0">
                    {totalWords.toLocaleString()} 字
                  </span>
                </div>

                {/* Scene Cards container */}
                <div className="flex-1 overflow-y-auto p-3 space-y-3">
                  {chapScenes.length === 0 ? (
                    <div className="p-4 text-center text-xs text-neutral-400">
                      暂无细分场景
                    </div>
                  ) : (
                    chapScenes.map((scene) => (
                      <div
                        key={scene.id}
                        onClick={() => onOpenChapter && onOpenChapter(chap.id)}
                        className="p-3 rounded-xl border border-neutral-200/80 dark:border-neutral-800 bg-neutral-50/60 dark:bg-neutral-800/40 hover:border-amber-400 dark:hover:border-amber-600 transition-all cursor-pointer space-y-2 group"
                      >
                        <div className="flex items-center justify-between text-xs font-semibold text-neutral-800 dark:text-neutral-200">
                          <span className="group-hover:text-amber-600 transition-colors">
                            {scene.title}
                          </span>
                          <span className="font-mono text-neutral-400 text-[10px]">
                            {scene.word_count || 0} 字
                          </span>
                        </div>

                        <p className="text-xs text-neutral-500 line-clamp-3 leading-relaxed">
                          {scene.summary || "暂无场景描述"}
                        </p>

                        {scene.beats && (
                          <div className="pt-1 border-t border-neutral-200/40 dark:border-neutral-700/40 flex items-center gap-1 text-[10px] text-amber-700 dark:text-amber-400">
                            <Sparkles className="w-3 h-3 shrink-0" />
                            <span className="truncate">{scene.beats}</span>
                          </div>
                        )}
                      </div>
                    ))
                  )}

                  <button
                    onClick={() => {
                      setActiveChapterForScene(chap.id);
                      setCreateSceneModalOpen(true);
                    }}
                    className="w-full py-2 border border-dashed border-neutral-200 dark:border-neutral-800 rounded-xl text-xs text-neutral-400 hover:text-amber-600 hover:border-amber-300 dark:hover:border-amber-700 transition-colors flex items-center justify-center gap-1"
                  >
                    <Plus className="w-3.5 h-3.5" />
                    <span>添加场景</span>
                  </button>
                </div>
              </div>
            );
          })
        )}

        {/* Add Chapter Card placeholder */}
        {chapters.length > 0 && (
          <button
            onClick={() => setCreateChapterModalOpen(true)}
            className="w-80 shrink-0 h-40 border-2 border-dashed border-neutral-200 dark:border-neutral-800 rounded-2xl flex flex-col items-center justify-center text-neutral-400 hover:text-amber-600 hover:border-amber-400 transition-all gap-2"
          >
            <Plus className="w-6 h-6" />
            <span className="text-sm font-medium">添加新章节卡片</span>
          </button>
        )}
      </div>

      {/* New Chapter Modal */}
      <Modal
        open={createChapterModalOpen}
        onClose={() => setCreateChapterModalOpen(false)}
        title="添加章节规划"
        footer={
          <>
            <Button variant="ghost" onClick={() => setCreateChapterModalOpen(false)}>
              取消
            </Button>
            <Button variant="primary" loading={creatingChapter} onClick={handleCreateChapter}>
              确认添加
            </Button>
          </>
        }
      >
        <div className="space-y-3">
          <label className="block text-xs font-medium text-neutral-700 dark:text-neutral-300">
            章节标题
          </label>
          <Input
            value={newChapterTitle}
            onChange={(e) => setNewChapterTitle(e.target.value)}
            placeholder="例如：万宝楼竞拍玄灵草..."
          />
        </div>
      </Modal>

      {/* New Scene Modal */}
      <Modal
        open={createSceneModalOpen}
        onClose={() => setCreateSceneModalOpen(false)}
        title="添加细分场景"
        footer={
          <>
            <Button variant="ghost" onClick={() => setCreateSceneModalOpen(false)}>
              取消
            </Button>
            <Button variant="primary" loading={creatingScene} onClick={handleCreateScene}>
              确认添加
            </Button>
          </>
        }
      >
        <div className="space-y-3">
          <div>
            <label className="block text-xs font-medium text-neutral-700 dark:text-neutral-300 mb-1">
              场景标题
            </label>
            <Input
              value={newSceneTitle}
              onChange={(e) => setNewSceneTitle(e.target.value)}
              placeholder="例如：绝壁遇伏..."
            />
          </div>
          <div>
            <label className="block text-xs font-medium text-neutral-700 dark:text-neutral-300 mb-1">
              核心剧情走向 / 事件概括
            </label>
            <Textarea
              value={newSceneSummary}
              onChange={(e) => setNewSceneSummary(e.target.value)}
              rows={4}
              placeholder="本场景发生什么事件？有哪些人物登场？"
            />
          </div>
        </div>
      </Modal>
    </div>
  );
}

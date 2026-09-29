import React, { useState, useEffect, useRef } from "react";
import cytoscape from "cytoscape";
import { User, Globe, Shield, Zap, Sparkles, RefreshCw, Layers, Plus } from "lucide-react";
import { Button, Card, Badge, Input, Textarea, Modal, ErrorPresenter } from "@inklore/ui";
import { getApiSDK, reportSafeDiagnostic, safeErrorText } from "@inklore/transport";
import { useTranslation } from "@inklore/i18n";
import { Character, LoreEntry, WorldviewGraph } from "@inklore/contracts";

export interface WorldviewRouteProps {
  novelId?: string;
}

export function WorldviewRoute({ novelId: initialNovelId }: WorldviewRouteProps) {
  const { t } = useTranslation();
  const api = getApiSDK();

  const [currentNovelId, setCurrentNovelId] = useState<string | null>(initialNovelId || null);
  const [subView, setSubView] = useState<"roles" | "settings" | "graph">("graph");
  const cyRef = useRef<HTMLDivElement>(null);
  const cyInstance = useRef<cytoscape.Core | null>(null);

  const [characters, setCharacters] = useState<Character[]>([]);
  const [lore, setLore] = useState<LoreEntry[]>([]);
  const [graphData, setGraphData] = useState<WorldviewGraph | null>(null);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState<unknown>(null);

  // 添加角色弹窗
  const [createCharModalOpen, setCreateCharModalOpen] = useState(false);
  const [charName, setCharName] = useState("");
  const [charRoleType, setCharRoleType] = useState<"protagonist" | "antagonist" | "supporting" | "cameo">("supporting");
  const [charSummary, setCharSummary] = useState("");
  const [creatingChar, setCreatingChar] = useState(false);

  // 1. 初始化 novelId
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
        reportSafeDiagnostic("creator.worldview_fetch_novels_failed", err);
      });
    return () => {
      active = false;
    };
  }, [initialNovelId]);

  // 2. 加载真实世界观数据
  const loadWorldviewData = async () => {
    if (!currentNovelId) return;
    setLoading(true);
    setLoadError(null);
    try {
      const [charsRes, loreRes, graphRes] = await Promise.allSettled([
        api.worldview.listCharacters(currentNovelId),
        api.worldview.listLore(currentNovelId),
        api.worldview.getGraph(currentNovelId),
      ]);

      if (charsRes.status === "fulfilled") setCharacters(charsRes.value);
      if (loreRes.status === "fulfilled") setLore(loreRes.value);
      if (graphRes.status === "fulfilled") setGraphData(graphRes.value);

      const firstFail = [charsRes, loreRes, graphRes].find(
        (r): r is PromiseRejectedResult => r.status === "rejected"
      );
      if (firstFail && charsRes.status === "rejected" && loreRes.status === "rejected") {
        setLoadError(firstFail.reason);
      }
    } catch (err) {
      reportSafeDiagnostic("creator.worldview_load_failed", err);
      setLoadError(err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (currentNovelId) {
      void loadWorldviewData();
    }
  }, [currentNovelId]);

  // 3. 基于真实数据初始化 Cytoscape 图谱
  useEffect(() => {
    if (subView !== "graph" || !cyRef.current) return;

    // 优先采用后端返回的 graphData，若无则由真实角色与设定条目映射节点
    const elementsNodes: cytoscape.NodeDefinition[] = [];
    const elementsEdges: cytoscape.EdgeDefinition[] = [];

    if (graphData && graphData.nodes && graphData.nodes.length > 0) {
      graphData.nodes.forEach((n) => {
        elementsNodes.push({
          data: {
            id: n.id,
            label: n.label,
            type: n.type,
            color: n.color || "#2563eb",
          },
        });
      });
      if (graphData.edges) {
        graphData.edges.forEach((e) => {
          elementsEdges.push({
            data: {
              source: e.source,
              target: e.target,
              label: e.label,
            },
          });
        });
      }
    } else {
      // 从真实角色构建节点
      characters.forEach((c) => {
        elementsNodes.push({
          data: {
            id: c.id,
            label: `${c.name} (${c.role_type === "protagonist" ? "主角" : "配角"})`,
            type: "character",
            color: c.role_type === "protagonist" ? "#2563eb" : "#f59e0b",
          },
        });
      });
      // 从真实设定词条构建节点
      lore.forEach((l) => {
        elementsNodes.push({
          data: {
            id: l.id,
            label: l.title,
            type: l.category,
            color: "#10b981",
          },
        });
      });
    }

    if (elementsNodes.length === 0) {
      if (cyInstance.current) {
        cyInstance.current.destroy();
        cyInstance.current = null;
      }
      return;
    }

    cyInstance.current = cytoscape({
      container: cyRef.current,
      elements: { nodes: elementsNodes, edges: elementsEdges },
      style: [
        {
          selector: "node",
          style: {
            "background-color": "data(color)",
            label: "data(label)",
            color: "#ffffff",
            "font-size": "11px",
            "text-valign": "center",
            "text-halign": "center",
            "text-outline-color": "#1e293b",
            "text-outline-width": 2,
            width: 44,
            height: 44,
          },
        },
        {
          selector: "edge",
          style: {
            width: 1.5,
            "line-color": "#94a3b8",
            "target-arrow-color": "#94a3b8",
            "target-arrow-shape": "triangle",
            "curve-style": "bezier",
            label: "data(label)",
            "font-size": "9px",
            color: "#64748b",
            "text-background-opacity": 0.8,
            "text-background-color": "#ffffff",
            "text-background-padding": "2px",
          },
        },
      ],
      layout: {
        name: "cose",
        padding: 30,
        animate: false,
      },
    });

    return () => {
      cyInstance.current?.destroy();
      cyInstance.current = null;
    };
  }, [subView, graphData, characters, lore]);

  // 创建真实角色
  const handleCreateCharacter = async () => {
    if (!currentNovelId || !charName.trim()) return;
    setCreatingChar(true);
    try {
      const created = await api.worldview.createCharacter(currentNovelId, {
        name: charName.trim(),
        role_type: charRoleType,
        summary: charSummary.trim() || undefined,
      });
      setCharacters([...characters, created]);
      setCharName("");
      setCharSummary("");
      setCreateCharModalOpen(false);
      void loadWorldviewData();
    } catch (err) {
      reportSafeDiagnostic("creator.create_character_failed", err);
      alert(safeErrorText(err, t));
    } finally {
      setCreatingChar(false);
    }
  };

  const hasGraphNodes = (graphData?.nodes?.length ?? 0) > 0 || characters.length > 0 || lore.length > 0;

  return (
    <div className="flex-1 flex flex-col overflow-hidden bg-neutral-50/50 dark:bg-neutral-950 p-6">
      {/* Top Header */}
      <div className="flex items-center justify-between pb-4 border-b border-neutral-200 dark:border-neutral-800 shrink-0">
        <div>
          <h2 className="text-xl font-bold text-neutral-900 dark:text-neutral-100">
            世界观设定与实体图谱
          </h2>
          <p className="text-xs text-neutral-500 mt-0.5">
            实体关系图谱 · 角色档案 · 世界观词条 · 事实溯源账本
          </p>
        </div>

        {/* View Switcher Tabs */}
        <div className="flex items-center bg-neutral-100 dark:bg-neutral-800 p-0.5 rounded-lg text-xs font-medium">
          <button
            onClick={() => setSubView("graph")}
            className={`px-3 py-1.5 rounded-md transition-all ${
              subView === "graph"
                ? "bg-white dark:bg-neutral-700 text-neutral-900 dark:text-neutral-100 shadow-xs"
                : "text-neutral-500 hover:text-neutral-800"
            }`}
          >
            实体图谱
          </button>
          <button
            onClick={() => setSubView("roles")}
            className={`px-3 py-1.5 rounded-md transition-all ${
              subView === "roles"
                ? "bg-white dark:bg-neutral-700 text-neutral-900 dark:text-neutral-100 shadow-xs"
                : "text-neutral-500 hover:text-neutral-800"
            }`}
          >
            角色档案 ({characters.length})
          </button>
          <button
            onClick={() => setSubView("settings")}
            className={`px-3 py-1.5 rounded-md transition-all ${
              subView === "settings"
                ? "bg-white dark:bg-neutral-700 text-neutral-900 dark:text-neutral-100 shadow-xs"
                : "text-neutral-500 hover:text-neutral-800"
            }`}
          >
            世界观条目 ({lore.length})
          </button>
        </div>
      </div>

      {loadError != null && (
        <div className="pt-3">
          <ErrorPresenter error={loadError} onRetry={() => void loadWorldviewData()} />
        </div>
      )}

      {/* Main Content Area */}
      <div className="flex-1 overflow-hidden pt-4 flex flex-col">
        {subView === "graph" && (
          <div className="flex-1 bg-white dark:bg-neutral-900 rounded-2xl border border-neutral-200 dark:border-neutral-800 overflow-hidden relative flex flex-col shadow-xs">
            {/* Graph Toolbar */}
            <div className="p-3 border-b border-neutral-100 dark:border-neutral-800 flex items-center justify-between z-10 bg-white/80 dark:bg-neutral-900/80 backdrop-blur-xs">
              <div className="flex items-center gap-2">
                <span className="text-xs font-semibold text-neutral-700 dark:text-neutral-300">
                  力导向关系网络
                </span>
                <Badge variant="outline" size="sm">
                  {characters.length} 角色 · {lore.length} 设定条目
                </Badge>
              </div>

              <div className="flex items-center gap-2">
                <Button
                  size="sm"
                  variant="outline"
                  icon={<RefreshCw className="w-3.5 h-3.5" />}
                  onClick={() => cyInstance.current?.layout({ name: "cose", animate: true }).run()}
                  disabled={!hasGraphNodes}
                >
                  重新排布
                </Button>
                <Button
                  size="sm"
                  variant="primary"
                  icon={<Plus className="w-3.5 h-3.5" />}
                  onClick={() => setCreateCharModalOpen(true)}
                >
                  添加角色
                </Button>
              </div>
            </div>

            {/* Cytoscape Canvas 或 空态 */}
            {!hasGraphNodes && !loading ? (
              <div className="flex-1 flex flex-col items-center justify-center p-12 text-center text-neutral-400">
                <Layers className="w-10 h-10 text-neutral-300 dark:text-neutral-700 mb-3" />
                <h4 className="text-sm font-semibold text-neutral-700 dark:text-neutral-300 mb-1">
                  当前作品暂无角色与世界观图谱
                </h4>
                <p className="text-xs text-neutral-400 mb-4">
                  添加首批登场角色或世界观条目后，系统将自动演化力导向拓扑关系网
                </p>
                <Button
                  size="sm"
                  variant="primary"
                  icon={<Plus className="w-3.5 h-3.5" />}
                  onClick={() => setCreateCharModalOpen(true)}
                >
                  立即创建角色
                </Button>
              </div>
            ) : (
              <div ref={cyRef} className="flex-1 w-full h-full cursor-grab active:cursor-grabbing" />
            )}
          </div>
        )}

        {subView === "roles" && (
          <div className="flex-1 overflow-y-auto space-y-4">
            <div className="flex justify-end">
              <Button
                size="sm"
                variant="primary"
                icon={<Plus className="w-3.5 h-3.5" />}
                onClick={() => setCreateCharModalOpen(true)}
              >
                新建角色档案
              </Button>
            </div>

            {characters.length === 0 && !loading ? (
              <div className="p-12 text-center text-xs text-neutral-400">
                暂无角色档案记录
              </div>
            ) : (
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                {characters.map((c) => (
                  <Card key={c.id} className="p-4 flex items-start gap-3.5 hover:border-amber-400 transition-colors">
                    <div className="w-10 h-10 rounded-full bg-amber-600/10 text-amber-700 dark:text-amber-400 flex items-center justify-center shrink-0 font-bold text-sm">
                      {c.name.slice(0, 1)}
                    </div>
                    <div className="flex-1 space-y-1.5">
                      <div className="flex items-center gap-2">
                        <span className="font-semibold text-sm text-neutral-900 dark:text-neutral-100">
                          {c.name}
                        </span>
                        <Badge
                          variant={c.role_type === "protagonist" ? "success" : "secondary"}
                          size="sm"
                        >
                          {c.role_type === "protagonist" ? "主角" : c.role_type === "antagonist" ? "反派" : "配角"}
                        </Badge>
                        <span className="text-[10px] text-neutral-400 font-mono">rev.{c.revision}</span>
                      </div>
                      <p className="text-xs text-neutral-500 leading-relaxed">
                        {c.summary || "暂无人物小传描述"}
                      </p>
                    </div>
                  </Card>
                ))}
              </div>
            )}
          </div>
        )}

        {subView === "settings" && (
          <div className="flex-1 overflow-y-auto space-y-4">
            {lore.length === 0 && !loading ? (
              <div className="p-12 text-center text-xs text-neutral-400">
                暂无世界观设定词条
              </div>
            ) : (
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                {lore.map((l) => (
                  <Card key={l.id} className="p-4 space-y-2 hover:border-amber-400 transition-colors">
                    <div className="flex items-center gap-2 font-semibold text-sm text-neutral-900 dark:text-neutral-100">
                      <Globe className="w-4 h-4 text-emerald-500" />
                      <span>{l.title}</span>
                      <Badge variant="outline" size="sm">{l.category}</Badge>
                    </div>
                    <p className="text-xs text-neutral-500 leading-relaxed">
                      {l.content}
                    </p>
                  </Card>
                ))}
              </div>
            )}
          </div>
        )}
      </div>

      {/* 创建角色模态框 */}
      <Modal
        open={createCharModalOpen}
        onClose={() => setCreateCharModalOpen(false)}
        title="创建新角色档案"
        footer={
          <>
            <Button variant="ghost" onClick={() => setCreateCharModalOpen(false)}>
              取消
            </Button>
            <Button variant="primary" loading={creatingChar} onClick={handleCreateCharacter}>
              确认创建
            </Button>
          </>
        }
      >
        <div className="space-y-3">
          <div>
            <label className="block text-xs font-medium text-neutral-700 dark:text-neutral-300 mb-1">
              角色姓名
            </label>
            <Input
              value={charName}
              onChange={(e) => setCharName(e.target.value)}
              placeholder="例如：林惊羽"
            />
          </div>
          <div>
            <label className="block text-xs font-medium text-neutral-700 dark:text-neutral-300 mb-1">
              角色定位
            </label>
            <select
              value={charRoleType}
              onChange={(e) => setCharRoleType(e.target.value as any)}
              className="w-full text-xs p-2 rounded-lg border border-neutral-300 dark:border-neutral-700 bg-white dark:bg-neutral-800 text-neutral-800 dark:text-neutral-200"
            >
              <option value="protagonist">主角</option>
              <option value="antagonist">反派</option>
              <option value="supporting">配角</option>
              <option value="cameo">客串/龙套</option>
            </select>
          </div>
          <div>
            <label className="block text-xs font-medium text-neutral-700 dark:text-neutral-300 mb-1">
              人设小传与动机
            </label>
            <Textarea
              value={charSummary}
              onChange={(e) => setCharSummary(e.target.value)}
              rows={3}
              placeholder="简述该角色的背景身份、核心执念或主要性格标签..."
            />
          </div>
        </div>
      </Modal>
    </div>
  );
}

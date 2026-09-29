import React, { useState, useEffect } from "react";
import { Activity, BarChart2, TrendingUp, Sparkles, Layers } from "lucide-react";
import { Card, Badge, Button, ErrorPresenter } from "@inklore/ui";
import { getApiSDK, reportSafeDiagnostic } from "@inklore/transport";
import { useTranslation } from "@inklore/i18n";
import { PacingAnalysis, PacingPoint } from "@inklore/contracts";

export interface AnalysisRouteProps {
  novelId?: string;
}

export function AnalysisRoute({ novelId: initialNovelId }: AnalysisRouteProps) {
  const { t } = useTranslation();
  const api = getApiSDK();

  const [currentNovelId, setCurrentNovelId] = useState<string | null>(initialNovelId || null);
  const [activeMetric, setActiveMetric] = useState<"overall" | "emotion" | "density" | "tension" | "filler">("overall");
  const [analysis, setAnalysis] = useState<PacingAnalysis | null>(null);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState<unknown>(null);

  const metrics = [
    { id: "overall", label: "综合评分", color: "#3b82f6" },
    { id: "emotion", label: "情绪强度", color: "#ef4444" },
    { id: "density", label: "信息密度", color: "#06b6d4" },
    { id: "tension", label: "断章张力", color: "#f59e0b" },
    { id: "filler", label: "注水风险", color: "#64748b" },
  ];

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
        reportSafeDiagnostic("creator.analysis_fetch_novels_failed", err);
      });
    return () => {
      active = false;
    };
  }, [initialNovelId]);

  const loadAnalysisData = async () => {
    if (!currentNovelId) return;
    setLoading(true);
    setLoadError(null);
    try {
      const data = await api.outline.getPacing(currentNovelId);
      setAnalysis(data);
    } catch (err) {
      reportSafeDiagnostic("creator.pacing_load_failed", err);
      setLoadError(err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (currentNovelId) {
      void loadAnalysisData();
    }
  }, [currentNovelId]);

  const points: PacingPoint[] = analysis?.points || [];

  return (
    <div className="flex-1 flex flex-col overflow-y-auto bg-neutral-50/50 dark:bg-neutral-950 p-6 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between pb-4 border-b border-neutral-200 dark:border-neutral-800">
        <div>
          <div className="flex items-center gap-2">
            <h2 className="text-xl font-bold text-neutral-900 dark:text-neutral-100">
              章节节奏分析与张力诊断
            </h2>
            {analysis && (
              <Badge variant="success" size="sm">
                全书健康度 {analysis.overall_score || 0}分
              </Badge>
            )}
          </div>
          <p className="text-xs text-neutral-500 mt-0.5">
            多维度节奏曲线 · 实时对比高分网文节奏基准与爽点波峰
          </p>
        </div>

        <div className="flex items-center gap-2">
          <Button
            size="sm"
            variant="outline"
            icon={<Sparkles className="w-3.5 h-3.5 text-amber-500" />}
            onClick={() => alert("正在生成诊断导出报告...")}
          >
            导出诊断报告
          </Button>
        </div>
      </div>

      {loadError != null && (
        <ErrorPresenter error={loadError} onRetry={() => void loadAnalysisData()} />
      )}

      {/* Metric selection pills */}
      <div className="flex flex-wrap gap-2">
        {metrics.map((m) => (
          <button
            key={m.id}
            onClick={() => setActiveMetric(m.id as any)}
            className={`px-3 py-1.5 rounded-full text-xs font-medium transition-all border ${
              activeMetric === m.id
                ? "bg-amber-600 text-white border-amber-600 shadow-xs"
                : "bg-white dark:bg-neutral-900 text-neutral-700 dark:text-neutral-300 border-neutral-200 dark:border-neutral-800 hover:border-neutral-300"
            }`}
          >
            {m.label}
          </button>
        ))}
      </div>

      {/* Pacing Curve Chart Card (数据驱动) */}
      <Card className="p-6 space-y-4">
        <div className="flex items-center justify-between">
          <div className="font-semibold text-sm text-neutral-800 dark:text-neutral-200 flex items-center gap-2">
            <Activity className="w-4 h-4 text-amber-500" />
            <span>章节多维度张弛度曲线 ({points.length > 0 ? `共 ${points.length} 章节数据点` : "等待数据采样"})</span>
          </div>
          <span className="text-xs text-neutral-400">纵轴：张力指数 (0-10) · 横轴：章节序列</span>
        </div>

        {points.length === 0 && !loading ? (
          <div className="w-full h-48 flex flex-col items-center justify-center p-8 text-center text-neutral-400 border border-dashed border-neutral-200 dark:border-neutral-800 rounded-xl">
            <Layers className="w-8 h-8 text-neutral-300 dark:text-neutral-700 mb-2" />
            <p className="text-sm font-medium text-neutral-600 dark:text-neutral-400 mb-1">
              暂无节奏采样点
            </p>
            <p className="text-xs text-neutral-400">
              在作品中完成章节正文创作并保存后，合规质检与大纲引擎将自动演化节奏诊断曲线
            </p>
          </div>
        ) : (
          <div className="w-full h-64 bg-neutral-50/50 dark:bg-neutral-950/50 rounded-xl border border-neutral-100 dark:border-neutral-800/80 p-4 relative flex flex-col justify-between">
            <svg className="w-full h-full overflow-visible" viewBox={`0 0 ${Math.max(400, points.length * 40)} 200`} preserveAspectRatio="none">
              <line x1="0" y1="50" x2="100%" y2="50" stroke="#e2e8f0" strokeDasharray="3 3" className="dark:stroke-neutral-800" />
              <line x1="0" y1="100" x2="100%" y2="100" stroke="#e2e8f0" strokeDasharray="3 3" className="dark:stroke-neutral-800" />
              <line x1="0" y1="150" x2="100%" y2="150" stroke="#e2e8f0" strokeDasharray="3 3" className="dark:stroke-neutral-800" />

              {/* 绘制真实采样曲线 */}
              {points.length > 1 && (
                <polyline
                  fill="none"
                  stroke="#f59e0b"
                  strokeWidth="2.5"
                  points={points
                    .map((p, idx) => {
                      const x = (idx / (points.length - 1)) * Math.max(400, points.length * 40);
                      const y = 200 - (p.tension / 10) * 180;
                      return `${x},${y}`;
                    })
                    .join(" ")}
                />
              )}
            </svg>

            <div className="flex justify-between text-[10px] font-mono text-neutral-400 mt-2">
              <span>第 1 章</span>
              {points.length > 1 && <span>第 {points.length} 章</span>}
            </div>
          </div>
        )}
      </Card>
    </div>
  );
}

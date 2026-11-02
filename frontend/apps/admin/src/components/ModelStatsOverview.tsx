import { Cpu, Eye, Layers, Sparkles } from "lucide-react";
import type { ReactNode } from "react";

type ModelStats = {
  total: number;
  llmCount: number;
  visionCount: number;
  unboundCount: number;
};

export function ModelStatsOverview({ stats }: { stats: ModelStats }) {
  return (
    <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 px-6 py-3 bg-[var(--bg-canvas)]/60 border-b border-[var(--border-decorative)] shrink-0">
      <StatCard icon={<Cpu className="w-4 h-4" />} label="在册模型" value={stats.total} tone="primary" />
      <StatCard icon={<Sparkles className="w-4 h-4" />} label="文本/对话模型" value={stats.llmCount} tone="blue" />
      <StatCard icon={<Eye className="w-4 h-4" />} label="支持视觉识别" value={stats.visionCount} tone="emerald" />
      <StatCard icon={<Layers className="w-4 h-4" />} label="未接入渠道" value={stats.unboundCount} tone="amber" />
    </div>
  );
}

function StatCard({ icon, label, value, tone }: { icon: ReactNode; label: string; value: number; tone: "primary" | "blue" | "emerald" | "amber" }) {
  const toneClass = {
    primary: "bg-[var(--action-primary)]/10 text-[var(--action-primary)]",
    blue: "bg-blue-500/10 text-blue-600 dark:text-blue-400",
    emerald: "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400",
    amber: "bg-amber-500/10 text-amber-600 dark:text-amber-400",
  }[tone];
  return (
    <div className="flex items-center gap-3 p-2.5 rounded-xl bg-[var(--bg-surface)] border border-[var(--border-decorative)]/70">
      <div className={`w-8 h-8 rounded-lg ${toneClass} flex items-center justify-center shrink-0`}>{icon}</div>
      <div>
        <span className="text-[11px] text-[var(--text-muted)] block">{label}</span>
        <span className="font-semibold text-sm text-[var(--text-primary)] font-mono">{value}</span>
      </div>
    </div>
  );
}

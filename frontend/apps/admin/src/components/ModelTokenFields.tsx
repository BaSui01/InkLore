import { Input } from "@inklore/ui";
import { useTranslation } from "@inklore/i18n";

interface ModelTokenFieldsProps {
  formModelContextWindow: string;
  formModelMaxOutput: string;
  setFormModelContextWindow: (value: string) => void;
  setFormModelMaxOutput: (value: string) => void;
}

function formatTokensLabel(raw: string | number): string {
  const num = typeof raw === "string" ? parseInt(raw, 10) : raw;
  if (!num || isNaN(num)) return "";
  const formatted = num.toLocaleString();
  let approx = "";
  if (num >= 1_000_000) {
    approx = ` (约 ${(num / 1_000_000).toFixed(num % 1_000_000 === 0 ? 0 : 1)}M)`;
  } else if (num >= 1_000) {
    approx = ` (约 ${(num / 1_000).toFixed(num % 1_000 === 0 ? 0 : 1)}K)`;
  }
  return `${formatted} Tokens${approx}`;
}

export function ModelTokenFields({
  formModelContextWindow,
  formModelMaxOutput,
  setFormModelContextWindow,
  setFormModelMaxOutput,
}: ModelTokenFieldsProps) {
  const { t } = useTranslation();
  return (
    <div className="grid grid-cols-2 gap-3">
      <div className="space-y-1">
        <div className="flex items-center justify-between">
          <label className="font-semibold text-[var(--text-secondary)]">
            {t("admin.models.modal_context_window")}
          </label>
          {formModelContextWindow && (
            <span className="text-[10px] font-mono text-[var(--action-primary)] font-medium">
              {formatTokensLabel(formModelContextWindow)}
            </span>
          )}
        </div>
        <Input
          type="number"
          value={formModelContextWindow}
          onChange={(e) => setFormModelContextWindow(e.target.value)}
        />
        <div className="flex items-center gap-1.5 pt-0.5">
          {["128000", "200000", "1000000", "2000000"].map((val) => (
            <button
              key={val}
              type="button"
              onClick={() => setFormModelContextWindow(val)}
              className="text-[10px] font-mono px-1.5 py-0.5 rounded bg-[var(--bg-canvas)] hover:bg-[var(--bg-subtle)] border border-[var(--border-decorative)] text-[var(--text-muted)] hover:text-[var(--text-primary)] transition-colors cursor-pointer"
            >
              {val === "1000000" ? "1M" : val === "2000000" ? "2M" : `${parseInt(val, 10) / 1000}K`}
            </button>
          ))}
        </div>
      </div>
      <div className="space-y-1">
        <div className="flex items-center justify-between">
          <label className="font-semibold text-[var(--text-secondary)]">
            {t("admin.models.modal_max_output")}
            <span className="text-[10px] font-normal text-[var(--text-muted)] ml-1">(上限 16K)</span>
          </label>
          {formModelMaxOutput && (
            <span
              className={`text-[10px] font-mono font-medium ${
                parseInt(formModelMaxOutput, 10) > 16384
                  ? "text-[var(--status-danger)]"
                  : "text-[var(--action-primary)]"
              }`}
            >
              {parseInt(formModelMaxOutput, 10) > 16384
                ? "超过上限 16,384"
                : formatTokensLabel(formModelMaxOutput)}
            </span>
          )}
        </div>
        <Input
          type="number"
          value={formModelMaxOutput}
          onChange={(e) => setFormModelMaxOutput(e.target.value)}
        />
        <div className="flex items-center gap-1.5 pt-0.5">
          {["2048", "4096", "8192", "16384"].map((val) => (
            <button
              key={val}
              type="button"
              onClick={() => setFormModelMaxOutput(val)}
              className="text-[10px] font-mono px-1.5 py-0.5 rounded bg-[var(--bg-canvas)] hover:bg-[var(--bg-subtle)] border border-[var(--border-decorative)] text-[var(--text-muted)] hover:text-[var(--text-primary)] transition-colors cursor-pointer"
            >
              {val === "16384" ? "16K (最大)" : `${parseInt(val, 10) / 1024}K`}
            </button>
          ))}
        </div>
      </div>
    </div>
  );
}

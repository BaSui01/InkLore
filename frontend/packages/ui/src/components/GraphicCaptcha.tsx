import { useRef, useEffect, useState, useCallback } from "react";
import { RotateCw } from "lucide-react";

export interface GraphicCaptchaProps {
  onCaptchaChange?: (captcha: { id: string }) => void;
  width?: number;
  height?: number;
  className?: string;
  refreshTitle?: string;
  unavailableTitle?: string;
}

export function GraphicCaptcha({
  onCaptchaChange,
  width = 108,
  height = 36,
  className = "",
  refreshTitle = "点击更换验证码",
  unavailableTitle = "验证码加载失败",
}: GraphicCaptchaProps) {
  const [serverSvg, setServerSvg] = useState<string | null>(null);
  const [isSpinning, setIsSpinning] = useState(false);
  const onCaptchaChangeRef = useRef(onCaptchaChange);

  useEffect(() => {
    onCaptchaChangeRef.current = onCaptchaChange;
  });

  // 对 SVG 实施主题适配（防御性保证：即使用户的后端服务未重启仍返回旧版深色 SVG，前端也能动态清洗重绘成与浅色底纹完全融合的样式）
  const adaptSvg = useCallback((rawSvg: string): string => {
    let svg = rawSvg;
    svg = svg.replace(/<svg\s+([^>]*?)>/i, (_, attrs) => {
      let cleaned = attrs
        .replace(/width="[^"]*"/g, 'width="100%"')
        .replace(/height="[^"]*"/g, 'height="100%"')
        .replace(/style="[^"]*"/g, 'style="background:transparent;border-radius:8px;user-select:none;display:block;"');
      return `<svg ${cleaned}>`;
    });

    const palette: Record<string, string> = {
      "#eeeae1": "var(--bg-subtle, #edf0ec)",
      "#273449": "var(--bg-subtle, #303a33)",
      "#6b7280": "var(--border-control, #98a69b)",
      "#94a3b8": "var(--border-control, #718076)",
      "#526071": "var(--text-muted, #6b766e)",
      "#b5c0d0": "var(--text-muted, #a4b0a6)",
      "#1d4ed8": "var(--auth-captcha-a, #386252)",
      "#93c5fd": "var(--auth-captcha-a, #a8c6ad)",
      "#4338ca": "var(--auth-captcha-b, #687354)",
      "#a5b4fc": "var(--auth-captcha-b, #c1c997)",
      "#15803d": "var(--auth-captcha-c, #9a6544)",
      "#86efac": "var(--auth-captcha-c, #d5a57e)",
      "#b45309": "var(--auth-captcha-d, #914d43)",
      "#fde68a": "var(--auth-captcha-d, #d49288)",
      "#b91c1c": "var(--auth-captcha-e, #3f5549)",
      "#fca5a5": "var(--auth-captcha-e, #a9c0b0)",
      "#6d28d9": "var(--auth-captcha-b, #c1c997)",
      "#c4b5fd": "var(--auth-captcha-b, #c1c997)",
    };
    for (const [color, value] of Object.entries(palette)) {
      svg = svg.replace(new RegExp(color, "gi"), value);
    }
    return svg;
  }, []);

  // 图形验证码只允许使用服务端签发的一次性挑战。
  const refresh = useCallback(async () => {
    onCaptchaChangeRef.current?.({ id: "" });
    setServerSvg(null);
    setIsSpinning(true);
    try {
      const res = await fetch("/v1/auth/captcha", {
        method: "GET",
        headers: { Accept: "application/json" },
        cache: "no-store",
      });
      if (!res.ok) throw new Error("captcha unavailable");
      const body = await res.json();
      if (typeof body?.data?.svg !== "string" || typeof body?.data?.captcha_id !== "string") {
        throw new Error("invalid captcha response");
      }
      setServerSvg(adaptSvg(body.data.svg));
      onCaptchaChangeRef.current?.({ id: body.data.captcha_id });
    } catch {
      // API 不可用时禁止本地伪造挑战与绕过校验。
      setServerSvg(null);
    } finally {
      setIsSpinning(false);
    }
  }, [adaptSvg]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  return (
    <button
      type="button"
      onClick={() => { void refresh(); }}
      title={refreshTitle}
      className={`flex items-center justify-between p-1 rounded-xl bg-[var(--bg-subtle)] border border-[var(--border-decorative)] hover:border-[var(--border-control)] transition-all cursor-pointer shadow-xs shrink-0 select-none group overflow-hidden ${className}`}
    >
      {serverSvg ? (
        <div
          className="rounded-lg overflow-hidden pointer-events-none flex items-center justify-center w-full h-full"
          style={{ width, height }}
          dangerouslySetInnerHTML={{ __html: serverSvg }}
        />
      ) : (
        <div
          role="status"
          className="rounded-lg text-center text-[10px] leading-tight text-[var(--text-muted)] flex items-center justify-center"
          style={{ width, height }}
        >
          {isSpinning ? refreshTitle : unavailableTitle}
        </div>
      )}
      <div className="px-1.5 text-[var(--text-muted)] group-hover:text-[var(--text-primary)] transition-colors flex items-center justify-center">
        <RotateCw
          className={`w-3.5 h-3.5 transition-transform duration-300 ${isSpinning ? "rotate-180" : ""}`}
        />
      </div>
    </button>
  );
}

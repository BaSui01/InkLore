import React, { useRef, useEffect, useState, useCallback } from "react";
import { RotateCw } from "lucide-react";

export interface GraphicCaptchaProps {
  onCodeChange?: (code: string) => void;
  onCaptchaChange?: (captcha: { id: string; code?: string }) => void;
  width?: number;
  height?: number;
  className?: string;
  refreshTitle?: string;
}

export function GraphicCaptcha({
  onCodeChange,
  onCaptchaChange,
  width = 108,
  height = 36,
  className = "",
  refreshTitle = "点击更换验证码",
}: GraphicCaptchaProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const [serverSvg, setServerSvg] = useState<string | null>(null);
  const [isServerCaptcha, setIsServerCaptcha] = useState(false);
  const [isSpinning, setIsSpinning] = useState(false);

  // 使用 Ref 引用外部回调，切断因父组件重渲染导致 refresh 依赖刷新而产生的死循环
  const onCodeChangeRef = useRef(onCodeChange);
  const onCaptchaChangeRef = useRef(onCaptchaChange);
  useEffect(() => {
    onCodeChangeRef.current = onCodeChange;
    onCaptchaChangeRef.current = onCaptchaChange;
  });

  // 客户端备用生成随机验证码字符（网络离线/纯前端预览时降级）
  const generateRandomCode = useCallback(() => {
    const chars = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ";
    let code = "";
    for (let i = 0; i < 4; i++) {
      code += chars.charAt(Math.floor(Math.random() * chars.length));
    }
    return code;
  }, []);

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

  // 绘制带有防刷噪点与干扰线的本地 Canvas 图形验证码（降级路径，动态跟随系统/应用主题）
  const drawFallbackCaptcha = useCallback(
    (code: string) => {
      const canvas = canvasRef.current;
      if (!canvas) return;
      const ctx = canvas.getContext("2d");
      if (!ctx) return;

      const isDark =
        typeof document !== "undefined" &&
        document.documentElement.classList.contains("dark");

      const theme = getComputedStyle(canvas.closest(".auth-shell") || document.documentElement);
      ctx.fillStyle = theme.getPropertyValue("--bg-subtle").trim() || (isDark ? "#303a33" : "#edf0ec");
      ctx.fillRect(0, 0, width, height);

      // 2. 绘制随机干扰线
      const lineStroke = isDark
        ? "rgba(169, 192, 176, 0.42)"
        : "rgba(83, 96, 87, 0.3)";
      for (let i = 0; i < 4; i++) {
        ctx.strokeStyle = lineStroke;
        ctx.lineWidth = Math.random() * 1.5 + 1;
        ctx.beginPath();
        ctx.moveTo(Math.random() * width, Math.random() * height);
        ctx.bezierCurveTo(
          Math.random() * width,
          Math.random() * height,
          Math.random() * width,
          Math.random() * height,
          Math.random() * width,
          Math.random() * height
        );
        ctx.stroke();
      }

      // 3. 绘制随机噪点
      const dotColor = isDark
        ? "rgba(255, 255, 255, 0.2)"
        : "rgba(32, 41, 35, 0.16)";
      for (let i = 0; i < 28; i++) {
        ctx.fillStyle = dotColor;
        ctx.beginPath();
        ctx.arc(
          Math.random() * width,
          Math.random() * height,
          Math.random() * 1.5,
          0,
          Math.PI * 2
        );
        ctx.fill();
      }

      // 4. 逐字绘制字符：高对比度色板
      const colors = [
        theme.getPropertyValue("--auth-captcha-a").trim(),
        theme.getPropertyValue("--auth-captcha-b").trim(),
        theme.getPropertyValue("--auth-captcha-c").trim(),
        theme.getPropertyValue("--auth-captcha-d").trim(),
        theme.getPropertyValue("--auth-captcha-e").trim(),
      ].filter(Boolean);
      ctx.textBaseline = "middle";

      for (let i = 0; i < code.length; i++) {
        const char = code[i];
        const fontSize = Math.floor(Math.random() * 4 + 21);
        ctx.font = `bold ${fontSize}px monospace`;
        ctx.fillStyle = colors[i % colors.length];

        ctx.save();
        const x = 14 + i * 22;
        const y = height / 2 + (Math.random() * 4 - 2);
        const angle = (Math.random() * 30 - 15) * (Math.PI / 180);

        ctx.translate(x, y);
        ctx.rotate(angle);
        ctx.fillText(char, 0, 0);
        ctx.restore();
      }
    },
    [width, height]
  );

  // 优先请求后端提供的安全图形验证码 (/v1/auth/captcha)
  const refresh = useCallback(async () => {
    setIsSpinning(true);
    try {
      const res = await fetch("/v1/auth/captcha", {
        method: "GET",
        headers: { Accept: "application/json" },
      });
      if (res.ok) {
        const body = await res.json();
        if (body?.data?.svg && body?.data?.captcha_id) {
          const adapted = adaptSvg(body.data.svg);
          setServerSvg(adapted);
          setIsServerCaptcha(true);
          onCaptchaChangeRef.current?.({ id: body.data.captcha_id });
          setTimeout(() => setIsSpinning(false), 300);
          return;
        }
      }
    } catch {
      // 后端服务不可达时平滑降级至本地生成
    }

    // 本地 Canvas 降级方案
    setIsServerCaptcha(false);
    setServerSvg(null);
    const newCode = generateRandomCode();
    drawFallbackCaptcha(newCode);
    onCodeChangeRef.current?.(newCode);
    onCaptchaChangeRef.current?.({ id: "", code: newCode });
    setTimeout(() => setIsSpinning(false), 300);
  }, [generateRandomCode, drawFallbackCaptcha, adaptSvg]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  return (
    <button
      type="button"
      onClick={refresh}
      title={refreshTitle}
      className={`flex items-center justify-between p-1 rounded-xl bg-[var(--bg-subtle)] border border-[var(--border-decorative)] hover:border-[var(--border-control)] transition-all cursor-pointer shadow-xs shrink-0 select-none group overflow-hidden ${className}`}
    >
      {isServerCaptcha && serverSvg ? (
        <div
          className="rounded-lg overflow-hidden pointer-events-none flex items-center justify-center w-full h-full"
          style={{ width, height }}
          dangerouslySetInnerHTML={{ __html: serverSvg }}
        />
      ) : (
        <canvas
          ref={canvasRef}
          width={width}
          height={height}
          className="rounded-lg pointer-events-none"
        />
      )}
      <div className="px-1.5 text-[var(--text-muted)] group-hover:text-[var(--text-primary)] transition-colors flex items-center justify-center">
        <RotateCw
          className={`w-3.5 h-3.5 transition-transform duration-300 ${
            isSpinning ? "rotate-180" : ""
          }`}
        />
      </div>
    </button>
  );
}

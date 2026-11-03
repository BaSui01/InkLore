/**
 * @inklore/ui - 设计系统语义 Token
 * Creator 与 Admin 共用的语义颜色及主题映射。
 */

import { Audience } from "@inklore/contracts";

export type ThemeMode = "light" | "dark";

export interface ColorTokens {
  bgCanvas: string;
  bgSurface: string;
  bgPaper: string;
  bgSubtle: string;
  textPrimary: string;
  textSecondary: string;
  textMuted: string;
  borderDecorative: string;
  borderControl: string;
  actionPrimary: string;
  actionOnPrimary: string;
  actionHover: string;
  focusRing: string;
  statusSuccess: string;
  statusWarning: string;
  statusDanger: string;
  statusInfo: string;
  diffAddBg: string;
  diffRemoveBg: string;
}

export const THEME_TOKENS: Record<Audience, Record<ThemeMode, ColorTokens>> = {
  creator: {
    light: {
      bgCanvas: "#F6F3ED",
      bgSurface: "#FFFFFF",
      bgPaper: "#FFFCF5",
      bgSubtle: "#EEEAE1",
      textPrimary: "#172033",
      textSecondary: "#4B5563",
      textMuted: "#526071",
      borderDecorative: "#D7D2C7",
      borderControl: "#6B7280",
      actionPrimary: "#1D4ED8",
      actionOnPrimary: "#FFFFFF",
      actionHover: "#1E40AF",
      focusRing: "#1D4ED8",
      statusSuccess: "#166534",
      statusWarning: "#92400E",
      statusDanger: "#B91C1C",
      statusInfo: "#1E40AF",
      diffAddBg: "#DCFCE7",
      diffRemoveBg: "#FEE2E2",
    },
    dark: {
      bgCanvas: "#171A20",
      bgSurface: "#1F2937",
      bgPaper: "#20242C",
      bgSubtle: "#2A303B",
      textPrimary: "#F3F4F6",
      textSecondary: "#D1D5DB",
      textMuted: "#B5C0D0",
      borderDecorative: "#465061",
      borderControl: "#94A3B8",
      actionPrimary: "#93C5FD",
      actionOnPrimary: "#172033",
      actionHover: "#BFDBFE",
      focusRing: "#93C5FD",
      statusSuccess: "#86EFAC",
      statusWarning: "#FDE68A",
      statusDanger: "#FCA5A5",
      statusInfo: "#BFDBFE",
      diffAddBg: "#163628",
      diffRemoveBg: "#45252D",
    },
  },
  admin: {
    light: {
      bgCanvas: "#F5F7FA",
      bgSurface: "#FFFFFF",
      bgPaper: "#FFFFFF",
      bgSubtle: "#EEF2F6",
      textPrimary: "#172033",
      textSecondary: "#4B5563",
      textMuted: "#526071",
      borderDecorative: "#D8DEE8",
      borderControl: "#6B7280",
      actionPrimary: "#1D4ED8",
      actionOnPrimary: "#FFFFFF",
      actionHover: "#1E40AF",
      focusRing: "#1D4ED8",
      statusSuccess: "#166534",
      statusWarning: "#92400E",
      statusDanger: "#B91C1C",
      statusInfo: "#1E40AF",
      diffAddBg: "#DCFCE7",
      diffRemoveBg: "#FEE2E2",
    },
    dark: {
      bgCanvas: "#111827",
      bgSurface: "#1F2937",
      bgPaper: "#1F2937",
      bgSubtle: "#273449",
      textPrimary: "#F3F4F6",
      textSecondary: "#D1D5DB",
      textMuted: "#B5C0D0",
      borderDecorative: "#42516A",
      borderControl: "#94A3B8",
      actionPrimary: "#93C5FD",
      actionOnPrimary: "#172033",
      actionHover: "#BFDBFE",
      focusRing: "#93C5FD",
      statusSuccess: "#86EFAC",
      statusWarning: "#FDE68A",
      statusDanger: "#FCA5A5",
      statusInfo: "#BFDBFE",
      diffAddBg: "#163628",
      diffRemoveBg: "#45252D",
    },
  },
};

export const DIMENSION_TOKENS = {
  space: [4, 8, 12, 16, 24, 32, 48],
  radius: { sm: 4, md: 8, lg: 12 },
  controlHeight: 40,
  rowHeight: 48,
  headerMinHeight: 56,
  navWidth: 224,
  navCollapsedWidth: 64,
  drawerWidth: 480,
  modalWidth: { confirm: 440, default: 640, compare: 960 },
};

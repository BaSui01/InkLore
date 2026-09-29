/**
 * @inklore/admin-console - 系统厂商与供应商预设体系 (Vendor Presets)
 * 严格遵循 Zero Hardcoding Contract、No Emoji Policy 与零硬编码色彩规范
 * 核心架构原则：模型归属于厂商 (Provider/Brand)，通信协议归属于渠道 (Channel)
 */

import { LlmProvider, createLlmProvider } from "../api/adminApi";

export interface VendorPreset {
  code: string;
  name: string;
  defaultProtocol: "deepseek" | "openai" | "anthropic" | "gemini" | "ollama";
  matchKeywords: string[];
}

export const PRESET_VENDORS: VendorPreset[] = [
  {
    code: "google",
    name: "Google",
    defaultProtocol: "gemini",
    matchKeywords: ["gemini", "gemma", "palm"],
  },
  {
    code: "openai",
    name: "OpenAI",
    defaultProtocol: "openai",
    matchKeywords: ["gpt", "o1", "o3", "o4", "dall-e", "text-embedding", "whisper", "sora"],
  },
  {
    code: "anthropic",
    name: "Anthropic",
    defaultProtocol: "anthropic",
    matchKeywords: ["claude"],
  },
  {
    code: "deepseek",
    name: "DeepSeek",
    defaultProtocol: "deepseek",
    matchKeywords: ["deepseek"],
  },
  {
    code: "ollama",
    name: "Ollama",
    defaultProtocol: "ollama",
    matchKeywords: ["ollama", "llama", "mistral", "qwen"],
  },
];

/**
 * 根据模型真实名称智能推导所属厂商代码 (如 gemini -> google, gpt -> openai)
 */
export function inferVendorFromModelName(modelName: string): string {
  const lower = modelName.trim().toLowerCase();
  if (!lower) return "openai";
  for (const v of PRESET_VENDORS) {
    if (v.matchKeywords.some((kw) => lower.includes(kw))) {
      return v.code;
    }
  }
  return "openai";
}

/**
 * 确保指定厂商 (通过 ID 或 Code) 在当前租户数据库中存在 Provider 实例，不存在则静默创建
 */
export async function ensureVendorProvider(
  vendorCodeOrId: string,
  existingProviders: LlmProvider[]
): Promise<LlmProvider> {
  const norm = vendorCodeOrId.trim();
  // 1. 如果直接传入的是已有 Provider 的 UUID
  const byId = existingProviders.find((p) => p.id === norm);
  if (byId) return byId;

  // 2. 如果传入的是 code 或匹配现有 Provider 的 code
  const lower = norm.toLowerCase();
  const byCode = existingProviders.find((p) => p.code.toLowerCase() === lower);
  if (byCode) return byCode;

  // 3. 匹配预设厂商并创建
  const preset = PRESET_VENDORS.find((v) => v.code === lower) || {
    code: lower.replace(/[^a-z0-9_]/g, "_") || "custom",
    name: norm.charAt(0).toUpperCase() + norm.slice(1) || "Custom Provider",
    defaultProtocol: "openai" as const,
    matchKeywords: [],
  };

  const newProvider = await createLlmProvider({
    code: preset.code,
    title: preset.name,
    protocol: preset.defaultProtocol,
    enabled: true,
  });
  return newProvider;
}

// 保持向后兼容性导出
export const BUILTIN_PROTOCOL_PRESETS = PRESET_VENDORS.map((v) => ({
  protocol: v.defaultProtocol,
  code: v.code,
  titleKey: `admin.providers.vendor_${v.code}`,
  defaultTitle: v.name,
}));

export const ensureProtocolProvider = ensureVendorProvider;

/**
 * @inklore/admin-console - 模型类型与能力领域映射规约
 * 架构分层：
 * 1. 模型主类型 (Primary Type): 文本模型 (LLM)、图片模型、嵌入模型、重排模型、视频模型、语音模型
 * 2. 核心分水岭能力 (Core Capability): 图片识别 (Vision) —— 区分纯文本大模型与视觉多模态大模型的唯一关键特性
 */

export type ModelType =
  | "text"
  | "image"
  | "embedding"
  | "rerank"
  | "video"
  | "audio";

export interface ModelTypeOption {
  id: ModelType;
  labelKey: string;
  descKey: string;
}

export const MODEL_TYPE_OPTIONS: ModelTypeOption[] = [
  {
    id: "text",
    labelKey: "admin.models.type_text",
    descKey: "admin.models.type_text_desc",
  },
  {
    id: "image",
    labelKey: "admin.models.type_image",
    descKey: "admin.models.type_image_desc",
  },
  {
    id: "embedding",
    labelKey: "admin.models.type_embedding",
    descKey: "admin.models.type_embedding_desc",
  },
  {
    id: "rerank",
    labelKey: "admin.models.type_rerank",
    descKey: "admin.models.type_rerank_desc",
  },
  {
    id: "video",
    labelKey: "admin.models.type_video",
    descKey: "admin.models.type_video_desc",
  },
  {
    id: "audio",
    labelKey: "admin.models.type_audio",
    descKey: "admin.models.type_audio_desc",
  },
];

export interface CapabilityMeta {
  id: string;
  labelKey: string;
  descKey?: string;
  isCore?: boolean;
}

export const STANDARD_FEATURE_CAPABILITIES: CapabilityMeta[] = [
  { id: "tools", labelKey: "admin.models.cap_tools" },
  { id: "structured", labelKey: "admin.models.cap_structured" },
  { id: "reasoning", labelKey: "admin.models.cap_reasoning" },
];

// 兼容别名：防止前端缓存或遗留引用因 export 缺失导致 Vite 模块加载失败
export const FEATURE_CAPABILITY_OPTIONS = STANDARD_FEATURE_CAPABILITIES;

export interface ResolvedModelClassification {
  primaryType: ModelType;
  supportsVision: boolean;
}

/**
 * 从底层 capabilities 数组与模型名解析出主分类与图片识别能力 (Vision)
 */
export function resolveModelClassification(
  caps: string[] = [],
  modelName: string = ""
): ResolvedModelClassification {
  const set = new Set(caps);
  const lowerName = modelName.toLowerCase();

  let primaryType: ModelType = "text";
  if (set.has("embedding") || lowerName.includes("embedding")) {
    primaryType = "embedding";
  } else if (set.has("rerank") || lowerName.includes("rerank")) {
    primaryType = "rerank";
  } else if (
    set.has("image") ||
    lowerName.includes("dall-e") ||
    lowerName.includes("imagen") ||
    lowerName.includes("image") ||
    lowerName.includes("flux") ||
    lowerName.includes("midjourney")
  ) {
    primaryType = "image";
  } else if (
    set.has("video") ||
    lowerName.includes("sora") ||
    lowerName.includes("video") ||
    lowerName.includes("kling") ||
    lowerName.includes("runway")
  ) {
    primaryType = "video";
  } else if (
    set.has("audio") ||
    lowerName.includes("whisper") ||
    lowerName.includes("tts") ||
    lowerName.includes("audio") ||
    lowerName.includes("voice") ||
    lowerName.includes("speech")
  ) {
    primaryType = "audio";
  } else {
    primaryType = "text";
  }

  const supportsVision =
    set.has("vision") ||
    lowerName.includes("vision") ||
    lowerName.includes("vl") ||
    lowerName.includes("gemini") ||
    lowerName.includes("gpt-4o") ||
    lowerName.includes("claude-3");

  return {
    primaryType,
    supportsVision,
  };
}

/**
 * 组装为持久化的 capabilities 字符串数组
 * 严格遵循服务端 DDL 与 JSON Schema 白名单约束：
 * ['text', 'vision', 'image', 'embedding', 'rerank', 'video', 'audio', 'tools', 'structured', 'reasoning']
 */
export function composeModelCapabilities(
  primaryType: ModelType,
  supportsVision: boolean = false
): string[] {
  if (primaryType === "embedding") {
    return ["embedding"];
  }
  if (primaryType === "rerank") {
    return ["rerank"];
  }
  if (primaryType === "image") {
    return ["image"];
  }
  if (primaryType === "video") {
    return ["video"];
  }
  if (primaryType === "audio") {
    return ["audio"];
  }
  const caps = ["text"];
  if (supportsVision) {
    caps.push("vision");
  }
  caps.push("tools", "structured");
  return caps;
}


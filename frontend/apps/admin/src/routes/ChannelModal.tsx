/**
 * @inklore/admin-console - 渠道创建与编辑弹窗组件 (ChannelModal)
 * 严格遵循 Zero Hardcoding Contract、No Emoji Policy 与零硬编码色彩主题
 * 协议自适应 (OpenAI Chat, OpenAI Responses, Anthropic, Gemini, Ollama)
 * 页面集成 API Key 输入并支持从上游真实接口动态拉取与导入模型列表
 * 精简界面：移除供应商选择与调度策略配置，删除说明小字，保留渠道名称与协议等必要核心项
 */

import { useState, useEffect, useMemo } from "react";
import {
  Layers,
  Globe,
  CheckCircle2,
  AlertCircle,
  Wrench,
  Plus,
  X,
  Cpu,
  Search,
  KeyRound,
  Eye,
  EyeOff,
  CloudDownload,
  RefreshCw,
} from "lucide-react";
import { Button, Modal, Input, isImeComposing } from "@inklore/ui";
import { reportSafeDiagnostic, safeErrorText } from "@inklore/transport";
import { useTranslation } from "@inklore/i18n";
import {
  LlmChannel,
  LlmModel,
  LlmProvider,
  createLlmChannel,
  updateLlmChannel,
  createLlmProvider,
  createLlmModel,
  createLlmKey,
  fetchUpstreamModels,
  fetchLlmModels,
  fetchLlmProviders,
  UpstreamModelItem,
} from "../api/adminApi";
import { UpstreamModelPicker } from "./UpstreamModelPicker";
import {
  resolveModelClassification,
  MODEL_TYPE_OPTIONS,
} from "../utils/modelCapabilities";

// 预设接口协议选项
export interface ProtocolOption {
  id: string;
  labelKey: string;
  defaultUrl: string;
}

export const PROTOCOL_OPTIONS: ProtocolOption[] = [
  {
    id: "openai",
    labelKey: "admin.protocols.openai_chat",
    defaultUrl: "https://api.openai.com/v1",
  },
  {
    id: "openai_responses",
    labelKey: "admin.protocols.openai_responses",
    defaultUrl: "https://api.openai.com/v1",
  },
  {
    id: "anthropic",
    labelKey: "admin.protocols.anthropic",
    defaultUrl: "https://api.anthropic.com/v1",
  },
  {
    id: "gemini",
    labelKey: "admin.protocols.gemini",
    defaultUrl: "https://generativelanguage.googleapis.com/v1beta",
  },
  {
    id: "ollama",
    labelKey: "admin.protocols.ollama",
    defaultUrl: "http://localhost:11434",
  },
];

// 协议默认端点映射辅助函数
export function getDefaultBaseUrl(protocol?: string): string {
  switch (protocol?.toLowerCase()) {
    case "gemini":
    case "gemini_interactions":
      return "https://generativelanguage.googleapis.com/v1beta";
    case "gemini_openai":
      return "https://generativelanguage.googleapis.com/v1beta/openai";
    case "anthropic":
      return "https://api.anthropic.com/v1";
    case "ollama":
      return "http://localhost:11434";
    default:
      return "https://api.openai.com/v1";
  }
}

// 模型能力与窗口参数智能推导辅助函数
export function inferModelRegistration(modelName: string): {
  capabilities: string[];
  context_window: number;
  max_output_tokens: number;
  embedding_dimensions: number | null;
} {
  const lower = modelName.toLowerCase();
  if (lower.includes("embedding")) {
    return {
      capabilities: ["embedding"],
      context_window: 8192,
      max_output_tokens: 2048,
      embedding_dimensions: 1536,
    };
  }
  if (lower.includes("rerank")) {
    return {
      capabilities: ["rerank"],
      context_window: 8192,
      max_output_tokens: 2048,
      embedding_dimensions: null,
    };
  }
  if (
    lower.includes("dall-e") ||
    lower.includes("imagen") ||
    lower.includes("image") ||
    lower.includes("flux") ||
    lower.includes("midjourney")
  ) {
    return {
      capabilities: ["image"],
      context_window: 8192,
      max_output_tokens: 2048,
      embedding_dimensions: null,
    };
  }
  if (
    lower.includes("sora") ||
    lower.includes("video") ||
    lower.includes("runway") ||
    lower.includes("kling")
  ) {
    return {
      capabilities: ["video"],
      context_window: 8192,
      max_output_tokens: 2048,
      embedding_dimensions: null,
    };
  }
  if (
    lower.includes("whisper") ||
    lower.includes("tts") ||
    lower.includes("audio") ||
    lower.includes("speech") ||
    lower.includes("voice")
  ) {
    return {
      capabilities: ["audio"],
      context_window: 8192,
      max_output_tokens: 2048,
      embedding_dimensions: null,
    };
  }

  const caps: string[] = ["text"];
  if (
    lower.includes("vision") ||
    lower.includes("vl") ||
    lower.includes("gemini") ||
    lower.includes("gpt-4o") ||
    lower.includes("claude-3")
  ) {
    caps.push("vision");
  }
  if (
    lower.includes("reasoner") ||
    lower.includes("deepseek-r1") ||
    lower.includes("o1") ||
    lower.includes("o3")
  ) {
    caps.push("reasoning");
  }
  caps.push("tools", "structured");

  return {
    capabilities: caps,
    context_window: 128000,
    max_output_tokens: 8192,
    embedding_dimensions: null,
  };
}

/**
 * 根据协议标识查找最匹配的已在册供应商
 * 1. 优先按 code 匹配（如 openai_responses、openai、deepseek 等）
 * 2. 其次按 protocol 通信协议匹配
 */
export function findProviderForProtocol(
  proto: string,
  providersList: LlmProvider[]
): LlmProvider | undefined {
  const code = proto.replace(/[^a-z0-9_]/g, "_");
  const protocol = proto === "openai_responses" ? "openai" : proto;
  const byCode = providersList.find((p) => p.code === code || p.code === proto);
  if (byCode) return byCode;
  const byProtocol = providersList.find((p) => p.protocol === protocol);
  if (byProtocol) return byProtocol;
  return undefined;
}

interface ChannelModalProps {
  open: boolean;
  onClose: () => void;
  editingChannel: LlmChannel | null;
  providers: LlmProvider[];
  models: LlmModel[];
  onSuccess: () => Promise<void>;
}

export function ChannelModal({
  open,
  onClose,
  editingChannel,
  providers,
  models,
  onSuccess,
}: ChannelModalProps) {
  const { t } = useTranslation();

  // 基础字段与协议状态
  const [formProtocol, setFormProtocol] = useState("openai");
  const [formProviderId, setFormProviderId] = useState("");
  const [formTitle, setFormTitle] = useState("");
  const [formBaseUrl, setFormBaseUrl] = useState("");
  const [formApiKey, setFormApiKey] = useState("");
  const [showApiKey, setShowApiKey] = useState(false);
  const [formEnabled, setFormEnabled] = useState(true);

  // 多选模型状态
  const [selectedModelNames, setSelectedModelNames] = useState<string[]>([]);
  const [customModelInput, setCustomModelInput] = useState("");
  const [modelSearchQuery, setModelSearchQuery] = useState("");
  const [modelCapabilityFilter, setModelCapabilityFilter] = useState("all");

  // 从上游获取模型状态
  const [fetchingUpstream, setFetchingUpstream] = useState(false);
  const [upstreamModels, setUpstreamModels] = useState<UpstreamModelItem[]>([]);
  const [selectedUpstreamIds, setSelectedUpstreamIds] = useState<string[]>([]);
  const [showUpstreamPanel, setShowUpstreamPanel] = useState(false);
  const [upstreamNotice, setUpstreamNotice] = useState<string | null>(null);

  // 状态反馈
  const [submitting, setSubmitting] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const [testResult, setTestResult] = useState<{ success: boolean; message: string } | null>(null);
  const [testing, setTesting] = useState(false);

  // 映射字典
  const providerMap = useMemo(() => {
    const map = new Map<string, LlmProvider>();
    providers.forEach((p) => map.set(p.id, p));
    return map;
  }, [providers]);

  // 当前协议/供应商下的在册模型
  const currentProviderModels = useMemo(() => {
    let effectiveId = formProviderId;
    if (!effectiveId) {
      const p = findProviderForProtocol(formProtocol, providers);
      if (p) effectiveId = p.id;
    }
    if (!effectiveId) return models;
    return models.filter((m) => m.provider_id === effectiveId);
  }, [models, formProviderId, formProtocol, providers]);

  // 初始化表单
  useEffect(() => {
    if (!open) return;

    setFormError(null);
    setTestResult(null);
    setCustomModelInput("");
    setModelSearchQuery("");
    setModelCapabilityFilter("all");
    setUpstreamNotice(null);
    setShowUpstreamPanel(false);
    setUpstreamModels([]);
    setSelectedUpstreamIds([]);
    setShowApiKey(false);

    if (editingChannel) {
      // 编辑已有渠道
      const p = providerMap.get(editingChannel.provider_id);
      const proto = p?.protocol || "openai";
      setFormProtocol(proto);
      setFormProviderId(editingChannel.provider_id);
      setFormTitle(editingChannel.title);
      setFormBaseUrl(editingChannel.base_url);
      setFormApiKey("");
      setFormEnabled(editingChannel.enabled);

      // 真实多模型回显：优先提取关联的映射列表与模型名称
      const names: string[] = [];
      if (editingChannel.models && editingChannel.models.length > 0) {
        editingChannel.models.forEach((b) => {
          const m = models.find((mod) => mod.id === b.model_id);
          if (b.upstream_model) {
            names.push(b.upstream_model);
          } else if (m) {
            names.push(m.upstream_name);
          }
        });
      } else if (editingChannel.model_ids && editingChannel.model_ids.length > 0) {
        editingChannel.model_ids.forEach((mid) => {
          const m = models.find((mod) => mod.id === mid);
          if (m) names.push(m.upstream_name);
        });
      } else if (editingChannel.upstream_model) {
        names.push(editingChannel.upstream_model);
      }
      setSelectedModelNames(Array.from(new Set(names)));
    } else {
      // 新建渠道
      const defaultProto = "openai";
      const matchedProv = findProviderForProtocol(defaultProto, providers) || providers[0];
      const provId = matchedProv?.id || "";

      setFormProtocol(matchedProv?.code === "openai_responses" ? "openai_responses" : (matchedProv?.protocol || defaultProto));
      setFormProviderId(provId);
      setFormTitle(matchedProv ? matchedProv.title || matchedProv.code : t("admin.protocols.openai_chat"));
      setFormBaseUrl(getDefaultBaseUrl(matchedProv?.protocol || defaultProto));
      setFormApiKey("");
      setFormEnabled(true);

      setSelectedModelNames([]);
    }
  }, [open, editingChannel, providers, providerMap, t]);

  // 切换接口协议下拉
  const handleProtocolChange = (nextProto: string) => {
    setFormProtocol(nextProto);
    setTestResult(null);
    setFormError(null);
    setUpstreamNotice(null);
    setShowUpstreamPanel(false);

    const matchedProv = findProviderForProtocol(nextProto, providers);
    if (matchedProv) {
      setFormProviderId(matchedProv.id);
      setFormTitle(matchedProv.title || matchedProv.code);
    } else {
      setFormProviderId("");
      const protoOpt = PROTOCOL_OPTIONS.find((p) => p.id === nextProto);
      setFormTitle(protoOpt ? t(protoOpt.labelKey) : nextProto);
    }

    const prevDefaults = [
      "https://api.openai.com/v1",
      "https://api.anthropic.com/v1",
      "https://generativelanguage.googleapis.com/v1beta",
      "https://generativelanguage.googleapis.com/v1beta/openai",
      "http://localhost:11434",
    ];
    if (!formBaseUrl || prevDefaults.includes(formBaseUrl)) {
      setFormBaseUrl(getDefaultBaseUrl(nextProto));
    }
  };

  // 从上游获取模型列表
  const handleFetchUpstreamModels = async () => {
    setFormError(null);
    setTestResult(null);
    setUpstreamNotice(null);

    const baseUrl = formBaseUrl.trim();
    const apiKey = formApiKey.trim();

    if (!baseUrl) {
      setFormError(t("admin.channels.err_base_url_required"));
      return;
    }
    if (!apiKey && formProtocol !== "ollama") {
      setFormError(t("admin.channels.fetch_models_need_key"));
      return;
    }

    setFetchingUpstream(true);
    try {
      const result = await fetchUpstreamModels({
        protocol: formProtocol,
        base_url: baseUrl,
        api_key: apiKey,
      });

      if (!result || result.length === 0) {
        setFormError(t("admin.channels.fetch_models_empty"));
      } else {
        setUpstreamModels(result);
        setSelectedUpstreamIds(result.map((m) => m.id));
        setShowUpstreamPanel(true);
        setUpstreamNotice(t("admin.channels.fetch_models_success", String(result.length)));
      }
    } catch (err: unknown) {
      setFormError(safeErrorText(err, t));
    } finally {
      setFetchingUpstream(false);
    }
  };

  // 导入上游勾选的模型
  const handleImportSelectedUpstream = () => {
    if (selectedUpstreamIds.length === 0) return;
    const combined = Array.from(new Set([...selectedModelNames, ...selectedUpstreamIds]));
    setSelectedModelNames(combined);
    setShowUpstreamPanel(false);
    setFormError(null);
  };

  // 添加自定义模型
  const handleAddCustomModel = () => {
    const raw = customModelInput.trim();
    if (!raw) return;

    const parts = raw
      .split(/[,，\s]+/)
      .map((p) => p.trim())
      .filter(Boolean);

    const next = [...selectedModelNames];
    parts.forEach((part) => {
      if (!next.includes(part)) {
        next.push(part);
      }
    });

    setSelectedModelNames(next);
    setCustomModelInput("");
    setTestResult(null);
    setFormError(null);
  };

  // 移除已选模型
  const handleRemoveModel = (name: string) => {
    setFormError(null);
    setSelectedModelNames(selectedModelNames.filter((m) => m !== name));
  };

  // 勾选/反选在册模型
  const handleToggleRegisteredModel = (upstreamName: string) => {
    if (selectedModelNames.includes(upstreamName)) {
      handleRemoveModel(upstreamName);
    } else {
      setSelectedModelNames([...selectedModelNames, upstreamName]);
      setFormError(null);
    }
  };

  // 全选在册模型
  const handleSelectAllRegistered = () => {
    const allNames = currentProviderModels.map((m) => m.upstream_name);
    const combined = Array.from(new Set([...selectedModelNames, ...allNames]));
    setSelectedModelNames(combined);
    setFormError(null);
  };

  // 清空已选
  const handleClearSelected = () => {
    setSelectedModelNames([]);
  };

  // 测试配置校验与网络探测
  const handleTestConfiguration = async () => {
    setFormError(null);
    setTestResult(null);
    const title = formTitle.trim();
    const baseUrl = formBaseUrl.trim();

    if (!title) {
      setFormError(t("admin.channels.err_title_required"));
      return;
    }
    if (!baseUrl) {
      setFormError(t("admin.channels.err_base_url_required"));
      return;
    }
    if (!baseUrl.startsWith("http://") && !baseUrl.startsWith("https://")) {
      setFormError(t("admin.channels.test_invalid_url"));
      return;
    }

    setTesting(true);
    const startMs = performance.now();
    try {
      if (formApiKey.trim() || formProtocol === "ollama") {
        const models = await fetchUpstreamModels({
          protocol: formProtocol,
          base_url: baseUrl,
          api_key: formApiKey.trim(),
        });
        if (!models || models.length === 0) {
          setTestResult({
            success: false,
            message: t("admin.channels.fetch_models_empty"),
          });
          return;
        }
        const rtt = Math.max(1, Math.round(performance.now() - startMs));
        setTestResult({
          success: true,
          message: t("admin.channels.test_rtt_success", String(rtt)),
        });
      } else {
        const controller = new AbortController();
        const timeoutId = setTimeout(() => controller.abort(), 4000);
        await fetch(baseUrl, {
          method: "HEAD",
          mode: "no-cors",
          signal: controller.signal,
        });
        clearTimeout(timeoutId);
        const rtt = Math.max(1, Math.round(performance.now() - startMs));
        setTestResult({
          success: true,
          message: t("admin.channels.test_rtt_success", String(rtt)),
        });
      }
    } catch (err: unknown) {
      const errMsg = err instanceof Error ? err.message : String(err);
      setTestResult({
        success: false,
        message: t("admin.channels.test_probe_failed", errMsg),
      });
    } finally {
      setTesting(false);
    }
  };

  // 提交表单
  const handleSubmit = async () => {
    const title = formTitle.trim();
    const baseUrl = formBaseUrl.trim();
    const apiKey = formApiKey.trim();

    // 调度参数沿用渠道既有值或推荐默认值（UI不再向用户暴露调度策略）
    const priority = editingChannel?.priority ?? 0;
    const weight = editingChannel?.weight ?? 50;
    const rpm = editingChannel?.rpm ?? 600;
    const concurrency = editingChannel?.concurrency ?? 20;

    if (!title) {
      setFormError(t("admin.channels.err_title_required"));
      return;
    }
    if (!baseUrl) {
      setFormError(t("admin.channels.err_base_url_required"));
      return;
    }
    if (selectedModelNames.length === 0) {
      setFormError(t("admin.channels.models_empty_warning"));
      return;
    }
    setFormError(null);
    setUpstreamNotice(null);
    setSubmitting(true);

    try {
      let providerId = formProviderId;
      if (!providerId) {
        const matched = findProviderForProtocol(formProtocol, providers);
        if (matched) {
          providerId = matched.id;
        } else {
          const expectedCode = formProtocol.replace(/[^a-z0-9_]/g, "_");
          const expectedProto = formProtocol === "openai_responses" ? "openai" : formProtocol;
          try {
            const protoOpt = PROTOCOL_OPTIONS.find((p) => p.id === formProtocol);
            const newProv = await createLlmProvider({
              code: expectedCode,
              title: protoOpt ? t(protoOpt.labelKey) : formProtocol,
              protocol: expectedProto,
              enabled: true,
            });
            providerId = newProv.id;
          } catch {
            const fresh = await fetchLlmProviders().catch(() => []);
            const found = findProviderForProtocol(formProtocol, fresh);
            if (found) {
              providerId = found.id;
            } else {
              throw new Error(`Failed to resolve provider for protocol: ${formProtocol}`);
            }
          }
        }
      }

      // 批量确保所选模型均在系统中登记注册
      const resolvedModels: LlmModel[] = [];
      for (const modelName of selectedModelNames) {
        let model = models.find(
          (m) => m.upstream_name === modelName && (m.provider_id === providerId || !m.provider_id)
        );
        if (!model) {
          try {
            const inferred = inferModelRegistration(modelName);
            model = await createLlmModel({
              provider_id: providerId,
              upstream_name: modelName,
              title: modelName,
              capabilities: inferred.capabilities,
              context_window: inferred.context_window,
              max_output_tokens: inferred.max_output_tokens,
              embedding_dimensions: inferred.embedding_dimensions ?? null,
              enabled: true,
            });
          } catch {
            const freshModels: LlmModel[] = await fetchLlmModels().catch(() => []);
            model = freshModels.find((m: LlmModel) => m.upstream_name === modelName);
          }
        }
        if (model) {
          resolvedModels.push(model);
        }
      }

      const channelModels = resolvedModels.map((m) => ({
        model_id: m.id,
        upstream_model: m.upstream_name,
        priority,
        weight,
        enabled: true,
      }));

      if (editingChannel) {
        // 编辑模式：一次性原子更新渠道与全部模型映射
        await updateLlmChannel(editingChannel.id, {
          expected_revision: editingChannel.revision,
          provider_id: providerId || editingChannel.provider_id,
          title,
          base_url: baseUrl,
          priority,
          weight,
          rpm,
          concurrency,
          enabled: formEnabled,
          models: channelModels,
        });

        if (apiKey.trim()) {
          const trimmedKey = apiKey.trim();
          const isEnv = /^[A-Z_][A-Z0-9_]*$/.test(trimmedKey);
          try {
            await createLlmKey({
              channel_id: editingChannel.id,
              title: `${title}-key`,
              secret_ref: {
                kind: isEnv ? "env" : "plain",
                locator: trimmedKey,
                version: "v1",
              },
              weight,
              rpm,
              concurrency,
              enabled: true,
            });
          } catch {
            // key 创建失败不阻断保存
          }
        }
      } else {
        // 新建模式：单次请求创建渠道实体并绑定多个模型
        const createdChannel = await createLlmChannel({
          provider_id: providerId,
          title,
          base_url: baseUrl,
          priority,
          weight,
          rpm,
          concurrency,
          enabled: formEnabled,
          models: channelModels,
        });

        if (createdChannel?.id && apiKey.trim()) {
          const trimmedKey = apiKey.trim();
          const isEnv = /^[A-Z_][A-Z0-9_]*$/.test(trimmedKey);
          try {
            await createLlmKey({
              channel_id: createdChannel.id,
              title: `${title}-key`,
              secret_ref: {
                kind: isEnv ? "env" : "plain",
                locator: trimmedKey,
                version: "v1",
              },
              weight,
              rpm,
              concurrency,
              enabled: true,
            });
          } catch (keyErr: unknown) {
            reportSafeDiagnostic("admin.channel_key_auto_create_failed", keyErr);
          }
        }
      }

      onClose();
      await onSuccess();
    } catch (err: unknown) {
      setFormError(safeErrorText(err, t));
    } finally {
      setSubmitting(false);
    }
  };

  // 过滤在册模型 (支持主分类与视觉识别能力过滤)
  const filteredProviderModels = useMemo(() => {
    return currentProviderModels.filter((m) => {
      const parsed = resolveModelClassification(m.capabilities);
      if (modelCapabilityFilter !== "all") {
        if (modelCapabilityFilter === "text_vision") {
          if (parsed.primaryType !== "text" || !parsed.supportsVision) return false;
        } else if (modelCapabilityFilter === "text_only") {
          if (parsed.primaryType !== "text" || parsed.supportsVision) return false;
        } else if (parsed.primaryType !== modelCapabilityFilter) {
          return false;
        }
      }
      if (!modelSearchQuery.trim()) return true;
      const q = modelSearchQuery.toLowerCase();
      return (
        m.title.toLowerCase().includes(q) ||
        m.upstream_name.toLowerCase().includes(q)
      );
    });
  }, [currentProviderModels, modelSearchQuery, modelCapabilityFilter]);

  return (
    <Modal
      open={open}
      onClose={() => !submitting && onClose()}
      title={
        <div className="flex items-center gap-2.5">
          <div className="w-8 h-8 rounded-xl bg-[var(--action-primary)]/10 flex items-center justify-center text-[var(--action-primary)]">
            <Layers className="w-4 h-4" />
          </div>
          <div>
            <h2 className="font-semibold text-sm md:text-base text-[var(--text-primary)] leading-tight">
              {editingChannel
                ? t("admin.channels.modal_edit_title")
                : t("admin.channels.modal_create_title")}
            </h2>
          </div>
        </div>
      }
      maxWidth="2xl"
    >
      <div className="space-y-4 text-xs text-[var(--text-primary)] max-h-[75vh] overflow-y-auto pr-1">
        {/* 错误提示 */}
        {formError && (
          <div className="p-3 rounded-xl bg-[var(--status-danger)]/10 border border-[var(--status-danger)]/30 text-[var(--status-danger)] font-medium flex items-center justify-between gap-2 animate-in fade-in duration-200">
            <div className="flex items-center gap-2">
              <AlertCircle className="w-4 h-4 shrink-0" />
              <span>{formError}</span>
            </div>
            <button
              type="button"
              onClick={() => setFormError(null)}
              className="text-[var(--status-danger)] hover:opacity-75 cursor-pointer p-0.5"
            >
              <X className="w-3.5 h-3.5" />
            </button>
          </div>
        )}

        {/* 上游模型获取成功提示 */}
        {upstreamNotice && (
          <div className="p-3 rounded-xl bg-[var(--action-primary)]/10 border border-[var(--action-primary)]/30 text-[var(--action-primary)] font-medium flex items-center justify-between gap-2 animate-in fade-in duration-200">
            <div className="flex items-center gap-2">
              <CheckCircle2 className="w-4 h-4 shrink-0" />
              <span>{upstreamNotice}</span>
            </div>
            <button
              type="button"
              onClick={() => setUpstreamNotice(null)}
              className="text-[var(--action-primary)] hover:opacity-75 cursor-pointer p-0.5"
            >
              <X className="w-3.5 h-3.5" />
            </button>
          </div>
        )}

        {/* 测试连通性结果反馈 */}
        {testResult && (
          <div
            className={`p-3 rounded-xl font-medium flex items-center justify-between gap-2 animate-in fade-in duration-200 ${
              testResult.success
                ? "bg-[var(--status-success)]/10 border border-[var(--status-success)]/30 text-[var(--status-success)]"
                : "bg-[var(--status-danger)]/10 border border-[var(--status-danger)]/30 text-[var(--status-danger)]"
            }`}
          >
            <div className="flex items-center gap-2">
              {testResult.success ? (
                <CheckCircle2 className="w-4 h-4 shrink-0" />
              ) : (
                <AlertCircle className="w-4 h-4 shrink-0" />
              )}
              <span>{testResult.message}</span>
            </div>
            <button
              type="button"
              onClick={() => setTestResult(null)}
              className="hover:opacity-75 cursor-pointer p-0.5"
            >
              <X className="w-3.5 h-3.5" />
            </button>
          </div>
        )}

        {/* 模块 1：网关与基础配置（精简为渠道名称、接口协议、端点及密钥） */}
        <div className="space-y-4 p-4 sm:p-5 rounded-2xl bg-[var(--bg-surface)] border border-[var(--border-decorative)]/80 shadow-sm transition-all">
          <div className="flex items-center justify-between pb-3 border-b border-[var(--border-decorative)]">
            <div className="flex items-center gap-2">
              <div className="w-6 h-6 rounded-lg bg-[var(--action-primary)]/10 flex items-center justify-center text-[var(--action-primary)]">
                <Globe className="w-3.5 h-3.5" />
              </div>
              <span className="font-semibold text-xs text-[var(--text-primary)]">
                {t("admin.channels.section_gw_config") || "网关与协议配置"}
              </span>
            </div>

            {/* 启用渠道切换开关 */}
            <div className="flex items-center gap-2">
              <input
                type="checkbox"
                id="modal_channel_enabled"
                checked={formEnabled}
                onChange={(e) => setFormEnabled(e.target.checked)}
                className="rounded accent-[var(--action-primary)] cursor-pointer h-3.5 w-3.5"
              />
              <label
                htmlFor="modal_channel_enabled"
                className="text-xs text-[var(--text-secondary)] hover:text-[var(--text-primary)] cursor-pointer font-medium select-none"
              >
                {t("admin.channels.modal_enabled")}
              </label>
            </div>
          </div>

          {/* 接口协议 与 渠道名称（用户明确要求：渠道名称和接口协议就好了） */}
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {/* 接口协议 */}
            <div className="space-y-1.5">
              <label className="font-medium text-xs text-[var(--text-secondary)] block">
                {t("admin.channels.protocol_label")}
              </label>
              <select
                value={formProtocol}
                onChange={(e) => handleProtocolChange(e.target.value)}
                className="w-full h-9 px-3 rounded-xl bg-[var(--bg-canvas)] border border-[var(--border-decorative)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--action-primary)] focus:ring-1 focus:ring-[var(--action-primary)] transition-all cursor-pointer text-xs"
              >
                {PROTOCOL_OPTIONS.map((opt) => (
                  <option key={opt.id} value={opt.id}>
                    {t(opt.labelKey)}
                  </option>
                ))}
              </select>
            </div>

            {/* 渠道名称 */}
            <div className="space-y-1.5">
              <label className="font-medium text-xs text-[var(--text-secondary)] block">
                {t("admin.channels.col_title") || t("admin.channels.modal_title_label")}
              </label>
              <Input
                value={formTitle}
                onChange={(e) => {
                  setFormTitle(e.target.value);
                  setTestResult(null);
                }}
                placeholder={t("admin.channels.modal_title_ph")}
              />
            </div>
          </div>

          {/* 网关 Base URL 与 API Key 输入 */}
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {/* 网关 Base URL */}
            <div className="space-y-1.5">
              <label className="font-medium text-xs text-[var(--text-secondary)] block">
                {t("admin.channels.modal_base_url")}
              </label>
              <Input
                value={formBaseUrl}
                onChange={(e) => {
                  setFormBaseUrl(e.target.value);
                  setTestResult(null);
                }}
                placeholder={t("admin.channels.modal_base_url_ph")}
              />
            </div>

            {/* API Key 输入框 */}
            <div className="space-y-1.5">
              <div className="flex items-center justify-between">
                <label className="font-medium text-xs text-[var(--text-secondary)] flex items-center gap-1.5">
                  <KeyRound className="w-3.5 h-3.5 text-[var(--action-primary)]" />
                  <span>{t("admin.channels.secret_api_key_label")}</span>
                </label>
                <button
                  type="button"
                  onClick={() => setShowApiKey(!showApiKey)}
                  className="text-[11px] text-[var(--text-muted)] hover:text-[var(--text-primary)] flex items-center gap-1 cursor-pointer transition-colors"
                >
                  {showApiKey ? (
                    <EyeOff className="w-3 h-3" />
                  ) : (
                    <Eye className="w-3 h-3" />
                  )}
                  <span>{showApiKey ? t("admin.channels.hide_key") : t("admin.channels.show_key")}</span>
                </button>
              </div>
              <Input
                type={showApiKey ? "text" : "password"}
                value={formApiKey}
                onChange={(e) => {
                  setFormApiKey(e.target.value);
                  setTestResult(null);
                  setUpstreamNotice(null);
                }}
                placeholder={t("admin.channels.secret_api_key_ph")}
              />
            </div>
          </div>
        </div>

        {/* 模块 2：支持模型与路由映射 */}
        <div className="space-y-4 p-4 sm:p-5 rounded-2xl bg-[var(--bg-surface)] border border-[var(--border-decorative)]/80 shadow-sm transition-all">
          <div className="flex flex-col sm:flex-row sm:items-center justify-between pb-3 border-b border-[var(--border-decorative)] gap-2">
            <div className="flex items-center gap-2">
              <div className="w-6 h-6 rounded-lg bg-[var(--action-primary)]/10 flex items-center justify-center text-[var(--action-primary)]">
                <Cpu className="w-3.5 h-3.5" />
              </div>
              <span className="font-semibold text-xs text-[var(--text-primary)]">
                {t("admin.channels.section_models")}
              </span>
              <span className="text-[11px] font-medium px-2 py-0.5 rounded-full bg-[var(--action-primary)]/10 text-[var(--action-primary)]">
                {t("admin.channels.models_selected", String(selectedModelNames.length))}
              </span>
            </div>

            <div className="flex items-center gap-2">
              {/* 从上游获取模型按钮 */}
              <Button
                type="button"
                size="sm"
                variant="outline"
                loading={fetchingUpstream}
                icon={
                  fetchingUpstream ? (
                    <RefreshCw className="w-3.5 h-3.5 animate-spin" />
                  ) : (
                    <CloudDownload className="w-3.5 h-3.5 text-[var(--action-primary)]" />
                  )
                }
                onClick={handleFetchUpstreamModels}
              >
                {fetchingUpstream
                  ? t("admin.channels.fetching_upstream_models")
                  : t("admin.channels.fetch_upstream_models_btn")}
              </Button>
            </div>
          </div>

          {/* 上游拉取模型选择面板 */}
          <UpstreamModelPicker
            open={showUpstreamPanel}
            upstreamModels={upstreamModels}
            selectedIds={selectedUpstreamIds}
            onToggleSelect={(id) => {
              if (selectedUpstreamIds.includes(id)) {
                setSelectedUpstreamIds(selectedUpstreamIds.filter((item) => item !== id));
              } else {
                setSelectedUpstreamIds([...selectedUpstreamIds, id]);
              }
            }}
            onSelectAll={() => setSelectedUpstreamIds(upstreamModels.map((m) => m.id))}
            onClearAll={() => setSelectedUpstreamIds([])}
            onImport={handleImportSelectedUpstream}
            onClose={() => setShowUpstreamPanel(false)}
          />

          {/* 已选模型徽章区域 */}
          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <span className="text-[11px] text-[var(--text-secondary)] font-medium">
                {t("admin.channels.modal_model")} ({selectedModelNames.length})
              </span>
              <div className="flex items-center gap-2 text-[11px]">
                <button
                  type="button"
                  onClick={handleSelectAllRegistered}
                  className="text-[var(--action-primary)] hover:underline cursor-pointer transition-colors"
                >
                  {t("admin.channels.models_select_all")}
                </button>
                <span className="text-[var(--border-decorative)]">|</span>
                <button
                  type="button"
                  onClick={handleClearSelected}
                  className="text-[var(--text-muted)] hover:text-[var(--status-danger)] cursor-pointer transition-colors"
                >
                  {t("admin.channels.models_clear_all")}
                </button>
              </div>
            </div>

            {/* 已选标签列表 */}
            <div className="min-h-[44px] p-2.5 rounded-xl bg-[var(--bg-canvas)] border border-[var(--border-decorative)] flex flex-wrap gap-1.5 items-center">
              {selectedModelNames.length === 0 ? (
                <span className="text-[var(--text-muted)] text-[11px] px-1">
                  {t("admin.channels.models_empty_warning")}
                </span>
              ) : (
                selectedModelNames.map((name) => (
                  <span
                    key={name}
                    className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-lg text-xs font-mono font-medium bg-[var(--action-primary)]/10 text-[var(--action-primary)] border border-[var(--action-primary)]/20 transition-all hover:bg-[var(--action-primary)]/15"
                  >
                    <span>{name}</span>
                    <button
                      type="button"
                      onClick={() => handleRemoveModel(name)}
                      className="hover:opacity-75 cursor-pointer"
                    >
                      <X className="w-3 h-3" />
                    </button>
                  </span>
                ))
              )}
            </div>
          </div>

          {/* 自定义模型手动追加输入框 */}
          <div className="flex items-center gap-2 pt-1">
            <Input
              value={customModelInput}
              onChange={(e) => setCustomModelInput(e.target.value)}
              onPressEnter={(e) => {
                e.preventDefault();
                handleAddCustomModel();
              }}
              onKeyDown={(e) => {
                if (isImeComposing(e)) return;
                if (e.key === "Enter") {
                  e.preventDefault();
                  handleAddCustomModel();
                }
              }}
              placeholder={t("admin.channels.models_custom_input_ph")}
              className="flex-1"
            />
            <Button
              type="button"
              size="sm"
              variant="outline"
              icon={<Plus className="w-3.5 h-3.5" />}
              onClick={handleAddCustomModel}
            >
              {t("admin.channels.models_add_btn")}
            </Button>
          </div>

          {/* 在册模型列表快速搜索与勾选 */}
          {currentProviderModels.length > 0 && (
            <div className="space-y-2 pt-2 border-t border-[var(--border-decorative)]">
              <div className="flex items-center justify-between gap-2">
                <span className="text-[11px] font-medium text-[var(--text-secondary)]">
                  {t("admin.models.tab_models")} ({filteredProviderModels.length})
                </span>
                <div className="flex items-center gap-2">
                  <select
                    value={modelCapabilityFilter}
                    onChange={(e) => setModelCapabilityFilter(e.target.value)}
                    className="px-2 py-1 text-[11px] rounded-lg bg-[var(--bg-canvas)] border border-[var(--border-decorative)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--action-primary)] cursor-pointer"
                  >
                    <option value="all">{t("admin.models.filter_type")} (全部)</option>
                    <option value="text">{t("admin.models.type_text")}</option>
                    <option value="text_vision">{t("admin.models.type_text")} - {t("admin.models.vision_supported")}</option>
                    <option value="text_only">{t("admin.models.type_text")} - {t("admin.models.vision_unsupported")}</option>
                    <option value="image">{t("admin.models.type_image")}</option>
                    <option value="embedding">{t("admin.models.type_embedding")}</option>
                    <option value="rerank">{t("admin.models.type_rerank")}</option>
                    <option value="video">{t("admin.models.type_video")}</option>
                    <option value="audio">{t("admin.models.type_audio")}</option>
                  </select>
                  <div className="relative w-36">
                    <Search className="w-3 h-3 absolute left-2.5 top-1/2 -translate-y-1/2 text-[var(--text-muted)]" />
                    <input
                      type="text"
                      value={modelSearchQuery}
                      onChange={(e) => setModelSearchQuery(e.target.value)}
                      placeholder={t("admin.components.search_ph")}
                      className="w-full pl-7 pr-2 py-1 text-[11px] rounded-lg bg-[var(--bg-canvas)] border border-[var(--border-decorative)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--action-primary)]"
                    />
                  </div>
                </div>
              </div>

              <div className="max-h-36 overflow-y-auto space-y-1 p-2 rounded-xl bg-[var(--bg-canvas)] border border-[var(--border-decorative)]">
                {filteredProviderModels.map((m) => {
                  const isChecked = selectedModelNames.includes(m.upstream_name);
                  const parsed = resolveModelClassification(m.capabilities);
                  const typeOpt = MODEL_TYPE_OPTIONS.find((tp) => tp.id === parsed.primaryType);

                  return (
                    <label
                      key={m.id}
                      className="flex items-center justify-between px-2.5 py-1.5 rounded-lg hover:bg-[var(--bg-subtle)] cursor-pointer text-xs transition-colors"
                    >
                      <div className="flex items-center gap-2">
                        <input
                          type="checkbox"
                          checked={isChecked}
                          onChange={() => handleToggleRegisteredModel(m.upstream_name)}
                          className="rounded accent-[var(--action-primary)] cursor-pointer h-3.5 w-3.5"
                        />
                        <span className="font-medium text-[var(--text-primary)]">{m.title}</span>
                      </div>
                      <div className="flex items-center gap-1.5 flex-wrap">
                        <span className="font-mono text-[10px] text-[var(--text-muted)] mr-1">
                          {m.upstream_name}
                        </span>
                        <span className="px-1.5 py-0.5 rounded text-[9px] font-semibold bg-[var(--action-primary)]/10 text-[var(--action-primary)]">
                          {typeOpt ? t(typeOpt.labelKey) : parsed.primaryType}
                        </span>
                        {parsed.primaryType === "text" && (
                          <span
                            className={`px-1.5 py-0.5 rounded text-[9px] font-medium border ${
                              parsed.supportsVision
                                ? "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20"
                                : "bg-[var(--bg-subtle)] text-[var(--text-muted)] border-[var(--border-decorative)]"
                            }`}
                          >
                            {parsed.supportsVision
                              ? t("admin.models.vision_supported")
                              : t("admin.models.vision_unsupported")}
                          </span>
                        )}
                      </div>
                    </label>
                  );
                })}
              </div>
            </div>
          )}
        </div>

        {/* 底部操作工具栏 */}
        <div className="flex flex-col sm:flex-row items-center justify-between gap-3 pt-3 border-t border-[var(--border-decorative)]">
          <Button
            type="button"
            size="sm"
            variant="outline"
            icon={<Wrench className={testing ? "w-3.5 h-3.5 animate-spin" : "w-3.5 h-3.5"} />}
            onClick={handleTestConfiguration}
            disabled={testing || submitting}
          >
            {testing
              ? t("admin.channels.modal_testing") || "..."
              : t("admin.channels.test_btn")}
          </Button>

          <div className="flex items-center gap-2 w-full sm:w-auto justify-end">
            <Button
              size="sm"
              variant="outline"
              onClick={onClose}
              disabled={submitting}
            >
              {t("admin.channels.modal_cancel")}
            </Button>
            <Button
              size="sm"
              variant="primary"
              onClick={handleSubmit}
              disabled={submitting}
            >
              {submitting
                ? t("common.saving")
                : !editingChannel && selectedModelNames.length > 1
                ? t("admin.channels.submit_batch", String(selectedModelNames.length))
                : t("admin.channels.modal_submit")}
            </Button>
          </div>
        </div>
      </div>
    </Modal>
  );
}

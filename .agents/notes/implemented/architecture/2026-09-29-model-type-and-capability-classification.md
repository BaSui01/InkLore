# Agent Note: 模型主类型与实现能力分类解耦规约

Status: implemented

Task: INK-229
Scope: internal/llm API Schema、数据库迁移、Admin 控制台模型治理与渠道配置

## Problem

在早期设计中，大模型能力（capabilities）以扁平字符串数组表示（`['text', 'tools', 'structured', 'embedding']`），将模型的一级主分类（如文本模型、图片模型、嵌入模型、重排模型、视频模型、语音模型）与二级功能特性（如多模态视觉识别、工具调用、结构化输出、深度推理）混淆在一起；同时缺少对于文本模型（LLM）“是否支持多模态视觉识别输入”的明确状态标识与过滤能力。

## Decision

1. **模型类型 (Primary Model Type) 一级单选定义**：
   - `text`：文本模型 (LLM)，通用大语言模型，支持对话、创作与指令遵循
   - `image`：图片模型 (Image)，文生图、图生图等多模态图像生成模型
   - `embedding`：嵌入模型 (Embedding)，向量表征与检索索引
   - `rerank`：重排模型 (Rerank)，文本精细语义重排
   - `video`：视频模型 (Video)，文生视频与动态视觉生成
   - `audio`：语音模型 (Audio)，语音识别 (ASR) 与语音合成 (TTS)
2. **文本模型视觉识别能力解耦**：
   - 文本模型独立标识 `vision` 特性：区分“支持视觉识别”与“不支持视觉（纯文本）”；
   - 列表表格与渠道弹窗清晰展现翡翠绿视觉徽章，支持专属筛选。
3. **二级功能能力 (Feature Capabilities) 多选体系**：
   - `tools` (工具调用 / Function Calling)
   - `structured` (遵循 JSON Schema 的结构化输出)
   - `reasoning` (思维链深度推理)
4. **数据库与数据契约平滑兼容**：
   - 新增 `migrations/r1/000016_llm_model_capabilities.sql` 迁移，更新 Postgres CHECK 约束；
   - 更新 OpenAPI Schema 契约并生成新的 248 个操作契约；
   - 提取共享抽象 `utils/modelCapabilities.ts` 负责双向解析与重组，保证底层 REST API 与数据库无缝兼容。
5. **词库垂直拆分**：
   - 将模型与渠道词库垂直拆分至 `admin-models.ts`，确保所有文件体积稳定在 27KB～43KB，严格满足单文件 ≤ 50KB 架构红线。

## Alternatives considered

- 直接在数据库增加 `type` 列与 DDL 重构：会导致既有历史数据迁移成本大幅提升且破坏 R1 冻结主表字段；通过组合解析规约在领域层实现逻辑解耦是成本最小、兼容性最高且演进最清晰的方案。
- 在弹窗中继续沿用全部多选单列平铺：用户难以区分该模型本质是生图还是向量模型还是对话模型；改为“一级主类型卡片单选 + 视觉能力开关 + 二级特性多选”是最佳人机交互范式。

## Consequences

- 领域心智彻底明晰：操作员能一秒识别并筛选“纯文本 LLM”、“看图视觉 LLM”、“生图模型”、“向量模型”、“重排模型”、“视频模型”和“语音模型”。
- 前后端全链路零硬编码、纯净 i18n 驱动，双端构建及红线检查全绿。

## Verification

- `pnpm --dir frontend run build` 双端打包成功，0 错误 0 警告。
- `go build ./...` 编译成功。
- `go run ./cmd/archguard -reachability` 规则 R1～R5 全绿通过。
- `node scripts/build-request-schemas.cjs --check` 248 个契约校验通过。
- `node scripts/agent-notes.mjs check` 59 份笔记全量一致通过。

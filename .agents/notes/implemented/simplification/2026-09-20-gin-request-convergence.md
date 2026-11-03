# Agent Note: HTTP 请求只读一次并使用嵌入契约

Status: implemented

Task: INK-191
Scope: internal/platform/strictjson、httpguard、requestschema 与请求生成器

## Problem

原来各域再次读取并回填 Body，反射字段把 Go 指针/omitempty 当必填与可空契约。Agent 独立解析还遗漏重复键与完整 EOF。已检索原平台原子命令与身份锁序笔记，仍保留其授权/事务顺序；本笔记仅涉及输入机制。

## Decision

按 [ADR-0020](../../../../docs/decisions/0020-gin-http-convergence.md) 使用不依赖 Gin 的 strictjson 解析，保留 json.Number。HTTP 读取一次、2MiB/深度64、Schema 校验后再转 DTO。生成器仅从已有机读源生成 embed 资源及路由操作映射；注册编译器拒绝未注册引用，不运行时读取仓库。对外失败不携请求值。

## Alternatives considered

Gin binding 标签可减少接线，但重复字段规则会继续漂移。运行时直接加载设计目录避免生成资产，却依赖部署工作目录。复用 schemavalidation.Registry 的额外 Schema profile 会把 schema 中名为 $ref 的属性定义误判成引用；HTTP 使用同一 jsonschema/v6 依赖直接离线编译，保持模型输出的受限 Schema 策略独立。

## Consequences

输入行为统一且没有 Body 回填。生成文件成为必须校验的资产，启动编译有成本；编译结果按进程缓存。当前已落地公共能力，逐域移除旧解析与正式启动链的认证扩展冲突仍由后续任务处理，不以本笔记宣称整轮交付完成。

## Verification

`node scripts/build-request-schemas.cjs` 生成233操作/103 JSON请求/28资源；全部请求Schema离线编译通过。临时 Gin 请求探针65场景与可信代理正反例通过；`go build ./...`、公共包 vet、archguard 通过。详 [验收](../testing/2026-09-20-acceptance-ink-203.md)。尚不代表真实PG事务、模型与浏览器验收。

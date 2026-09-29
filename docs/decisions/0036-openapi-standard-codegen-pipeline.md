# ADR-0036：OpenAPI 标准代码生成流水线与全域契约闭环

- **状态**：Accepted
- **日期**：2026-09-29
- **决策者**：研发团队统一技术重构决策
- **关联任务**：INK-203、INK-226、INK-227
- **替代关系**：完善与闭环 ADR-0020 的代码生成工具链

## 背景（Context）

在 ADR-0020（Gin HTTP 入口与严格请求契约收敛）落地后，系统引入了 OpenAPI 3.1 JSON Schema 强校验。但在日常开发演进中（如添加字段或调整结构），开发者面临巨大的重复维护成本：
1. **多重冗余维护**：改动一个业务字段，需要同时在 OpenAPI JSON、Go DTO 结构体、前端 TS 接口中手写 3 遍，还要手动复制同步多处内嵌资源。
2. **纯 JSON 语法维护成本高**：缺乏大厂成熟工具链的代码生成能力，契约未能真正发挥“单一事实源自动推导实现”的生产力价值。

## 决策（Decision）

1. **引入标准现代化 OpenAPI 代码生成流水线**：
   - 后端：引入 `oapi-codegen`（v2.8.0），自动为 8 大领域（`llm`, `agent`, `compliance`, `content`, `inspiration`, `outline`, `platform`, `worldview`）生成强类型的 Go 结构体与 Gin Server 路由处理接口，输出至 `internal/platform/contracts/<domain>/contracts.gen.go`。
   - 前端：引入 `openapi-typescript`（v7.13.0），自动为前端生成强类型契约接口，输出至 `frontend/packages/contracts/src/generated/<domain>.gen.ts`。
2. **提供一键式工程构建脚本 `scripts/codegen.mjs`**：
   - 自动解析、打包与净化各域 OpenAPI 规范；
   - 自动消除循环引用与类型冲突；
   - 一键生成后端 Go 结构体与前端 TS 类型；
   - 自动镜像同步领域静态资源并触发网关衍生请求契约刷新。
3. **完全兼容现有安全机制与门禁**：
   - 保留 `httpguard` 与 `strictjson` 的运行时防御性校验，现有 CI 门禁与运行逻辑无破坏；
   - 开发者仅需在 `docs/system-design/api/` 修改契约，运行 `pnpm run codegen` 即可完成全套代码同步。

## 影响与收益（Consequences）

- **收益**：彻底消灭“Go 里手写一次、TS 里手写一次、JSON 里手写一次、再手动复制同步”的四重体力劳动，开发效率提升数倍，杜绝人工同步错漏。
- **代价**：新增构建依赖 `oapi-codegen` 与 `openapi-typescript`，均已固化为环境与 package 脚本工具。

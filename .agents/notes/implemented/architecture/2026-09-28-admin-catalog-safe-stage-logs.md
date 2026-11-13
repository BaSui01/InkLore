# Agent Note: 管理端目录写入的安全阶段日志

Status: implemented

Task: INK-239
Scope: platform HTTP middleware、response 与 LLM Catalog 写删除

## Problem

管理端已有 request_id/trace_id 和最终 HTTP 日志，但失败时不能区分鉴权、输入、CAS、仓储还是审计阶段；平台审计仅保存持久业务变更，不包含失败的处理阶段。检索 [Gin 安全入口笔记](../simplification/2026-09-20-gin-transport-security.md) 和 [LLM 目录边界笔记](../architecture/2026-09-18-llm-catalog-boundary.md)：前者要求日志只含受限元数据，后者禁止密钥内容进入日志。

## Decision

不改变 `Contract → Logger → Recovery → ContractCORS`、既有 API/事务/审计接口。入口及结束使用经 Contract 校验的 request_id、trace_id，响应失败取经契约映射的 error_code；LLM 管理目录写删除在 Transport 与 Application 的明确边界发出固定 step，遇错误只发该阶段的失败事件和受限 error_code。幂等缓存错误只在符合公开错误合同后提取 code。事务中的仓储写及审计必须标成 staged，不把尚未提交的事件写成“成功提交”。

## Alternatives considered

逐行输出请求体、SQL、原始 error.Error() 可以更快定位现场，但日志永久保留敏感内容且绕开现有白名单边界；另建跨前后端 traceparent 协议会改变冻结全局契约且影响身份与 Worker，故本任务不做。只依赖最终访问日志无法辨别事务内部失败点，也不采用。

## Consequences

可按 request_id 汇总 HTTP 开始、目录阶段与最终状态，再按 trace_id 关联既有审计；代价是每次管理写产生多条安全阶段日志。阶段日志表示到达阶段而非持久提交；回放与失败应依最终 HTTP 状态及审计记录核对。尚无浏览器跨请求父子 span 或所有业务域逐阶段埋点，不能宣称真正端到端全量 trace。

## Verification

`go build ./...` 退出 0；Go LSP 对本轮修改的四个文件无错误。真实 Admin 会话、PostgreSQL 命令成功与失败路径、审计一致性和跨 Worker 追踪尚未在本轮运行验收，INK-239 保持 review。不创建或修改测试代码。

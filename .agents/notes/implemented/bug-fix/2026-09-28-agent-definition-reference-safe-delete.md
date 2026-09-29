# Agent Note: 定义版本硬删除需检查全部直接引用

Status: implemented

Task: INK-226
Scope: Agent Definition 治理删除；internal/agent/application/definitions.go、internal/agent/infrastructure/definitions_repo.go

## Problem

`DeleteDefinition` 会物理删除定义的全部版本，而旧 `IsDefinitionInUse` 只检查 `agent_runs`，并可能吞掉查询错误。R1 的 `agent_run_nodes` 与新增 `agent_novel_creation_sessions` 均以 `(tenant_id,definition_version_id)` 外键引用不可变定义版本。对已有小说引导历史的 `novel_architect` 执行删除时，前置检查误判为无引用，最终由 PostgreSQL 外键拒绝并冒泡为 HTTP 500。按工程记忆规约检索活跃笔记，已有 Agent 配置集成笔记涉及冻结版本，但没有覆盖三表删除边界。

## Decision

保持现有 API、端口、版本 CAS 及迁移不变；`IsDefinitionInUse` 对本租户全部历史定义版本使用一条 `EXISTS` SQL 检查运行、节点、小说创建引导三张直接引用表，任何查询错误原样返回，不作“无引用”推断。应用层保留事务外预检，并在同一删除事务内、删除版本前再次调用检查；发现引用统一返回既有 `agent.resource_in_use`，无引用仍调用原仓储 CAS 删除。

## Alternatives considered

只检查 `agent_runs` 并依赖数据库外键：复现当前 500，不能向管理员传递业务冲突，因此不采用。只在事务外扩充三表检查：无法覆盖两次操作之间新增引用，不采用。级联删除引导与节点：破坏历史记录且触碰用户数据，不采用。本次未提升事务隔离级别或引入新锁协议，以避免扩大治理写入范围。

## Consequences

已存在任一直接引用的定义不可物理删除，查询故障将失败关闭；无引用定义沿原 CAS 语义删除。事务内复验缩短竞态窗口但不是串行化保证：复验后若并发提交新的外键引用，PostgreSQL 仍可能拒绝删除，这一极窄竞态未转换为业务 409，需独立真实并发验收及后续事务/锁策略裁决。所有历史引导 session 保留，不清理、不更改用户记录。

## Verification

只读 `inklore_r1` PostgreSQL 查询：`novel_architect` ID `d3f2b143-29f5-4f14-a7a6-22bb9e4eb1e9` 的 versions=1、runs=0、nodes=0、guides=1；与实现相同的三表 `EXISTS` 返回 `in_use=true`。`lsp-references` 已核查 `IsDefinitionInUse` 与应用/仓储 `DeleteDefinition` 调用方。两处源码 `lsp-diagnostics` 均为 0 错误/警告；`go build ./...`、`go vet ./internal/agent/application ./internal/agent/infrastructure`、`gofmt -d` 均退出 0。没有发出写 SQL、删除请求或新增测试文件；未对无引用真实记录执行删除，不宣称 HTTP 409、事务复验竞态或真实无引用 CAS 已端到端验收。
# Agent Note: 用户裁决的500KiB源码上限

Status: implemented

Task: INK-251
Scope: AGENTS、开发skill、cmd/archguard与ADR-0037

## Problem

原50KiB源码门禁报告17处违规；用户明确要求最大文件500k。当前最大生成源码157276字节。生成契约分片不再是用户选择。

## Decision

按既有二进制单位使用500*1024=512000字节，等值允许、超值拒绝。新ADR0037仅替代旧源码大小条款，既有扫描范围、ignore及其他规则不变；不动生成器、冻结契约或独立Markdown长度。PR9手写组件拆分保留，不回滚。

## Alternatives considered

- 原阈值加生成契约分片：用户明确选择调整限值，不实施。
- 移除限制或忽略生成文件：超出用户要求，拒绝。
- 全局替换所有50KB历史与文档限制：会篡改历史事实或扩大冻结治理范围，拒绝。

## Consequences

新500KiB规则允许现有源码，但不证明原50KiB通过。业务调用链、Go依赖、运行装配均不变，只有开发/CI护栏阈值变化。历史Accepted ADR/归档不改写。

## Verification

2026-11-01 dot本地真实go build -buildvcs=false ./...退出0；GOFLAGS=-buildvcs=false go run ./cmd/archguard -reachability退出0，R1–R5通过并输出可达性报告。第一次仅对go run传-buildvcs=false时子go list报告失败，因此改为明确环境GOFLAGS让子命令同样适用快照VCS设置，最终日志已覆盖为成功结果，不把第一次报告失败当成功。

使用同一构建出的archguard CLI在仓库外临时模块验证：512000字节.ts夹具退出0，512001字节退出1；未创建或修改测试源，夹具不入库。本地日志/workspace/shared/inklore-ink251-evidence/。未执行受阻独立复核、未做业务验收、不合并；保持review。

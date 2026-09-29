# Agent Note: LLM R1 catalog boundary and fail-closed handoff

Status: implemented

Task: INK-167
Scope: internal/llm/domain/r1, application/r1, infrastructure/r1, migrations/r1/000005_llm.sql

## Problem

仓库已有legacy LLM provider/model和task policy实现，但其int64模型标识、覆盖式policy、尽力usage和内存限流不满足R1 tenant/CAS、不可变版本、SecretRef和Redis fail-closed契约。把legacy handler直接接到R1路由会让旧语义伪装成冻结实现。

## Decision

先落地一个可审计的R1 catalog边界：domain只定义typed输入/输出、SecretRef、Repository和Accounting Port；application执行严格schema输入、tenant授权、CAS、catalog advisory lock、policy不可变版本、pricing insert-only和引用保护；infrastructure只访问llm私有表，并通过deployment supplied SecretBinding/allowlist和provider-bound HMAC fingerprint处理密钥元数据。精确Cost使用big.Int中间值和独立ceil。R1 migration 000005_llm.sql与冻结data/llm.sql字节一致。

该切片不挂legacy入口，也不把编译结果当成业务完成。HTTP transport已由同域切片补齐并由app装配；Budget五文件和Redis admission四文件已补齐Port/PG/Lua边界；Budget GET/PUT已接入App并通过真实PG/API基础验收，Redis recovery/Lua Acquire/Renew/Report及client关闭后的fail-closed已通过真实Redis验收，Policy Catalog outbox kind在App adapter规范为task_policies。尚无worker生产装配、PG RecoveryProbe/KeyHealth adapter、10秒联合续租或真实provider driver证据；100个唯一 invocation 的并发 Acquire/Report及1:3权重SWRR（40次10:30）已通过，完整Gateway闭环仍必须继续沿冻结契约实现。

## Alternatives considered

直接复用legacy handler会暴露int64旧模型ID和非CAS写；把secret明文放进llm表或日志会突破SecretRef边界；Redis不可用时退回内存限流会违反容量安全；这些方案均未采用。

## Consequences

000005迁移进入R1启动验证，catalog与transport切片已由server真实挂载32个冻结管理路由，其中30个Catalog/pricing/usage操作可达。Budget由fixture显式Provision后真实GET/PUT/CAS可达；Redis recovery/Lua/fail-closed已有真实Redis证据，Budget reserve/dispatch/settle/recovery和完整Gateway仍有未验收路径，严格验收记录4个GAP，不能以路由挂载或部分HTTP成功宣称INK-167完成。

## Verification

- go build ./internal/llm/...：退出0。
- go build ./...：退出0。
- go run ./cmd/migrate -config configs/r1-acceptance-not-present.yaml -cmd up/status：退出0；000005_llm已应用。
- node scripts/check-r1-platform.mjs：通过，5份R1 DDL一致，97错误映射一致。
- go run ./cmd/acceptance/ink167 budget_gateway：严格缺口语义退出1；显式Provision后Budget GET/PUT/CAS、Creator403及Redis recovery/Lua/fail-closed通过，跨日/rollback/replay与production Gateway仍缺证据，总4 GAP。
- 未宣称Budget跨日/rollback/replay、production Gateway、driver、浏览器或完整HTTP行为证据；Redis Lua本轮已有真实Redis证据，不能外推为完整Gateway完成。

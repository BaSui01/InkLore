> 历史记录：本文件保留迁移前的设计或实施证据，不是当前工程规范，也不证明当前功能已验收。当前规则见 [AGENTS](../../../../AGENTS.md)；实现以源码和本轮验证为准。

# 13 · 部署、观测与恢复

R1冻结。运行时同单体仓库，PG权威，Redis限流；详细生命周期见02/21。本文件替代旧release-and-rollback与运维蓝图，不表示本轮已经部署。

## 1. 环境与配置

开发：creator3000/admin3001/API8080、strictPort，代理/v1；生产：两个独立静态站点与单API上游，HTTPS，Origin精确白名单，禁止任意*携credential。仅PG/Redis在私网，/metrics不公开站点代理。镜像只含配置模板，真实配置只读挂载；DATABASE_DSN、REDIS_URL、JWT_SIGNING_KEY、CSRF_KEY、IDEMPOTENCY_HASH_KEY、CURSOR_SIGNING_KEY及Provider凭据由env/Secret Store注入。

配置启动校验：PG连接、JWT/HMAC密钥至少32随机字节、Origin完整、worker并发默认8范围1..64、文件上限不超R1hard cap、lease30s>heartbeat10s、平台时区固定Asia/Shanghai。缺PG或签名拒绝启动，不允许内存业务档位；缺Provider可启动非AI业务，生成返回llm.provider_not_configured；无Secret引用授权表则MCP/Key配置不可新增任意引用。

热更新只通过版本化LLM/Agent配置API，已运行任务持有原快照；进程配置/密钥更新走部署滚动重启，不承诺全部配置任意热切。配置页面分别显示stored_revision与新运行effective_revision。

## 2. 探针与降级

/healthz只进程存活/版本；/readyz检查PG与关键启动初始化，不依赖模型上游或Redis可用性。PG故障503；Redis故障ready200 state=degraded且所有新AI admission503，非AICRUD继续。/v1/platform/status鉴权返回安全依赖、queue、outbox汇总；不能暴露DSN/环境/完整错误栈。

Redis限流租约ttl30s、worker每10s续期；Redis恢复不能重复发放已有持久budget reservation。PG是用量/预算权威，Redis计数只是容量控制。模型熔断降级遵循09，既有输出后不得透明换模型拼接正文。

## 3. 停机顺序

SIGTERM后ready=false→停止claim新job→通知SSE注释server_draining（非run终态）→最多20秒等待已开始事务与event flush→释放可安全交接lease/保持inflight receipt→HTTP最多30秒shutdown→关闭Redis/DB。SSE连接关闭客户端按21重订阅，新实例从PG读同run；不把停机事件假装run成功。强杀场景租约恢复规则与正常关闭相同，不要求依赖goroutine内存完成任务。

## 4. 日志、指标、追踪

结构日志字段timestamp UTC、level、domain、operation、request_id、trace_id、duration_ms、error_code。用户/租户只允许受控匿名hash用于日志查询，不作为metrics标签。禁正文、系统提示全文、APIKey、Authorization、Cookie、一次性票据、工具arguments全量输出；结构对象递归脱敏与最长字符串截断，由白名单序列化优先。

指标：HTTP rate/error/latency；PG pool/inflight/slow查询；jobs queue_wait/lease_lost/retries/dead_letters；SSE active/replay/cursor_expired；LLM tokens/cost/TTFT/error/capacity/breaker；files bytes/rejections。label限domain/operation/status/model_id/channel_id（model/channel数量受注册表上限控制）。traceparent经HTTP→job入队meta→worker新span接续，request_id独立UUID；不将正文当span attribute。

报警阈值（持续时间）：PG不可用30s=critical；5xx>2%且请求≥100/5min=high；queue oldest>120s/5min=high；lease loss>5/5min=high；outbox dead_letter>0/5min=high；剩余PG磁盘<20%/10min=high；budget reservation unknown>0/10min=需人工核查。无样本不是100%成功。

## 5. 保留与清理

| 数据 | 保留/清理 | 失败处理 |
| --- | --- | --- |
| 可恢复业务资源 | deleted_at起30天 | cleanup作业按scope先禁读，再Owner清payload |
| 账本事件证明 | 保留UUID/机器key/摘要证明，不含正文/理由PII | payload分表擦除；不能保留清除文本副本 |
| SSE事件 | 7天 | 过期返回410，从artifact获取终态结果 |
| 幂等记录 | 24小时 | 清理过期完成记录；执行中先恢复不强删 |
| 导出文件 | 7天 | 下载过期404，作者可重新导出 |
| 未绑定上传 | 24小时 | 引用检查后清理，已绑定来源不误删 |
| 原始运行输入/产物 | 最多30天或资源purge时更早 | 安全审计只保留脱敏动作 |
| 审计 | 90天 | 到期按tenant批处理；备份最长同保留上限 |
| 浏览器恢复副本 | 7天/注销立即清除 | 配额失败持续提示，禁止跨账号可见 |

清理与备份恢复必须维护tombstone清单；恢复旧备份后先重新应用删除清单再开放读，防止已删除正文复活。证明中不能包含可恢复的原始文本编码。

## 6. 备份与发布

PG每日加密全备+持续WAL归档至部署侧备份介质，目标RPO≤5分钟/RTO≤30分钟；R1文件bytea与元数据同快照，无另一个存储一致性问题。至少每月隔离恢复一次，验收tenant隔离、章hash、版本数、账本heads重建、queue lease过期可恢复。备份密钥与数据库凭据分离，备份不是业务可直接浏览的导出。

新版本镜像不可变tag+digest，迁移expand→兼容代码→回填/索引→切换→后续contract；重构R1采用新库导入与整体切换，不自动覆盖旧库。回滚需核对schema兼容和新增数据去向，不在回滚时执行破坏性down。失败版本记录digest/迁移版本/触发原因，保留上次可用镜像。上线前检查两静态bundle与API同契约hash。

## 7. 运维验收

OPS-01 PG停机ready503且无内存降级；OPS-02 Redis停机AI503而非AI可用；OPS-03 SIGTERM无新claim，重连重放不重复执行；OPS-04 kill worker导致未知外呼结果不重发；OPS-05审计/日志合成凭据与正文零泄漏；OPS-06备份恢复后删除清单生效；OPS-07两个app端口/产物隔离；OPS-08按阈值触发报警并记录恢复时间。上述是实施验收，不是本轮运行结论。

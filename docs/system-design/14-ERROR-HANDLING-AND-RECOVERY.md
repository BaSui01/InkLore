# 14 · 错误模型、传播与恢复

R1冻结，替代旧源码错误码审计和兼容补丁。机读错误唯一入口是[错误登记](api/error-catalog.json)，由各operation.x-errors与公共规则生成；每条具有code、HTTP、恢复动作与使用操作。普通用户文案由message_key映射，不展示实现错误字符串。

## 1. 类型与映射

DomainError{code,kind,fields,contextSafe,cause}不携HTTP；application包装保留cause与可恢复语义；transport按kind映射400/401/403/404/405/409/413/416/422/429/500/502/503/504，error code稳定。未知cause变platform.internal_error且500，日志仅安全诊断。panic恢复未发头时JSON500，已建SSE时不得再写JSON，由订阅断连与持久run状态恢复。

错误封包由common.json Error约束：code、message_key、details、request_id、trace_id均存在，details空也传{}；message_key=errors.+code；不返回堆栈、SQL、DSN、正文或凭据。field_errors字段JSONPointer，reason机器key；冲突提供current_revision，限流提供retry_after_seconds及Retry-After。跨租户/owner不可见404，与随机不存在一致，不暴露另一个tenant ID。

## 2. 通用错误

| code | HTTP | 场景 | 恢复 |
| --- | --- | --- | --- |
| platform.validation_failed | 400 | JSON、未知字段、必填前置、UUID错误 | 保留输入，定位字段 |
| auth.unauthenticated | 401 | JWT/session失效 | 单飞刷新一次；失败登录覆盖层 |
| auth.forbidden | 403 | 明确权限不满足 | 只读/退出，不重试 |
| platform.resource_not_found | 404 | 不存在或不可见 | 返回列表，不暴露归属 |
| platform.method_not_allowed | 405 | 已知路径不支持方法，带Allow | 按API选择允许方法 |
| platform.audit_export_limit_exceeded | 422 | 导出超过31天或10000条 | 缩小时间窗或筛选条件 |
| platform.version_conflict | 409 | 平台实体CAS失败 | 读最新，人工合并 |
| platform.idempotency_conflict | 409 | 同键不同载荷 | 阻止自动换键，检查逻辑操作 |
| platform.operation_in_progress | 409 | 同键命令未完成 | 1秒后同键查/重放，不并发新操作 |
| platform.cursor_invalid | 400 | 过期/跨scope分页游标 | 从首屏重新查询，保留筛选 |
| platform.request_too_large | 413 | 请求超过上限 | 告知上限，缩小输入 |
| platform.internal_error | 500 | 未分类错误 | 保留输入/关联ID；写请求先查结果 |
| platform.dependency_unavailable | 503 | PG等必要依赖失败 | 保留缓冲，明确稍后恢复 |
| agent.event_cursor_expired | 410 | 7天SSE窗口外 | GET run/artifact，不新建运行 |
| agent.event_cursor_invalid | 400 | cursor不合法或超未来 | 清本地游标并按run详情核对 |
| agent.execution_outcome_unknown | 409 | 外呼已发送但结果未知 | 标未知、人工核对，不自动再次计费 |

领域版本冲突统一<domain>.version_conflict，字段/约束错误由领域x-errors声明；不能复用409表示所有错误。HTTP200但工具success=false由ToolResult转换为失败卡片，不能靠HTTP成功弹“执行成功”。质检有风险不是工具失败，更不是合规保证。

## 3. 重试矩阵

| 请求类型 | 自动重试 | 保留内容/下一步 |
| --- | --- | --- |
| GET只读 | 网络/503最多2次1s/2s，遵守Retry-After | 保留旧数据+stale，不归零 |
| 已有幂等键的本地写 | 网络丢响应可同键重放，禁止重新分配key | 相同body/expected_revision；查询权威结果 |
| 409版本冲突 | 不自动写 | 本地/基线/远端三方对照，用户合并后新逻辑命令 |
| 创建Agent run | 响应丢失同键重放得到原run | 不能换key触发第二次生成 |
| SSE GET重连 | 按21，最多8次，仅回放 | partial片段+最后seq，失败仍可查询终态 |
| Gateway上游已输出/未知结果 | 不自动切模型重发 | partial/unknown，成本保留待对账 |
| 审批决定/取消 | 同键重放；过期或终态先GET | 不新开run，不覆盖已终态 |
| refresh令牌 | 不重复使用旧refresh | 轮换不确定时重新登录，安全优先 |
| 文件下载 | 只读可重试/Range，重新授权 | hash核验，错误JSON不保存成文件 |

retryable只表明再次尝试可能有意义，不授予自动重复副作用。客户端AbortError是本地订阅/请求终止，不直接表示后台cancelled。

## 4. UI呈现与状态

字段错误显示字段旁并焦点首错；资源读取失败显示区域ErrorState+重试；后台刷新失败保留上次可信数据和时间；写失败显示持续保存状态并保留缓冲；全局认证失效用覆盖层，不丢编辑器；计费/权限/冲突用明确解释和动作，不用瞬时toast隐藏。

一次操作只一个主错误提示，底层ApiClient不自发toast；domain控制器选择ErrorPresenter scope。可复制安全诊断仅code/request_id/trace_id/时间/资源ID，默认不含正文。未知code用通用翻译兜底且保留诊断，旧客户端不崩溃。禁网络失败时展示空列表或成功默认值。

## 5. 关键补偿

保存快照失败同事务回滚；确认任一域失败不留半账；导入预览不落正文、提交部分成功逐项报告；导出失败清理临时blob而不删除源；LLM预算预留未使用释放，实际未知不武断退费；outbox失败只重投投影，不重新执行核心写；审批拒绝释放未花额度，已发生调用费用不抹除。

## 6. 验收

ERR-01每个x-errors码有zh-CN/en-US词条；ERR-02错误schema与HTTP一致；ERR-03保存响应丢失同键一版本；ERR-04冲突保留dirty；ERR-05断SSE不第二次POST；ERR-06error终态不被done覆盖（R1无done）；ERR-07工具业务false有失败卡；ERR-08跨tenant404无差异；ERR-09Unknown cause/panic日志与响应脱敏；ERR-10导入只重试未完成项；ERR-11refresh重放吊销；ERR-12依赖失败不生成假内容。详FR及页面验收，不把文档示例当已执行证据。

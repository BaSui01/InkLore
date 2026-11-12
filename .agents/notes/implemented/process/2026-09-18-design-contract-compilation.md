# Agent Note: R1设计收敛使用契约编译而非并列方案

Status: implemented

Task: INK-141
Scope: docs/system-design设计资产、旧文档清理与离线契约检查，不包含业务功能实现

## Problem

上一轮把源码现状和补丁提案补充到旧蓝图，留下字符串/数字错误码、GET/POST流、领域Owner与UI字段等多套口径。用户随后明确要求删除旧文档并先完成详细整体重构设计；继续追加分册不能解决后续开发猜契约的问题。

## Decision

全局选择记录在[ADR-0016](../../reference/decisions/0016-reconstruction-design-freeze.md)，治理位置按[ADR-0017](../../reference/decisions/0017-preserve-decision-records-and-single-design-entry.md)。七域分片是API源，编译器合并同一路径不同方法并把所有Schema引用重定位为外部ref；不flatten各域同名Report/Budget。生成入口、错误目录和FR映射只由源生成。DDL是独立真实SQL；前端按实际Schema校齐字段和命令边界。

30个旧设计/旧API/重复DDL文件已按manifest实际删除，保留ADR原文和活动工具任务。历史ADR-0010指向的旧评估仅留退役说明页，避免修改已接受正文或恢复旧设计。正文/metadata双命令保存、单阶段审批、job/node双fence等集成冲突在设计阶段明确后再冻结。

## Alternatives considered

保留旧API兼容能减迁移初期代码，但用户要求整体重构且会延续两套协议，未采用。把所有Schema平铺到单文件方便下载，却会重名覆盖并超过50KB，改为生成path shards和保留源引用。只做链接检查无法发现审批枚举/租约语义冲突，因此额外做跨域人工审阅；没有把静态工具宣称为业务正确性证明。

## Consequences

未来修改必须回源分片再编译，不能直接改generated目录。冻结hash用于漂移检测，不是历史防篡改封印，也不取代notes自身归档工具。实施依据R1；旧任务done不抵扣新接口、事务与权限的真实验收。仍需隔离PG执行DDL、真实模型协议故障及浏览器交互验收。

## Verification

node docs/system-design/tools/build-contracts.cjs退出0：152路径、233操作、95错误码。node docs/system-design/tools/check-design.cjs退出0：7593本地refs、420文档链接、74表Owner引用、无重复operationId/缺幂等/CAS等结构问题。node scripts/check-project-skills.mjs退出0。go build ./...退出0。清理manifest的30路径均不存在。最终报告以[design-validation.json](../../reference/system-design/implementation/design-validation.json)为准；未执行设计SQL或业务重构，不宣称运行功能通过。

# ADR-0017：设计单入口与工程记录原位保留

状态：Accepted（文档治理，未实现产品）。日期2026-09-18。任务INK-141。部分替代ADR-0016第1条及“替代范围”中的历史目录/执行记录位置，其余R1架构与协议决策不变。

## 背景

用户要求删除旧文档并按system-design完整重构。设计汇编过程中新增的项目决策技能明确复用既有docs/decisions，禁止平行决策库。保留必要因果与任务记录不应保留两套施工方案。

## 决策

- 唯一目标设计仍为docs/system-design；旧archive、docs/api、重复DDL与CONTRACT分册、旧roadmap/release方案在替代后实际删除。
- ADR原位保留docs/decisions，不搬迁至system-design/decisions或history，不修改已接受正文；索引标明被0016替代的具体范围。
- docs/backlog是唯一任务状态记录；system-design/18只定义冻结实施顺序。现状记录docs/architecture明确R1尚未实现，仅记录证据/差距；docs/project-management仅引用新12分册，不复制旧规约。
- 本轮新建但未形成并行历史的0016已落回docs/decisions。其旧history位置描述由本ADR显式替代，避免改写已接受正文。

## 权衡

全删ADR会丢失为什么选择/放弃的信息；另建history树会形成重复所有权；原位保留因果与移除旧设计兼顾可追溯和单一施工入口。保留薄治理入口可使刚引入的Skills继续工作，所有规则指向R1而非旧文档。

## 验证

静态链接与Skills检查必须通过；不存在docs/api与docs/archive、旧DDL/CONTRACT；仅一个docs/decisions；00索引无旧设计引用。没有部署/代码运行变更。

# 接口与数据资产

本目录保留生成器和工具实际消费的机器资产，路径保持兼容，不再承载冻结阶段或固定实施顺序。

- [OpenAPI](api/openapi.json) 与 [接口说明](api/README.md)：接口定义和生成视图。
- [数据说明](data/README.md)：聚合 DDL 与版本化迁移的关系。
- [契约构建工具](tools/build-contracts.cjs)：编译接口分片、错误目录和导航。
- [语义检查工具](tools/check-design.cjs)：检查引用、操作身份、权限、幂等、CAS 和表关系，不冻结设计内容。
- [当前架构](../architecture.md)：实际装配与运行限制。

源码、生成契约和历史设计仍存在差距。检查失败时报告具体差异，不能将 OpenAPI 存在或静态检查通过等同于业务可用。

旧分册、实施计划、冻结清单和检查报告位于 [历史设计资料](../../.agents/notes/reference/system-design/README.md)，用于追溯，不作为当前工程约束。

# 历史导入工具的报告路径兼容

历史验收报告已迁到 [工程参考资料](../../../.agents/notes/reference/system-design/implementation/)。它们不再充当当前设计检查结果或开发门禁。

保留此目录是因为 [离线数据导入工具](../../../cmd/migrate/r1migration/main.go) 的默认报告路径仍为 docs/system-design/implementation/r1-migration-report.json。该名称属于旧迁移工具的输出兼容约定，不是当前产品版本。调用者可用 --report 指定独立输出位置；报告应关联实际执行环境与源码，不能以样本演练当真实迁移完成。

本轮未运行该导入工具，也未改变其数据映射、标志或输出格式。当前契约检查器直接输出结果到标准输出，不依赖本目录的历史报告。

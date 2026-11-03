# InkLore 接口契约

[openapi.json](openapi.json) 汇总领域 API 分片；[错误目录](error-catalog.json) 与 [生成路径](generated/) 是工具消费的资产。
修改时核对真实路由、处理器、前端 SDK 与生成代码，并保留对外兼容性。

- 生成与验证命令见 [开发指南](../../development-guide.md)。
- 历史版本字段保留原格式，不代表项目开发阶段。
- 文档接口不等于已经注册或接线，当前差距见 [架构说明](../../architecture.md)。
- 语义检查失败必须显式报告，不得通过删除权限、幂等、CAS 或错误映射让检查变绿。

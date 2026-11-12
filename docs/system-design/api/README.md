# InkLore API目录

唯一入口：[openapi.json](openapi.json)。源分片定义操作与Schema；generated/是确定性编译视图，不手改。接口兼容、错误、权限与事务见[当前工程规则](../../../AGENTS.md)；源码接线与运行证据需单独核对。

| 分组 | 操作数 | 明细 |
| --- | ---: | --- |
| agents | 41 | [agents](catalog/agents.md) |
| auth | 13 | [auth](catalog/auth.md) |
| compliance | 21 | [compliance](catalog/compliance.md) |
| content | 38 | [content](catalog/content.md) |
| inspiration | 29 | [inspiration](catalog/inspiration.md) |
| llm | 33 | [llm](catalog/llm.md) |
| outline | 29 | [outline](catalog/outline.md) |
| platform | 17 | [platform](catalog/platform.md) |
| worldview | 25 | [worldview](catalog/worldview.md) |

错误登记：[error-catalog.json](error-catalog.json)；实时Schema：[events.json](events.json)。

生成命令：node docs/system-design/tools/build-contracts.cjs。检查命令：node docs/system-design/tools/check-design.cjs。

> 历史记录：本文件保留迁移前的设计或实施证据，不是当前工程规范，也不证明当前功能已验收。当前规则见 [AGENTS](../../../../AGENTS.md)；实现以源码和本轮验证为准。

# 02T · 技术栈与命名冻结

R1版本基线，依赖锁定在实施阶段写入go.mod/go.sum与前端lockfile；文档列主版本/最低patch，不绕过仓库安全更新。架构选择依据[ADR-0016](../decisions/0016-reconstruction-design-freeze.md)。

| 层 | 固定选择 | 用途与边界 |
| --- | --- | --- |
| 后端 | Go1.25+、Gin1.12、GORM1.31、pgx5 | 单体HTTP/Worker；Gin只transport，GORM只infra/platform |
| 权威存储 | PostgreSQL16、pgvector、pg_trgm | 业务/任务/文件同库；向量1536维；不引Mongo/S3/Redis队列 |
| 缓存限流 | Redis7、go-redis9 | 配额、并发租约和可失效缓存；不替代权威写 |
| 配置 | yaml.v3、环境变量 | configs/config.yaml+环境覆盖；SecretRef解析来自部署允许表 |
| 认证 | JWT HS256、bcrypt cost12 | 签名环境注入；会话PG撤销，15分钟access/30天refresh |
| schema | santhosh-tekuri/jsonschema/v6 | Draft2020-12完整校验；禁止网络$ref |
| WebSocket | coder/websocket | 低频安全通知，不承载正文delta |
| 前端 | React19、TS5.8 strict、Vite7 | apps/creator与apps/admin独立入口、构建、端口 |
| UI | antd6、Lucide SVG、CSS变量 | 减少自造基础控件，无Emoji |
| 路由/缓存/表单 | ReactRouter7、TanStackQuery5、Ajv8+formats3 | URL状态、服务器缓存、JSONschema；不承接领域业务规则 |
| 正文编辑 | 原生textarea+领域Controller | 纯文本规范；不引入富文本第二数据模型 |
| Markdown | @ant-design/x-markdown2 | 仅建议/报告展示；禁原始HTML与脚本URL |
| 运维 | Nginx、Docker、Prometheus、OTel | 可观测性与部署；不把运维菜单混入creator |

## 命名

- Go包小写，文件snake_case；类型PascalCase，接口按能力命名如ChapterReader/FactAppender，不用I前缀。domain聚合不带ORM/JSON标签，HTTPDTO独立。
- DB表domain_plural，字段snake_case；主键UUIDv4，内部事件序号seq是int64不是实体ID。索引idx_<table>_<purpose>、唯一uq_<table>_<purpose>、外键fk_<table>_<ref>；SQL文件Owner由前缀与data/目录核验。
- API/v1/<domain>，agents为agent公开前缀；path parameter snake_case，query limit/cursor，JSON snake_case；时间RFC3339 Z，金额十进制定点字符串或integer微单位，禁止JS浮点直接结算。
- operationId全局唯一，domain动作资源，如contentRollbackChapter。x-use-case带FR-XX，x-errors带code/http_status/recovery，x-permission固定audience/roles/scope结构。
- 错误domain.reason，翻译errors.domain.reason；字段错误reason是稳定机器词条。旧数字400xx和ERR_*不进入R1。
- UI组件PascalCase，hooks useX；所有界面文案t(key)，代码状态枚举不翻译；展示名称统一翻译。API属性不以中英混合标题命名。
- 日志snake_case键，level、timestamp、request_id、trace_id、domain、operation、duration_ms、error_code；业务正文/工具全参数不得入日志。metrics使用inklore_<subsystem>_<name>_<unit>，标签限domain/model/channel/status等受控低基数，禁止novel/user/run ID作label。

## 依赖准入

已有库如已满足R1用例先复用，经Adapter统一输入输出；新增库限上表。供应商协议均在llm/infrastructure下适配，Agent不能散落SDK。生成类型由api生成，业务实体手写，生成文件标明来源hash；不得导入另一个前端app以共享页面。升级patch不改变接口需要重新构建；升级主版本、存储或传输语义必须R1.x记录。

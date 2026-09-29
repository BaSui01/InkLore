# 12 · 设计冻结与工程治理

R1是唯一施工基线，入口[00](00-INDEX.md)，全局契约[baseline](00-CONTRACT-BASELINE.md)，决策[ADR-0016](../decisions/0016-r1-reconstruction-design-freeze.md)。现有代码与目标不同是重构工作，不通过改设计把缺陷变合理。

## 1. 不变量

模块化单体七域；各域transport→application→domain←infrastructure。跨域消费方Port与app/adapters；禁止跨私表/跨infra。Provider密钥只env/Secret Store；接口只安全metadata。SSE请求级生成，WS系统通知。PG UTC存储、北京业务日。单手写文件<50000字节。无UnicodeEmoji、无静态业务数据兜底、无模拟进度。前端两独立app/3000与3001，所有文案i18n。

禁止新增/修改 *_test.go 与 *.test.ts；验收用可执行命令、受控HTTP/SQL探针和浏览器步骤记录。禁止用这项约束当不验证的理由。设计资产的静态校验器是tools/check-design.cjs，不是业务测试文件。

## 2. 权威顺序与资产责任

用户明确要求→AGENTS工程红线→本轮有效ADR-0016→00-CONTRACT-BASELINE→api/data机读字段→领域用例/UX/复用→18实施记录。同基线不能留相互冲突；任何发现必须在合并前同时修正所有相关资产。history仅追溯，不覆盖现行设计；旧architecture/backlog不再是入口。

接口只在api/*.json定义字段；DDL只在data/*.sql定义物理结构；14错误总表从operation扩展生成；21唯一实时事件协议；20唯一组件/状态复用契约。不得再创建“修正版API”“临时错误表”“第二套规范”。生成入口openapi.json和目录README标记由tools/build-contracts.cjs重建。

## 3. R1冻结与变更控制

freeze-manifest.json记录设计文件SHA256，排除自身和运行验收记录。冻结意味着实施前关键选择已经确定，不意味着以后发现缺陷也不能修。R1.x变更必须：提出原因与旧条款→列影响FR/operationId/表/组件→写增量ADR→更新机读Schema及迁移→说明前后兼容或切换窗口→重新验收/更新hash。开发者不得只改代码绕过条件写、幂等、角色、状态或时间口径。

新需求如TTS/图片/协同/支付属于R2，不能塞进R1模糊责任。修复引用/拼写不改变语义可作为文档patch，但仍运行静态检查。实现证据只追加到18/implementation，不污染冻结正文为一份不断变化的现状审计。

## 4. 开发任务与完成定义

任务只在docs/backlog登记，18定义冻结实施顺序；编号INK-三位且永不复用，1..140历史不抵扣R1验收，141为本轮设计，142为独立Skills任务，R1实施从160开始。状态todo→doing→review→done，外部阻塞写具体条件。依赖done后实施；一个任务一个可独立验收结果，跨域应用变更拆子任务并在总集成验收汇合。

完成必须具备：go build ./...；约定架构守卫；对应前端app构建；schema/DDL静态检查；按FR运行可复现正反例；SQL/API/浏览器证据；运行路径影响；无凭据/Emoji/测试文件/超限文件；本轮无部署或迁移则明确写未执行。已有旧代码通过build不能证明R1契约已实现。

## 5. 评审检查点

- 数据Owner与调用方向清晰？事务是否涵盖版本/投影/幂等？是否错误地跨私表？
- 命令能否安全重放？不同载荷同键是否409？上游结果未知是否避免再计费？
- 租户/主体/资源权限是否在缓存、下载、SSE与重试都检查？
- 每个失败对用户保留什么、如何恢复？Schema不能说明的业务约束是否写了应用层规则？
- 页面是否有真实接口、typed数据、异常/空/过期状态、键盘路径？共享组件是否反向引用业务？
- 关键状态是否可从持久化重建？终态是否CAS不可逆？没有真实数据时是否诚实不可用？

## 6. 文档清理

旧docs/archive、旧docs/api、重复DDL/CONTRACT分册及旧路线图/发布方案被R1资产替代后删除；不是再次归档整套矛盾设计。docs/architecture仅记录实现差距，backlog仅记录任务，project-management仅导航新治理；不保留第二套目标方案。以前ADR原位保留docs/decisions，索引标替代范围，依据ADR-0017。删除清单与替代位置见19，根README只指向新入口；不能留下失效链接要求开发者猜路径。

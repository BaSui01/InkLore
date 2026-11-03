# ADR-0018：R1.1最小契约勘误

- **状态**：Accepted
- **日期**：2026-09-18
- **决策者**：项目用户明确批准“四项最小勘误，继续开发”
- **关联任务**：INK-178、INK-161、INK-162
- **替代关系**：补充ADR-0016的冻结资产，仅替代下述四项不一致；七域边界、依赖DAG、DDL和其余R1语义不变。原ADR不改写。

## 背景（Context）

施工与真实响应验收发现本人改密缺基线要求的幂等头、下载缺已登记416响应、审计导出缺规定422及错误码、未声明方法405缺JSON错误登记。用户先选择保持冻结完成无冲突部分；该部分验收后，本轮明确批准以下最小勘误以解除依赖门禁。

原冻结清单保存为[0018-r1-freeze-before.json](assets/0018-r1-freeze-before.json)，SHA-256为bcf2bd031d725fec486b484a4da61ac0306b7c0a23863c594c915be54280811d。它记录原资产逐文件哈希，保留修改前依据，不将新清单冒充原R1。

## 决策（Decision）

以R1.1勘误统一既有语义与机读定义，批准范围严格如下：

1. authChangePassword显式声明公共IdempotencyKey参数；expected_revision与原请求字段不变。本人改密吊销该用户全部会话，幂等重放依旧先检查当前授权，绝不绕过吊销返回旧结果或缓存令牌。
2. platformDownloadFile补416公共Error响应，沿用platform.range_invalid，不改变Range语义。
3. platformExportAudit补422公共Error响应并登记platform.audit_export_limit_exceeded / 422 / fix_input。超过既有31天或10000条上限整次拒绝、不截断、不提前发送文件头。
4. 已知路径未支持方法返回405标准Error及Allow头，登记platform.method_not_allowed / 405 / fix_input；未知路径仍404。不得将400/404注册码强配405。

批准修改api/platform.json、api/common.json、build-contracts.cjs公共规则、00基线与14错误分册必要说明及生成派生文件。错误码总数95→97。记录原manifest后重新生成并冻结新清单；DDL字节不变。不扩展业务范围、权限、TTL、API路径或业务字段。

## 备选方案（Alternatives）

| 方案 | 优点 | 缺点 | 为什么未采用 |
| --- | --- | --- | --- |
| 继续全部保持原冻结 | 原资产不动 | 完整INK-161与后续依赖持续阻塞 | 用户已明确批准最小修正 |
| 代码单独遵从其中一份文档 | 改动少 | API、错误登记与实际响应漂移 | 无法严格响应验收 |
| 放宽所有认证幂等/复用任意错误码 | 快速消除表面冲突 | 破坏原授权、重试与状态映射 | 超出批准范围 |

## 后果（Consequences）

四处差异获得唯一实施依据，可以完成改密与审计导出并恢复逐任务验收。增加两个错误词条，生成资产与运行映射必须同步。405属于公共HTTP行为，不新增未声明PATCH操作。旧冻结基线通过保存的manifest追溯，后续不可扩大本次勘误范围。

## 验证方式（Verification）

生成器与静态设计检查通过后重新冻结；比较旧清单确保仅批准源及生成派生产物变化，DDL不变；公共错误映射恰97且逐项匹配。真实PG/HTTP验证改密CAS、幂等前置与全会话吊销，审计导出31天/10000条边界和隐私，405封包及Allow。全仓构建、archguard、notes与diff检查。任务完成以backlog与实际验收证据为准，本ADR Accepted只代表决策生效。

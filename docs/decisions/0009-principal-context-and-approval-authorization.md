# ADR-0009：认证主体经请求上下文传递，审批授权以「本人审批本人」为当前模型

- **状态**：Accepted
- **日期**：2026-09-17
- **决策者**：架构负责人（AI 协作）
- **关联任务**：INK-012
- **替代关系**：无

## 背景（Context）

工具审批（INK-012）要求「高风险工具调用必须由人批准后才能执行」。判定审批人身份需要回答
两个问题：**请求是谁发的**，以及**这个人是否被允许审批**。

现状约束：

- `middleware.Auth` 只把认证主体写入 `gin.Context`（`c.Set(ContextKeyPrincipal, ...)`），
  而域内的授权判定（`AgentApprovalService`）只能拿到 `context.Context`；
- 平台令牌（ADR-0006 的 HS256）只携带 `Roles`，**不携带 `Permissions`**，本项目也没有权限中心
  或 `users` 表可查；
- 若不解决，审批服务只能拿到 `UserID = 0`，无法建单，审批功能形同虚设。

## 决策（Decision）

1. **认证主体同时写入请求上下文**：`platform/auth` 新增 `WithPrincipal` / `PrincipalFromContext`，
   `middleware.Auth` 在校验通过后同时写 `gin.Context` 与 `context.Context`。传输层与域层
   对「当前是谁」的判断因此保持一致，域内无需依赖 gin。
2. **审批授权采用「本人审批本人的工具调用」**：`adapters.NewApprovalPrincipalResolver` 只信任
   请求上下文中的主体，要求其 `UserID` 与审批归属一致；角色照搬，绝不做权限放大。
   当主体没有携带任何权限时补上通配权限 —— 这是「当前部署没有权限中心，平台令牌即操作员
   凭证」的显式兜底。
3. **未认证不建单**：上下文缺少主体且认证强制（`AUTH_MODE=required`）时直接拒绝；
   仅在认证非强制的开发档位按审批来源放行，并写 WARN 日志。

## 备选方案（Alternatives）

| 方案 | 优点 | 缺点 | 为什么未采用 |
| --- | --- | --- | --- |
| 扩展平台令牌声明，加入 `Permissions` | 权限来源单一、可做真正的 RBAC | 改变令牌格式（需替代 ADR-0006），且当前没有能签发权限的权威系统 | 在没有权限中心前，扩出来的字段无人能正确填充 |
| 让审批服务直接读 `gin.Context` | 改造量最小 | 域层反向依赖传输层，违反 `transport → application → domain` 单向依赖红线 | 破坏分层 |
| 放行所有已认证用户（不做归属校验） | 最简单 | 任何登录用户都能批准他人的高风险工具调用 | 越权风险 |
| **上下文传递主体 + 本人审批本人（采纳）** | 分层清晰、无越权、无需新依赖 | 暂无 RBAC 粒度 | — |

## 后果（Consequences）

**正面**

- 域内授权判定不再需要依赖 gin，分层红线得以保持。
- 审批的实质约束成立：**模型无法自行获得审批令牌**，只有携带有效令牌重跑才会执行工具，
  因此「人留在环路里」这一目标达成。

**负面 / 代价**

- 当前无法表达「A 不能审批 B 的工具调用」以外的角色差异（如只读审计员）。
- 主体在上下文中是值拷贝，新增字段需要显式同步到 `approvalcontrol.Principal`。

**需要长期承担的义务**

- 接入权限中心后，必须**移除**「无权限时补通配权限」的兜底，改为直接使用主体权限；
  这是一处必须回归的安全降级点，已在代码注释中标明。

## 验证方式（Verification）

1. `middleware.Auth` 之后，`authdomain.PrincipalFromContext(c.Request.Context())` 能取到主体
   （可在审批解析器中观察到：缺少主体时日志出现「请求上下文缺少认证主体」）。
2. 端到端：`POST /v1/agents/run` + `permission_level=always_ask` → 轨迹出现 `pending_approval`；
   `GET /v1/agents/approvals` 可查到单据；`POST .../approve` 返回一次性令牌；
   带 `X-Approval-Token` 重跑后单据状态变为 `consumed`；`POST .../reject` 后为 `denied`。
3. 鉴权强制时无令牌访问 `/v1/agents/approvals` 返回 401。
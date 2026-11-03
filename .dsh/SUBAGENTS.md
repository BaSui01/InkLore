# InkLore DSH并行开发配置

专用preset：inklore-parallel-terra，界面名称InkLore Parallel · GPT-5.6 Terra。初始配置记录沿用 INK-144。

## 启用

在InkLore工作区新建空白对话，选择上述preset，再下达开发任务。DSH在读取preset列表时重新扫描用户目录；若已打开选择器，关闭后重新打开或刷新页面。已开始对话的preset不可切换，本次配置不改变本对话的现有子代理，也不修改其他项目默认模型。

项目配置源：[agent.cordis.yml](presets/inklore-parallel-terra/agent.cordis.yml)与[preset.yml](presets/inklore-parallel-terra/preset.yml)。已安装到本机DSH_HOME/.agent-presets/inklore-parallel-terra。项目目录不是DSH自动发现根；修改源后需把这两个文件同步到用户preset目录。配置没有API密钥，复用本机已注册basui供应商。

## 生效参数

- 所有通用spawn/fork与11个角色工具：provider=basui，model=gpt-5.6-terra，reasoningEffort=high。
- 主代理模型仍由对话模型选择器决定；子代理不继承成另一个模型。
- 连续后台子代理支持send_message继续同一子任务；maxDepth=1禁止子代理再次扩散。
- 调度规则最多4个活跃子代理（包含审阅），由主代理遵守；这是协调规则，不是伪造一个DSH并不存在的maxConcurrency配置。
- fork继承已完成对话，若主模型不同，不保证KV缓存复用。独立领域优先spawn，减少无关上下文。
- 只读审阅工具的允许工具仅read/read_image/glob/grep/send_message，不提供shell或文件写工具。

## 角色与写入边界

| 工具 | 默认Owner | 职责 |
| --- | --- | --- |
| subagent_platform | internal/platform/ | 事务、scope、会话、幂等、队列、文件 |
| subagent_content | internal/content/ | 作品、卷章、版本、确认用例 |
| subagent_outline | internal/outline/ | 大纲、场景、节拍、伏笔 |
| subagent_worldview | internal/worldview/ | 设定、关系、只追加事实 |
| subagent_llm | internal/llm/ | 网关、路由、预算、用量 |
| subagent_inspiration | internal/inspiration/ | 拆书、文风、灵感资产 |
| subagent_compliance | internal/compliance/ | 规则、报告、净化建议 |
| subagent_agent_runtime | internal/agent/ | run、工作流、审批、SSE |
| subagent_creator | frontend/apps/creator/ | 作者工作台 |
| subagent_admin | frontend/apps/admin/ | 管理端 |
| subagent_review | 无，仅只读 | 跨域契约、安全与集成审阅 |

写入Owner是协作约束，不是每域单独的文件系统沙箱。DSH子代理共享工作区；父代理派发时还必须给每项明确的独占文件列表。先检查真实目录，旧代码布局与目标不同不能靠猜测创建重复模块。

以下文件由主代理单独整合：cmd/、internal/app/、迁移序号与公共迁移、go.mod/go.sum、前端workspace和lockfile、frontend共享packages、AGENTS/文档/看板。确需委派公共文件时，先撤销其他写入Owner再给唯一代理；禁止两个代理同时改这些文件。

## 执行顺序

以 [AGENTS](../AGENTS.md)、当前源码与 [GitHub Issues](https://github.com/BaSui01/InkLore/issues) 的真实依赖和验收范围安排工作：

1. 小任务由主代理直接完成；独立且工作量足够的任务才并行派发。
2. 先核实调用链、实际接口和数据依赖，依赖未满足的部分不宣称可独立交付。
3. 明确每个代理的独占文件和验证方式；公共契约、装配与迁移由主代理协调。
4. 收齐实际变更和验证后整合；共享输出目录或数据库的验证串行执行。

主代理派发需包含：用户目标或 Issue 链接、验收、实际依赖、相关源码、允许文件、公共接口、禁止改动、验证命令、输出格式。子代理回报路径/结果/证据/未验证项；不自行提交、部署、改变未授权的公共契约或覆盖其他工作。

启动指令：

> 从当前用户任务和实际源码开始，核对调用链、依赖与验收条件；独立的大任务按文件Owner并行推进，主代理协调公共文件并核对真实验证结果。

校验命令：`node .dsh/verify-preset.mjs`，默认使用本机DSH应用路径，可用第一个参数指定另一安装目录。它调用DSH原生scanRoot与tool-subagent Config，核对13个代理工具、模型登记、只读工具白名单及安装副本一致性。

配置安装和模型登记不等于已进行业务开发或上游模型调用成功；实际模型请求仍取决于供应商可用性与额度。

## 原生会话子代理入口（INK-190）

提供原生 collaboration 工具的会话直接使用 spawn_agent，逐次显式指定 `model=gpt-5.6-terra`、`reasoning_effort=high` 和独立上下文；不依赖 DSH preset，不修改全局模型或既有会话。禁止子代理继续派生。最多四个活跃子代理，实际还受宿主并发槽限制；当前宿主四槽包含主代理，因此最多同时派发三个子代理。

主代理保留公共装配、依赖、生成器、文档和任务管理 Owner；每批公共 Platform 文件仅一个写者。派发准确白名单、实际依赖、公共接口和验收命令。只读审阅在原生工具无权限白名单时是任务约束，不声称工具隔离。子代理不得提交、发布、改变未授权的公共契约或操作共享验收数据库。保留已有差异，最终集成与数据库验收串行执行。

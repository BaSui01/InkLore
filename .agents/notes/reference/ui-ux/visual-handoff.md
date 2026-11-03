> 历史记录：本文件保留迁移前的设计或实施证据，不是当前工程规范，也不证明当前功能已验收。当前规则见 [AGENTS](../../../../AGENTS.md)；实现以源码和本轮验证为准。

# 高保真 UI 图片与交互原型交接单

> INK-259 · 视觉制作任务说明。当前 docs PR 未生成、未上传和未验证图片；本文件用于随后在支持 Product Design 的 Work 模式制作并通过单独 PR 归档选定方案，不把待做事项写成已完成。

## 1. 需要的素材与参考

来源必须是本仓库对应分支的实际页面及其可见状态：[Admin App](../../../../frontend/apps/admin/src/App.tsx)、[Creator App](../../../../frontend/apps/creator/src/App.tsx)、[管理端蓝图](admin-console-plan.md)、[创作者蓝图](creator-studio-plan.md)、[Token 与无障碍](../system-design/17-DESIGN-SYSTEM-AND-ACCESSIBILITY.md) 及 UI/UX 15、16、20 分册。

开始视觉生成前，先在安全测试环境中截取现有 Admin 概览、渠道/Agent、Creator 书架/写作页面作为版式参考；若浏览器/登录环境不可用，记录缺口并明确区分仅凭代码的推测。不得使用包含真实密钥、正文隐私、访问令牌、Cookie 或个人信息的截图。

## 2. 双端视觉设计 Brief

| 维度 | Admin 概览 | Creator 写作台 |
| --- | --- | --- |
| 主要用户 | operator / auditor | 拥有作品 scope 的 creator |
| 主目标 | 判断模型运行与渠道风险，安全进入治理动作 | 连续写作、可靠保存、审阅后采纳 AI 建议 |
| 桌面基准 | 1440×1024 | 1440×1024 |
| 重点结构 | 固定管理导航、清晰范围标题、真实采样信息、健康/用量表 | 作品章节顶栏、目录、纯文本正文、上下文检查器 |
| 主要动作 | 刷新当前观测 | 保存当前章节 |
| 主要反馈 | unknown/stale/partial 与 request_id | saved/dirty/offline/conflict 与版本/恢复 |
| 约束 | 只读内置工具；无正文/凭据；auditor 不得写 | 无管理员入口；AI 不越权保存；原生纯文本正文 |
| 视觉系统 | Admin light/dark 语义 token | Creator light/dark 语义 token |

设计图采用真实且脱敏的示例结构，不虚构渠道健康值、运行费率、作品正文或产品功能。数字若仅用于布局必须明确注明“演示数据，非实时”。不得在 Creator 放图片生成、投稿、TTS、协同编辑、订阅购买等非 R1 入口。

## 3. 后续出图和选稿流程

先就一个确定主页面使用 Product Design 进行三个独立视觉方向探索，分别展示结果供用户选稿；不要提前根据 AI 自行选定方案并改代码。Admin 与 Creator 各自复用此流程，确保选择的是完整页面而非杂糅功能清单。用户选择后再补齐：

1. 选定方向的 1440×1024 light 主稿，并附 dark 同结构稿。
2. 1200/768/480/320 CSS px 响应式关键断点，保留必需主操作与错误。
3. 至少各一组 loading/empty/error/stale 及 409 冲突等关键状态；一组键盘焦点与弹窗/抽屉行为说明。
4. 页面→组件→operationId→权限/状态的标注表，设计图片与当前真实 API/契约逐项核对。
5. 基于选定方向的交互原型与浏览器截图，再单独提交源码 PR；真正运行通过的截图才可作为验收证据。

## 4. 建议后续存档位置（不存在不代表失败）

视觉稿通过安全审查、选定、导出后，建议按如下目录归档，文件名称需要包含端别、页面、主题、尺寸与修订号：

- docs/ui-ux/assets/admin-overview-light-1440-v1.png
- docs/ui-ux/assets/admin-overview-dark-1440-v1.png
- docs/ui-ux/assets/creator-editor-light-1440-v1.png
- docs/ui-ux/assets/creator-editor-dark-1440-v1.png
- docs/ui-ux/assets/admin-overview-mobile-390-v1.png
- docs/ui-ux/assets/creator-editor-mobile-390-v1.png

这些路径现在只是命名约定，**当前 PR 不含任何上述文件**。只提交审核通过的视觉素材，不提交第三方字体文件或敏感截图。

## 5. 视觉审核与完成定义

只有当用户已选稿、四种主题均核对原有 Token、页面功能与 API 一致、没有假按钮或越权入口、移动端无主操作遮挡、键盘/焦点状态完整、图像实际纳入 PR，才可将“视觉稿设计”标记完成。视觉稿完成仍不代表前端实现/运行时验收完成。

推荐下一阶段 Work 任务：基于本 PR docs/ui-ux 四份文档和现有两端真实代码与页面截图，依次为 Admin 概览与 Creator 写作台各生成三个高保真 UI 方向，展示给我选；选择后制作独立交互原型及后续 PR，不改动冻结系统合同。
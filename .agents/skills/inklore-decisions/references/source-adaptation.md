# 来源与 InkLore 融合取舍

本技能包依据用户指定的三份公开资料及仓库现有治理文档编写，采用原创的项目流程说明，没有复制上游脚本、看板或大段模板。

## 查阅来源

- [从 DSH 源码看 DeepSeek Harness 团队是如何维护项目的](https://linux.do/t/topic/2904342)：作者个人分析；采用规则、现状与因果分离，分层阅读和非平凡改动记录的思路，不把文章当官方证明。
- [像 DeepSeek 团队一样维护项目](https://linux.do/t/topic/2874256)：采用为下个会话保留设计意图与放弃方案的思路。
- [write-notes-like-deepseek](https://github.com/czm15053/write-notes-like-deepseek)：查阅 README 与 SKILL；访问时 GitHub 页面显示提交 a6073d3933a919b5407213ab94e8122f17c4210c。原始 main 内容为在线抓取，未做逐文件 commit 哈希校验。
- [上游 SKILL](https://github.com/czm15053/write-notes-like-deepseek/blob/main/SKILL.md)：参考先探测宿主、非平凡判定、备选方案、历史检索及机械检查；不安装上游包或执行远程脚本。

## 具体适配

| 上游方法 | InkLore 采用方式 | 原因 / 代价 |
| --- | --- | --- |
| AGENTS + docs + notes 职责分开 | AGENTS 规则、architecture 现状、system-design 目标、decisions 理由、backlog 执行 | 复用既有事实来源；需持续核对目标与实现差异 |
| 每次按需读资料 | 三个短 SKILL + 文档路由 + 模板 | 避免复制全套 docs；路径迁移需要校验 |
| 非平凡改动留因果 | 架构裁决 ADR；实现因果写 notes 并由验收记录引用 | 防止理由散落在任务日志，纯机械变更免写 |
| 四生命周期 × 六分类 notes 树 | 采用 .agents/notes 独立工程记忆，ADR/backlog 保持原职责 | 用户在 INK-143 明确要求；笔记不重复架构裁决和任务状态 |
| 事实可原地修旧笔记 | 活跃 note 可同步事实；决定翻转另写新 note；Accepted ADR 正文保持不可变 | notes 与正式 ADR 区别管理，归档 note 由封印保护 |
| 测试 + 文档机械门禁 | 构建、archguard、可复现请求/浏览器证据、技能静态校验 | 项目禁止测试文件；编译和静态检查不等于功能通过 |
| 固定确认门 | 查清事实，沿现有授权推进；真正架构不确定才问 | 用户授权和宿主协作协议优先，避免重复批准 |
| 哈希归档 | 本地工具归档 implemented，LF规范化SHA-256及Git基线比较 | 防止无声改写归档；不是数字签名，不冻结活跃笔记或ADR |
| 网页看板 | 不引入 | 目录检索即可，避免给工程记忆额外引入运行服务 |

## 本次流程决定的验证与边界

INK-142 的 [初始交付记录](../../../notes/implemented/process/2026-09-29-skill-delivery-ink-142.md) 保留当时未采用 notes 的历史。用户在 INK-143 明确要求长期工程记忆，本页是当前适配说明；新选择及真实备选记录在 notes，证据见 [工程记忆交付](../../../notes/implemented/process/2026-09-18-notes-delivery-ink-143.md)。工具无第三方依赖，不重写七域或R1契约。技能检查加笔记结构、任务关联、链接、归档哈希和Git基线门禁，仍不能判断架构裁决正确性、备选真实性或自动保证每次非平凡改动都有笔记。

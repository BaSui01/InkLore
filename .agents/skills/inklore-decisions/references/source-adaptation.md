# 来源与当前采用范围

本页保留初始技能的资料来源线索。当前流程由用户要求、AGENTS、真实源码和现行工具确定，不要求每次任务重新阅读这些外部文章。

## 初始资料线索

- [从 DSH 源码看 DeepSeek Harness 团队是如何维护项目的](https://linux.do/t/topic/2904342)
- [像 DeepSeek 团队一样维护项目](https://linux.do/t/topic/2874256)
- [write-notes-like-deepseek](https://github.com/czm15053/write-notes-like-deepseek)
- [上游 SKILL](https://github.com/czm15053/write-notes-like-deepseek/blob/main/SKILL.md)

以上链接保留历史来源，不代表本轮已重新核实外部内容，不安装或执行其中的远程脚本。

## 当前采用方式

| 方法 | 本项目采用范围 |
| --- | --- |
| 规则、现状、因果分开 | AGENTS 保存工程要求，docs 描述当前系统，notes 保存有价值的理由 |
| 按需读取 | 从源码问题进入相关短技能、契约或历史记录，避免全量加载文档 |
| 非显然取舍留证据 | 已有记录能承接就更新，重要替代说明范围；普通局部修复无需 ADR |
| 生命周期笔记 | 沿用既有目录与模板；记录状态不替代 GitHub 任务状态 |
| 历史追溯 | 旧设计/ADR/规划放 reference，不因曾被接受就永久限制当前实现 |
| 验证 | 按风险执行构建、必要测试、真实请求和交互；未验证项明确保留 |
| 授权 | 沿用户已给出的范围推进，高风险未授权事项单独说明 |
| 历史封印 | 保护 archived 笔记及其 Git 基线，不冻结活跃设计与普通编辑 |

初始交付记录：[项目技能](../../../notes/implemented/process/2026-09-29-skill-delivery-ink-142.md)、[工程记忆](../../../notes/implemented/process/2026-09-18-notes-delivery-ink-143.md)。这些记录只证明当时的交付，不替代当前功能验收。

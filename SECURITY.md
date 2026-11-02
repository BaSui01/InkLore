# 安全政策（Security Policy）

InkLore 处于持续开发阶段。请勿将生产凭据、用户数据或未修复漏洞详情提交到公开 Issue、PR 评论或讨论区。

## 报告安全问题

请优先使用仓库 **Security → Report a vulnerability**（若仓库已启用 GitHub 私密漏洞报告）向维护者报告，避免公开披露可利用细节。未启用该功能时，请先通过仓库维护者提供的私人联系渠道联系，不要在公开 Issue 中粘贴密钥、用户信息或完整攻击载荷。

## 凭据处理

- LLM Provider API Key、数据库密码、JWT / 加密密钥只允许从环境变量或 Secret Store 加载，不应写入源码、示例配置、日志或文档。
- `.env`、`configs/config.yaml` 等本地配置不允许纳入版本控制；示例文件仅提供空值或不可用占位符。
- 发现凭据误提交时，先在实际使用的系统中**吊销或轮换**，再修复当前文件并评估 Git 历史、PR diff、Fork、Actions 日志及缓存中的残留。删除最新文件不等于撤销已泄露的凭据。
- 外部 Fork 的 Pull Request 不应获得部署密钥或其他权限；维护者需要检查 Actions 的 `pull_request_target`、写权限与第三方 Action 权限。

## 维护与验证范围

安全修复应符合 `AGENTS.md` 中的架构与交付约束。静态编译和 CI 成功不代表数据库隔离、鉴权、Agent 工具执行及真实模型安全边界已完成运行时验收。

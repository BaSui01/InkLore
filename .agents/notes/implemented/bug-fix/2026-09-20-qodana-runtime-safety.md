# Agent Note: Qodana 静态检查中的运行时安全修复

Status: implemented

Task: INK-186
Scope: Go 异常路径、资源生命周期、反射枚举与导入格式

## Problem

Qodana 对当前工作区报告了资源未关闭、可能的 nil 解引用、未处理错误、不可达代码、反射枚举分支缺失和大量导入排序问题。项目禁止测试文件，且 R1 代码包含尚未由当前组合根直接消费的 Port、领域值和适配器契约，因此未使用导出符号不能仅凭静态报告删除。

## Decision

优先修复会改变失败路径或资源生命周期的结果：HTTP 限流 reader、DOCX zip writer、日志文件、数据库和网络响应的关闭均有明确的生命周期处理；跨域提交、文件读取、模型能力投影、大纲重排和世界观检查在使用结果前先处理错误或 nil；反射 kind 分派补默认分支；迁移与工具 CLI 显式处理输出、关闭和序列化错误。对 Qodana 命中的 Go 文件运行 goimports，统一导入分组与排序。领域契约常量、导出 Port 和接口参数保留，交由实际装配和契约验收判断。

## Alternatives considered

- 直接删除全部未使用导出符号：未采用。R1 的领域 Port、错误码、状态值和适配器接口可能由未来组合根或契约工具消费，批量删除会破坏冻结设计的词汇表。
- 在 qodana.yaml 中排除全部 warning：未采用。这样会隐藏实际的资源和异常路径问题，不能证明代码安全。
- 只依赖 `go build`：未采用。编译器不会覆盖资源关闭、错误返回和部分控制流风险。

## Consequences

异常路径现在会更早返回真实错误，应用关闭时会释放可选日志文件，HTTP 输入和导出 writer 的资源生命周期更明确。导入整理触及 Qodana 命中的 Go 文件，代码行为不变但工作区 diff 增大。未使用符号类结果仍会出现在没有新 Qodana 云端报告的本地旧 SARIF 中，需在带 `QODANA_TOKEN` 的环境重新扫描后再按实际调用关系处置。

## Verification

- `gofmt`：对本次修改和 Qodana 命中的 Go 文件执行，完成。
- `goimports -l $(rg --files -g '*.go' cmd internal migrations)`：无输出。
- `go build ./...`：退出码 0。
- `go vet ./...`：退出码 0。
- `go run ./cmd/archguard -reachability`：R1-R5 全部通过；可达性报告仅列信息项。
- `npm run build:creator`：退出码 0。
- `npm run build:admin`：退出码 0。
- `node scripts/agent-notes.mjs check`：12 篇笔记及链接检查通过。
- `node scripts/check-project-skills.mjs`：12 份 Markdown、74 个相对链接目标检查通过。
- `git diff --check`：退出码 0。
- Qodana Docker 复扫：未完成。`jetbrains/qodana-go:2026.2` 要求 `QODANA_TOKEN` 连接 Qodana Cloud；本机没有该凭据。IDE 生成的旧 SARIF 已用于定位，不能冒充本轮复扫结果。

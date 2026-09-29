# Agent Note: 修复 i18n 检查器顶层导出误判与路径漂移

Status: implemented

Task: INK-212
Scope: cmd/i18ncheck, scripts/check.ps1, .github/workflows/ci.yml

## Problem

此前 cmd/i18ncheck 在单遍扫描 TypeScript 文件时，未判断当前是否处于对象字面量内部（stack > 0），导致将文件末尾的类型注解冒号与导出标识符（如 export const zhCN: Record<string, string> 和 enUS: Record<...>）误识别为词条 key。这导致比较 zh-CN.ts 与 en-US.ts 时误报缺失 zhCN / 多余 enUS。此外，CI 与 check.ps1 中残留了重构前的旧前端路径。

## Decision

1. 修复 cmd/i18ncheck 状态机：仅在 len(stack) > 0 时才处理冒号与词条标识符，忽略所有顶层变量声明与类型注解；
2. 同步更新 scripts/check.ps1 与 .github/workflows/ci.yml 中的多语言文件路径为 frontend/packages/i18n/src/locales/zh-CN.ts 与 en-US.ts。

## Alternatives considered

使用 Babel 或 TypeScript AST 解析器：需要引入外部 Node 依赖或重型解析库；通过在轻量 Go 词法扫描器中增加简单的嵌套深度保护（len(stack) > 0），既保持了 0 外部依赖的高性能，又彻底杜绝了顶层语法干扰。

## Consequences

- go run ./cmd/i18ncheck 能够准确扫描平面或嵌套的多语言词条，不再对顶层导出标识符与类型声明误报；
- CI 与本地检查脚本路径与真实 Monorepo 目录结构完全一致。

## Verification

1. 运行 go run ./cmd/i18ncheck frontend/packages/i18n/src/locales/zh-CN.ts frontend/packages/i18n/src/locales/en-US.ts 输出全部一致；
2. 逐一运行子模块 admin.ts、auth.ts、home.ts 检查均输出全部一致；
3. go build ./...、go vet ./... 与系统检查通过。

# 合约生成器与 Go 变更最终 QA

比较基线：`b0732b31fb9c306ec60d42c4793a069b474b74a1`。

## 合约生成器

在两个临时隔离副本中运行生成器，未在真实仓库运行生成或覆盖资产。

- 基线和当前的 34 份源 API 分片逐字节相同。
- 两个生成器均退出 0，无 stderr，无 issues。
- 两次结果均为：168 paths、246 operations、17 path shards、102 error codes。
- 比较 73 / 72 个产物：70 文件逐字节相同，差异仅三项：
  1. `api/openapi.json` 仅 `/info/title` 和 `/info/description` 改变。
  2. `api/README.md` 当前工程规则链接与说明改变。
  3. 新生成器不再产出 `implementation/requirements-map.md`。
- 所有 path shards、错误 catalog/index/分片、源 schema、其他 catalog MD 都相同。
- 当前生成器重跑一次，所有产物完全一致，确定性通过。
- 生成器 SHA256：旧 `ad60bb743b339384180774ab23ca4f626e9c683e9725b1fc9f670ebb8510fb3c`；当前 `5b6d8e5d43539e2f630d6bb7e1628ee5565f90886c6476b44b04a7699bcb2148`。
- 完整结果：[生成器比较](2026-11-03-generator-checks.json)。

结论：行为差异严格处于预期范围，不需修复生成器。

## Go 业务源码

使用 Go 标准库 `go/parser` 和 `go/scanner` 比较基线与当前所有变更的 Go 文件，排除 `cmd/archguard/main.go`。

- 18 个文件均通过语法解析。
- 17 个文件在去除普通注释后 token 序列逐项相同。
- `internal/app/domains.go` 只将四个导入别名 `llmr1app/llmr1domain/llmr1infra/llmr1http` 对应归一化为 `llmapp/llmdomain/llminfra/llmhttp` 后，token 序列逐项相同（2429 tokens）。
- 人工检查 diff 未改 Go 构建/嵌入等语义指令。
- 完整结果：[Go token 比较](2026-11-03-go-token-checks.json)。

结论：除另行审查的 archguard 外，Go 业务变更仅注释与导入别名，无业务逻辑改变。本 QA 不代替构建、vet 或真实业务运行验收。

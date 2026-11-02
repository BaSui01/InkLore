# 开发环境与快速验证

## 所需组件

- Go：版本以 [go.mod](https://github.com/BaSui01/InkLore/blob/dev/go.mod) 的 toolchain 为准
- Node.js 22、pnpm 11.25.0
- PostgreSQL 17、Redis 7（按环境需求启动）

## 本地构建

```bash
go build ./...
go vet ./...
cd frontend
pnpm install --frozen-lockfile
pnpm run build
```

双端目标端口：API 8080、Creator 3000、Admin 3001。完整 CI 验证遵循 `make ci NOTES_BASE=<真实基准提交>`；不把编译成功代替真实业务或权限验收。

## 推荐先读

- [开发维护指南](https://github.com/BaSui01/InkLore/blob/dev/docs/development-guide.md)
- [本地配置指南](https://github.com/BaSui01/InkLore/blob/dev/docs/local-environment.md)
- [贡献指南](https://github.com/BaSui01/InkLore/blob/dev/CONTRIBUTING.md)

配置仅使用示例或环境变量；请勿在 Wiki、Issue 或日志中填写任何实际密钥。

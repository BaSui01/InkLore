# InkLore 统一开发命令
#
# Windows 用户若未安装 make，可执行等价脚本：pwsh scripts/check.ps1
# 目标：所有校验命令只有一处定义（本文件），CI 与本地共用同一套判据。

SHELL := /bin/sh
GO ?= go
FRONTEND_DIR := frontend

.DEFAULT_GOAL := help

.PHONY: help
help: ## 显示所有可用命令
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

# ── 构建与运行 ──────────────────────────────────────────────

.PHONY: build
build: ## 编译后端二进制到 bin/server
	$(GO) build -o bin/server ./cmd/server

.PHONY: run
run: ## 运行后端服务（默认 :8080）
	$(GO) run ./cmd/server

.PHONY: dev
dev: ## Air 热重载开发
	air

# ── 静态校验 ────────────────────────────────────────────────

.PHONY: fmt
fmt: ## 格式化 Go 代码（会修改文件）
	gofmt -w .

.PHONY: fmt-check
fmt-check: ## 校验格式（不修改文件）
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "以下文件未格式化，请运行 make fmt ："; echo "$$out"; exit 1; fi; echo "gofmt 校验通过"

.PHONY: vet
vet: ## go vet 静态检查
	$(GO) vet ./...

.PHONY: guard
guard: ## 架构红线校验（AGENTS.md R1–R5）
	$(GO) run ./cmd/archguard

.PHONY: guard-report
guard-report: ## 架构红线校验 + 运行路径可达性报告
	$(GO) run ./cmd/archguard -reachability

.PHONY: i18n-check
i18n-check: ## 校验前端多语言 key 集合一致
	$(GO) run ./cmd/i18ncheck $(FRONTEND_DIR)/src/shared/i18n/locales/zh-CN.ts $(FRONTEND_DIR)/src/shared/i18n/locales/en-US.ts

.PHONY: lint
lint: fmt-check vet guard i18n-check ## 一键静态校验（CI 门禁）
	@echo "lint 全部通过"

.PHONY: lint-deep
lint-deep: ## 可选：golangci-lint 深度检查（需先安装）
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run || echo "golangci-lint 未安装，已跳过（make lint 已覆盖基础校验）"

# ── 前端 ───────────────────────────────────────────────────

.PHONY: frontend-install
frontend-install: ## 安装前端依赖
	cd $(FRONTEND_DIR) && npm ci

.PHONY: frontend-build
frontend-build: ## 类型检查 + 构建前端产物
	cd $(FRONTEND_DIR) && npm run build

# ── 组合命令 ────────────────────────────────────────────────

.PHONY: ci
ci: build fmt-check vet guard i18n-check frontend-build ## CI 全量校验（与 .github/workflows/ci.yml 一致）
	@echo "CI 全量校验通过"

# ── 本地依赖 ────────────────────────────────────────────────

.PHONY: up
up: ## 启动本地开发依赖（PostgreSQL + Redis）
	docker compose -f docker-compose.dev.yml up -d

.PHONY: down
down: ## 停止本地开发依赖
	docker compose -f docker-compose.dev.yml down

.PHONY: clean
clean: ## 清理构建产物
	rm -rf bin tmp

# InkLore 生产镜像：多阶段构建
#
# 构建：
#   docker build -t inklore/server:dev .
# 版本注入：
#   docker build --build-arg APP_VERSION=1.0.0 --build-arg GIT_COMMIT=$(git rev-parse --short HEAD) -t inklore/server:1.0.0 .

# ── 构建阶段 ────────────────────────────────────────────────
FROM golang:1.27.1-alpine AS builder

WORKDIR /src

# 依赖单独一层，源码变更时可命中缓存
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG APP_VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_TIME=unknown

RUN CGO_ENABLED=0 GOOS=linux go build \
      -trimpath \
      -ldflags "-s -w -X InkLore/internal/platform/version.Version=${APP_VERSION} -X InkLore/internal/platform/version.GitCommit=${GIT_COMMIT} -X InkLore/internal/platform/version.BuildTime=${BUILD_TIME}" \
      -o /out/server ./cmd/server

# ── 运行阶段 ────────────────────────────────────────────────
FROM alpine:3.20

# ca-certificates：访问各家模型 API 必需；tzdata：北京时间换算必需
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 10001 -s /sbin/nologin inklore

WORKDIR /app

COPY --from=builder /out/server /app/server
COPY --from=builder /src/migrations /app/migrations
# 运行配置模板（ADR-0011）：生产环境要求 configs/config.yaml 存在（platform/config.Bootstrap），
# 镜像只带模板；真实配置由部署侧以只读方式挂载，密钥仍只走环境变量（ADR-0004）。
COPY --from=builder /src/configs /app/configs

USER inklore

EXPOSE 8080

ENV APP_ENV=production
ENV HTTP_ADDR=:8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/healthz >/dev/null 2>&1 || exit 1

ENTRYPOINT ["/app/server"]

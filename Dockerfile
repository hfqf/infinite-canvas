# 构建 Next.js 前端产物。
FROM oven/bun:1.3.13 AS web-build

WORKDIR /app/web
COPY web/package.json web/bun.lock ./
RUN --mount=type=cache,target=/root/.bun/install/cache bun install --frozen-lockfile --cache-dir=/root/.bun/install/cache
COPY VERSION /app/VERSION
COPY CHANGELOG.md /app/CHANGELOG.md
COPY web ./
RUN bun run build

# 构建 Go 后端入口。
FROM golang:1.25-alpine AS api-build

WORKDIR /app
COPY go.mod go.sum ./
COPY config ./config
COPY handler ./handler
COPY middleware ./middleware
COPY model ./model
COPY repository ./repository
COPY router ./router
COPY service ./service
COPY main.go ./
RUN go build -o /server .

# 构建 VTracer 矢量化命令行工具。
FROM rust:1-bookworm AS vtracer-build

RUN cargo install vtracer --version 0.6.4 --locked

# 运行镜像：Next.js 对外监听 3000，Go 只在容器内部监听 8080。
FROM node:22-bookworm-slim

WORKDIR /app
COPY VERSION /app/VERSION
COPY CHANGELOG.md /app/CHANGELOG.md
COPY --from=api-build /server /app/server
COPY --from=vtracer-build /usr/local/cargo/bin/vtracer /usr/local/bin/vtracer
COPY --from=web-build /app/web/public /app/web/public
COPY --from=web-build /app/web/.next/standalone /app/web
COPY --from=web-build /app/web/.next/static /app/web/.next/static
ENV NODE_ENV=production
ENV HOSTNAME=0.0.0.0
ENV PORT=3000
ENV PROMPT_DATA_DIR=/app/data/prompts
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates imagemagick potrace && rm -rf /var/lib/apt/lists/*
# Bump ImageMagick resource limits: 4096²×Q16×RGBA + morphology operations 容易撞 256MiB 缓存上限。
RUN sed -i 's|name="memory" value="256MiB"|name="memory" value="2GiB"|; s|name="area" value="128MP"|name="area" value="256MP"|' /etc/ImageMagick-6/policy.xml
RUN mkdir -p /app/data/prompts

EXPOSE 3000
# 先启动内部 Go API，再由 Next.js 提供页面并代理 /api/*。
CMD ["sh", "-c", "PORT=8080 /app/server & cd /app/web && PORT=3000 node server.js"]

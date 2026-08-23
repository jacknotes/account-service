# ---- 前端构建（Vue3 + Vite）----
FROM node:20-alpine AS frontend

WORKDIR /fe
COPY frontend/package*.json ./
RUN npm install --no-audit --no-fund
COPY frontend/ ./
RUN npm run build

# ---- 后端构建 ----
FROM golang:1.24-alpine AS builder

ENV GOPROXY=https://goproxy.cn,direct
ENV CGO_ENABLED=0

WORKDIR /build

# 复制依赖
COPY go.mod go.sum ./
RUN go mod download

# 复制源码并编译（纯 Go，无需 CGO）
COPY . .
RUN go build -ldflags="-s -w" -o account-service .

# ---- 运行阶段 ----
FROM alpine:3.20

WORKDIR /app

# 复制二进制与前端构建产物
COPY --from=builder /build/account-service .
COPY --from=frontend /fe/dist ./frontend/dist

# 创建用户并降权
RUN adduser -D -u 1001 appuser && \
    chown -R appuser:appuser /app

USER appuser

ENV PORT=8081
ENV FRONTEND_DIR=/app/frontend/dist
EXPOSE 8081

CMD ["./account-service"]

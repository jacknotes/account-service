# 构建阶段
FROM golang:1.21-alpine AS builder

ENV GOPROXY=https://goproxy.cn,direct

WORKDIR /build

# 复制依赖
COPY go.mod go.sum ./
RUN go mod download

# 复制源码并编译（纯 Go，无需 CGO）
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o account-service .

# 运行阶段
FROM alpine:3.19

WORKDIR /app

# 复制二进制和前端
COPY --from=builder /build/account-service .
COPY --from=builder /build/frontend ./frontend

# 创建数据目录（SQLite 数据库）
RUN mkdir -p /app/data

# 默认端口
ENV PORT=8081
EXPOSE 8081

# 数据持久化（需挂载卷）
VOLUME ["/app/data"]

# 默认数据库路径
ENV DATABASE_PATH=/app/data/accounting.db
ENV FRONTEND_DIR=/app/frontend

CMD ["./account-service"]

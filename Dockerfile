# 构建阶段
FROM golang:1.23-alpine AS builder

# 设置工作目录
WORKDIR /app

# 安装必要的工具
RUN apk add --no-cache git ca-certificates tzdata

# 复制 go mod 文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建 Web 服务
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ebook-generator cmd/web/main.go

# 运行阶段
FROM alpine:latest

# 安装必要的运行时依赖
RUN apk --no-cache add ca-certificates tzdata

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/ebook-generator .

# 复制静态文件
COPY --from=builder /app/web/static ./web/static

# 创建必要的目录
RUN mkdir -p web/uploads web/outputs

# 设置权限
RUN chmod +x ebook-generator

# 暴露端口
EXPOSE 8080

# 设置环境变量
ENV GIN_MODE=release
ENV PORT=8080

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# 运行应用
CMD ["./ebook-generator"]
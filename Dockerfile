# 阶段1: 使用官方Go镜像作为构建环境
FROM golang:1.24-alpine AS builder

# 安装CGO编译所需的工具和库
RUN apk add --no-cache gcc g++ musl-dev libwebp-dev

# 设置工作目录
WORKDIR /app

# 复制go.mod和go.sum文件，下载依赖
COPY go.mod go.sum ./
RUN go mod download

# 复制项目所有文件到工作目录
COPY . .

# 启用CGO以支持webp库的编译
RUN GOOS=linux go build -a -installsuffix cgo -o main ./main.go

# 阶段2: 使用轻量级Alpine镜像作为运行环境
FROM alpine:3.18

# 安装运行时依赖
RUN apk --no-cache add ca-certificates tzdata libwebp

# 设置工作目录
WORKDIR /app

# 从构建阶段复制编译好的应用到当前镜像
COPY --from=builder /app/main .

# 暴露应用端口（根据你的应用需要修改）
EXPOSE 8080

# 运行应用
CMD ["./main"]
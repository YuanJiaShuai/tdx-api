# 多阶段构建 - 第一阶段：构建
# 使用官方镜像（如果国内拉取慢，可以配置docker daemon的registry-mirrors）
FROM docker.1ms.run/library/golang:1.23-alpine AS builder

# 替换Alpine镜像源为阿里云
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories

# 设置工作目录
WORKDIR /app

# 设置Go代理（使用国内镜像加速）
ENV GO111MODULE=on \
    GOPROXY=https://goproxy.cn,https://mirrors.aliyun.com/goproxy/,direct \
    GOTOOLCHAIN=auto \
    CGO_ENABLED=0

# 复制 Go 模块文件
COPY go.mod go.sum ./
RUN go mod download

# 复制整个项目的源代码
COPY . .

# 在子 shell 中编译，避免模块路径混淆问题
RUN go mod tidy && (cd web && go build -ldflags="-s -w" -o ../stock-web .)

# 多阶段构建 - 第二阶段：运行
FROM docker.1ms.run/library/alpine:latest

# 替换Alpine镜像源为阿里云，安装必要的运行时依赖
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories && \
    apk --no-cache add ca-certificates tzdata wget python3 py3-pip libstdc++

# 设置时区为上海
ENV TZ=Asia/Shanghai

# 创建非root用户
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

# 设置工作目录
WORKDIR /app

# ===================================================================
# 【语法修正】
# 从构建阶段复制编译好的二进制文件
COPY --from=builder /app/stock-web .
# ===================================================================

# ===================================================================
# 【语法修正】
# 复制静态文件
COPY --from=builder /app/web/static ./static
# ===================================================================

COPY --from=builder /app/formula-worker ./formula-worker
COPY --from=builder /app/deploy/docker-entrypoint.sh ./docker-entrypoint.sh

RUN apk --no-cache add --virtual .hqchart-build g++ python3-dev zlib-dev linux-headers && \
    python3 /app/formula-worker/install_hqchartpy2.py && \
    apk del .hqchart-build

# 更改文件所有者
RUN chmod +x /app/docker-entrypoint.sh /app/formula-worker/worker.py && \
    chown -R appuser:appuser /app

# 切换到非root用户
USER appuser

# 暴露端口
EXPOSE 8080

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/health || exit 1

# 启动应用
CMD ["./docker-entrypoint.sh"]

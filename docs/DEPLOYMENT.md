# 部署与运行

## Docker

推荐使用 Docker Compose：

```bash
docker-compose up -d
docker-compose logs -f
```

访问 `http://localhost:8080`。

常用维护命令：

```bash
docker-compose restart
docker-compose stop
docker-compose down
docker-compose down --rmi all
```

如果 Docker Hub 拉取基础镜像超时，在 Docker Desktop 的 Docker Engine 配置里添加 registry mirrors，或在网络条件更好的环境构建。

## 源码运行

要求 Go 1.23+。

```bash
go mod download
cd web
go run .
```

Windows 可运行 `web/start.bat`，macOS/Linux 可运行：

```bash
cd web
chmod +x start.sh
./start.sh
```

## 验证

```bash
curl "http://localhost:8080/api/health"
curl "http://localhost:8080/api/quote?code=000001"
```

根模块和 Web 子模块测试：

```bash
GOPROXY=https://goproxy.cn,direct go test ./...
cd web
GOPROXY=https://goproxy.cn,direct go test ./...
GOPROXY=https://goproxy.cn,direct go build -o /tmp/tdx-api-web .
```

## 常见问题

| 问题 | 处理 |
| --- | --- |
| `go` 命令找不到 | 安装 Go 1.23+，确认 `go version` 正常 |
| 模块下载慢 | 设置 `GOPROXY=https://goproxy.cn,direct` |
| 端口 8080 被占用 | 修改 `web/server.go` 中端口或调整 Docker 端口映射 |
| Web 启动失败 | 用 `go run .`，不要只运行 `server.go` |
| Docker 构建失败 | 先看 `docker-compose logs -f`，再确认基础镜像能拉取 |

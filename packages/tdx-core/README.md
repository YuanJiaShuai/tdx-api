# TDX Core

通达信协议、行情客户端、数据模型、扩展拉取和示例代码。

这个目录是 `github.com/injoyai/tdx` Go module。上层服务通过 `apps/web/go.mod` 中的本地 `replace` 引用它：

```text
replace github.com/injoyai/tdx => ../../packages/tdx-core
```

常用验证命令：

```bash
GOPROXY=https://goproxy.cn,direct go test ./...
```

# Workbench Core

Web 工作台和选股服务共享的 Go module。

当前包含：

- 公式、股票池、策略、自动化任务、运行记录、选股结果、交易系统状态等 DTO
- SQLite 存储层与默认数据初始化
- 系统市场股票池生成逻辑

上层服务通过本地 `replace` 引用：

```go
replace workbench-core => ../../packages/workbench-core
```

行情代码列表由上层服务通过 `SetCodeModelProvider` 注入，避免共享包直接持有服务运行时。

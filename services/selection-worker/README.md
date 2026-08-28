# Selection Worker

独立选股和自动化任务服务，负责 Cron 调度、选股任务运行和运行记录写入。

容器入口使用 `services/selection-worker/Dockerfile`。

默认端口：`8082`

当前版本仍复用 `apps/web/` 下的 Go 代码启动 worker 角色，后续可以继续把选股逻辑从 Web app 中进一步下沉到这里。

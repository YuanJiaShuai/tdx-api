# Web App

Web 页面、业务 API 网关和工作台后端。容器入口使用 `apps/web/Dockerfile`。

默认端口：`8080`

依赖服务：

- `MARKET_SERVICE_URL=http://market-service:8081`
- `FORMULA_WORKER_URL=http://formula-worker:8712`

Web 容器默认关闭定时调度：`AUTOMATION_SCHEDULER_ENABLED=false`。

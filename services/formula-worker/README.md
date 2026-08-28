# Formula Worker

独立公式引擎服务，负责运行 HQChartPy2 或 fallback 公式执行器。

容器入口使用 `services/formula-worker/Dockerfile`。

默认端口：`8712`

Web 和选股任务服务通过 `FORMULA_WORKER_URL` 调用它。

宿主机默认映射端口为 `18712`。

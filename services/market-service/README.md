# Market Service

独立行情服务，负责通达信行情、K 线、分时、成交、代码、板块、交易日等 API。

容器入口使用 `services/market-service/Dockerfile`。

默认端口：`8081`

Web 容器通过 `MARKET_SERVICE_URL` 调用它。

宿主机默认映射端口为 `18081`，避免和其他本地服务冲突。

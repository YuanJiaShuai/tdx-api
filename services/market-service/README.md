# Market Service

独立行情服务，负责通达信行情、K 线、分时、成交、代码、板块、交易日等 API。

服务源码和 Go module 位于当前目录，直接依赖 `packages/tdx-core`，不会再编译 `apps/web`。

容器入口使用 `services/market-service/Dockerfile`。

默认端口：`8081`

Web 容器通过 `MARKET_SERVICE_URL` 调用它。

宿主机默认映射端口为 `18081`，避免和其他本地服务冲突。

源码运行：

```bash
cd services/market-service
go run .
```

可选环境变量：

- `PORT`：监听端口，默认 `8081`
- `MARKET_SYNC_ON_START`：启动时是否同步代码和交易日数据，默认 `true`
- `MARKET_QUOTE_POLL_INTERVAL_SECONDS`：SSE 行情订阅后台轮询间隔，默认 `3` 秒，最大 `60` 秒
- `MARKET_QUOTE_CACHE_TTL_SECONDS`：行情缓存视为新鲜的时长，默认 `15` 秒
- `MARKET_QUOTE_CACHE_STALE_SECONDS`：行情允许回落到陈旧缓存的最长时长，默认 `300` 秒
- `MARKET_INFO_SYNC_INTERVAL_MS`：龙虎榜、游资、研报、公告批量同步时的请求间隔，默认 `300` 毫秒
- `HIKYUU_DATA_SERVICE_URL`：hikyuu 历史数据服务地址，Compose 默认配置为
  `http://hikyuu-data-service:8091`。未配置时历史 K 线继续使用原有数据源。

`market-service` 还提供后台任务接口：

- `POST /api/tasks/pull-kline`：批量拉取 K 线并写入本地数据库
- `POST /api/tasks/sync-gbbq`：同步股本变迁/除权除息数据
- `GET /api/tasks/{task_id}`：查询任务状态
- `POST /api/tasks/{task_id}/cancel`：取消任务

行情接口：

- `GET /api/quote?code=600519,000001`：兼容原有的通达信行情响应
- `GET /api/quote/standard?codes=600519.SH,000001.SZ`：返回带标准代码、市场、来源和获取时间的行情快照
- `GET /api/stream/quotes?codes=600519.SH,000001.SZ`：SSE 行情订阅，盘中默认每 3 秒轮询并只推送变化数据
- `GET /api/kline?code=600519&type=day`：历史 K 线，优先使用 hikyuu，失败时回退原有数据源
- `GET /api/kline-history?code=600519&type=day&limit=120`：指定数量的历史 K 线，优先使用 hikyuu，失败时回退原有数据源
- `GET /api/finance/standard?code=600519.SH`：标准财务快照，失败时读取本地缓存
- `GET /api/news?symbol=600519.SH&limit=20`：查询去重后的资讯
- `POST /api/news`：写入一条或多条已解析资讯
- `POST /api/news/sync`：从 RSS/Atom 风格 XML 资讯源采集并去重
- `POST /api/market/sync`：按交易日批量同步龙虎榜、游资动向、个股研报和公司公告，并写入本地 SQLite 快照
- `GET /api/analysis/context?codes=600519.SH,000001.SZ`：返回 AI/策略可复用的聚合上下文

财务和资讯数据存储在 `MARKET_DATABASE_PATH` 指定的 SQLite 数据库中，默认是
`data/database/market.db`。资讯目前采用标准化写入和 RSS 采集适配，具体新闻源的解析与授权
应在接入时单独配置。

市场信息同步快照存储在同一个数据库的 `market_snapshots` 表中。页面查询会优先读取当天或最近一次
快照，只有本地没有快照时才回源东方财富。自动化模块会自动创建 `龙虎榜同步`、`游资动向同步`、
`个股研报同步`、`公司公告同步` 四个默认任务，工作日从 `18:00` 起错峰执行；任务可以在 Web 的
“自动化”页面中分别启停和编辑。

代码参数支持 `600519`、`SH600519`、`600519.SH` 等形式。标准行情和订阅接口当前支持沪深北交易所的股票、ETF 和指数。

SSE 事件类型包括：

- `ready`：连接建立
- `quote`：行情发生变化
- `heartbeat`：连接保活

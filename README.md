# TDX Workbench

一个本地运行的 A 股行情、公式选股与交易研究工作台。

它从通达信公共行情服务取数，把报价、K 线、分时、分笔、财务、板块、除权除息等数据整理成 API；同时提供 Web 界面，用来查看行情、维护公式、管理股票池、运行选股任务和复盘结果。

> 本项目仅供学习和研究使用。行情数据可能延迟、缺失或出错，不构成任何投资建议。

## 服务拆分

当前项目已经按多服务方式组织，几个服务可以单独构建、单独启动、单独重启：

| 服务 | 目录 | 容器 | 说明 |
| --- | --- | --- | --- |
| Web 工作台 | `apps/web/` | `tdx-workbench-web` | 前端页面、业务 API 网关、公式/选股管理入口 |
| 行情服务 | `services/market-service/` | `tdx-workbench-market` | 通达信行情、K 线、分时、成交、代码、板块、交易日等 API |
| Hikyuu 数据服务 | `services/hikyuu-data-service/` | `tdx-workbench-hikyuu-data` | hikyuu 全量/盘后增量下载、定时收盘作业和任务状态 API |
| 公式引擎 | `services/formula-worker/` | `tdx-workbench-formula` | HQChartPy2 或 fallback 公式执行器 |
| 指标选股 | `services/selection-worker/` | `tdx-workbench-selection` | Cron 调度、选股运行、运行记录写入 |
| AI 分析 | `services/ai-service/` | `tdx-workbench-ai` | DeepSeek/OpenAI-compatible 模型调用、股票分析、自选分析 |
| 通用核心 | `packages/tdx-core/` | 无独立容器 | 通达信协议库、数据模型、扩展拉取逻辑 |
| 工作台核心 | `packages/workbench-core/` | 无独立容器 | 公式、股票池、策略、自动化记录等共享模型与存储 |

## 目录结构

```text
tdx-workbench/
├── apps/
│   └── web/                    # Web 页面、API 网关和工作台后端
├── services/
│   ├── market-service/          # 独立 Go 行情服务
│   ├── formula-worker/          # Python 公式引擎服务
│   ├── selection-worker/        # 独立 Go 指标选股服务
│   └── ai-service/              # 独立 Go AI 分析服务
├── packages/
│   ├── tdx-core/                # Go 通达信核心库和示例
│   └── workbench-core/          # Web 和选股服务共享的模型与存储层
├── deploy/                      # 本地部署辅助脚本
├── data/                        # 本地数据库和运行数据，Docker 会挂载
├── reports/                     # 选股、行情跟踪等输出
├── docs/                        # 项目文档
├── docker-compose.yml
└── go.work
```

## Docker 启动

启动全部服务：

```bash
docker compose up -d --build
```

打开 Web：

```text
http://localhost:8080
```

宿主机端口：

| 服务 | 容器内端口 | 宿主机端口 |
| --- | --- | --- |
| Web 工作台 | `8080` | `8080` |
| 行情服务 | `8081` | `18081` |
| 公式引擎 | `8712` | `18712` |
| Hikyuu 数据服务 | `8091` | `18091` |
| 指标选股 | `8082` | `18082` |
| AI 分析 | `8083` | `18083` |

## 单独部署

改了哪个服务，就可以只重建/重启哪个服务：

```bash
docker compose up -d --build stock-web
docker compose up -d --build market-service
docker compose up -d --build formula-worker
docker compose up -d --build hikyuu-data-service
docker compose up -d --build selection-worker
docker compose up -d --build ai-service
```

只重启不重建：

```bash
docker compose restart stock-web
docker compose restart market-service
docker compose restart formula-worker
docker compose restart hikyuu-data-service
docker compose restart selection-worker
docker compose restart ai-service
```

查看日志：

```bash
docker compose logs -f stock-web
docker compose logs -f market-service
docker compose logs -f formula-worker
docker compose logs -f hikyuu-data-service
docker compose logs -f selection-worker
docker compose logs -f ai-service
```

## 源码运行

要求 Go 1.23+，Python 用于公式 worker。

```bash
python3 services/formula-worker/worker.py
cd services/market-service
go run .
cd ../hikyuu-data-service
python3 -m pip install -r requirements.txt
uvicorn app:app --host 0.0.0.0 --port 8091
cd ../selection-worker
go run .
cd ../../services/ai-service
go run .
cd ../../apps/web
go run .
```

根目录有 `go.work`，所以也可以在根目录统一管理多个 Go module。

## 开发验证

```bash
cd packages/tdx-core
GOPROXY=https://goproxy.cn,direct go test ./...

cd ../workbench-core
GOPROXY=https://goproxy.cn,direct go test ./...

cd ../../apps/web
GOPROXY=https://goproxy.cn,direct go test ./...
GOPROXY=https://goproxy.cn,direct go build -o /tmp/tdx-workbench-web .

cd ../../services/market-service
GOPROXY=https://goproxy.cn,direct go test ./...
GOPROXY=https://goproxy.cn,direct go build -o /tmp/tdx-market-service .

cd ../selection-worker
GOPROXY=https://goproxy.cn,direct go test ./...
GOPROXY=https://goproxy.cn,direct go build -o /tmp/tdx-selection-worker .

cd ../ai-service
GOPROXY=https://goproxy.cn,direct go test ./...
```

Docker 配置检查：

```bash
docker compose config --quiet
```

## 常用 API

| 接口 | 说明 | 示例 |
| --- | --- | --- |
| `GET /api/quote` | 五档行情 | `/api/quote?code=000001` |
| `GET /api/kline` | K 线，日线默认前复权 | `/api/kline?code=000001&type=day` |
| `GET /api/minute` | 分时走势 | `/api/minute?code=000001` |
| `GET /api/trade` | 分笔成交 | `/api/trade?code=000001` |
| `GET /api/search` | 股票搜索 | `/api/search?keyword=平安` |
| `GET /api/stock-info` | 行情、K 线、分时综合信息 | `/api/stock-info?code=000001` |
| `POST /api/batch-quote` | 批量行情 | `{"codes":["000001","600519"]}` |
| `GET /api/formula/health` | 公式 worker 状态 | `/api/formula/health` |
| `GET /api/hikyuu/health` | Hikyuu 数据服务状态 | `http://localhost:18091/api/hikyuu/health` |
| `POST /api/hikyuu/tasks/full-sync` | 启动 hikyuu 首次全量下载 | `http://localhost:18091/api/hikyuu/tasks/full-sync` |
| `POST /api/hikyuu/tasks/after-close-sync` | 启动 hikyuu 盘后增量下载 | `http://localhost:18091/api/hikyuu/tasks/after-close-sync` |
| `POST /api/formula/run` | 直接执行公式 | `{"symbol":"000001","script":"T:MA(C,5);"}` |
| `GET /api/automations` | 自动化任务列表 | `/api/automations` |
| `GET /api/selection-results` | 选股命中结果 | `/api/selection-results?limit=100` |
| `GET /api/ai/providers` | AI 供应商列表 | `/api/ai/providers` |
| `POST /api/ai/analyze/stock` | 单股 AI 分析 | `{"provider":"deepseek","symbol":"603171"}` |
| `POST /api/ai/analyze/watchlist` | 自选/观察池 AI 分析 | `{"provider":"deepseek","pool_id":"watchlist"}` |

在 Docker 模式下，`selection-worker` 的行情请求默认转发给
`market-service`。只有显式设置 `SELECTION_MARKET_FALLBACK_DIRECT=true` 时，
worker 才会在行情服务失败后尝试直接连接通达信。

完整接口见 [API 参考](docs/api-reference.md)。

## 文档

| 文档 | 说明 |
| --- | --- |
| [部署指南](docs/deployment-guide.md) | Docker、本地运行、验证和排障 |
| [Web 使用指南](docs/web-guide.md) | 页面入口、功能区域和操作流程 |
| [API 参考](docs/api-reference.md) | REST API 参数、响应和示例 |
| [除权除息与复权算法](docs/gbbq-adjustment.md) | gbbq 数据结构和复权计算说明 |
| [文档历史](docs/document-history.md) | 旧文档和阶段性说明的去向摘要 |

## 开源组件

| 项目 | 用途 | 说明 |
| --- | --- | --- |
| [oficcejo/tdx-api](https://github.com/oficcejo/tdx-api) | 原始项目基础 | 本项目在其基础上继续扩展 Web、API、自动化和部署能力 |
| [injoyai/tdx](https://github.com/injoyai/tdx) | 通达信协议库 | 当前整理为 `packages/tdx-core/` 的通用核心 |
| [jones2000/HQChart](https://github.com/jones2000/HQChart) | 专业行情展示 | 用于专业 K 线、指标和图表展示 |
| [jones2000/hqchartPy2](https://github.com/jones2000/hqchartPy2) | 公式计算引擎 | 用于接入通达信/麦语法风格公式解析与批量选股 |

Docker 公式 worker 会自动检测 `HQChartPy2`：检测到时报告 `engine=hqchartpy2`，未安装时使用内置 fallback 公式执行器，保证本地流程仍能跑通。

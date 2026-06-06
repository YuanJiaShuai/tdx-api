# TDX 股票数据查询系统

一套面向本地部署的股票数据服务：基于通达信协议获取行情数据，内置 Web 可视化界面、RESTful API、公式选股、自动化任务和 Docker 单服务部署能力。

## 项目初衷

这个项目最开始的想法很简单：先通过 `tdx-api` 稳定获取通达信行情数据，再结合 Codex 帮助梳理、编写和迭代自动化策略，让本地环境可以逐步形成一套可运行、可观察、可复用的策略选股工作流。

随着使用场景增加，项目从单纯的数据接口扩展到 Web 展示、公式管理、股票池、定时任务、任务结果保存和 Webhook 通知。现在它更像一个本地股票数据与自动化选股中心：数据服务负责把行情取回来，公式和任务系统负责把策略跑起来，Web 界面负责让结果看得见、查得到、能复盘。

> [!IMPORTANT]
> 目标是让用户只启动一个 `tdx-api` Docker 服务，就能在本地完成行情查询、专业 K 线展示、公式测试、股票池选股、定时同步和任务结果查看。

| 能力方向 | 当前重点 |
| --- | --- |
| 数据服务 | 通达信行情、K 线、分时、分笔、财务、除权、板块和扩展行情接口 |
| Web 界面 | 股票搜索、行情卡片、K 线/分时图、专业行情页、选股结果中心 |
| 公式选股 | 自定义公式管理、公式测试、手动选股、定时选股、运行记录 |
| 自动化 | 系统同步任务、Cron 调度、任务模板、Webhook 通知 |
| 部署方式 | Docker Compose 单服务启动，同时运行 Go Web 服务和 Python 公式 worker |

## 项目来源与组件声明

| 项目 | 用途 | 说明 |
| --- | --- | --- |
| [oficcejo/tdx-api](https://github.com/oficcejo/tdx-api) | 原始项目基础 | 本项目在其基础上继续扩展 Web、API、自动化和部署能力 |
| [injoyai/tdx](https://github.com/injoyai/tdx) | 上游协议库 | 提供通达信协议相关能力 |
| [jones2000/HQChart](https://github.com/jones2000/HQChart) | 专业行情展示 | 用于专业 K 线、指标和图表展示方向 |
| [jones2000/hqchartPy2](https://github.com/jones2000/hqchartPy2) | 公式计算引擎 | 用于后续接入通达信/麦语法风格公式解析与批量选股 |

相关开源组件的版权与许可证归原作者及对应项目所有。本项目的 Docker 公式 worker 会自动检测 `HQChartPy2` 模块：检测到时报告 `engine=hqchartpy2`，未安装时使用内置 fallback 公式执行器保障本地闭环。

## 功能概览

| 模块 | 能力 |
| --- | --- |
| 行情数据 | 五档报价、K 线、分时、分笔成交、指数、ETF、交易日 |
| 扩展数据 | 集合竞价、股本变迁、财务/F10、板块、行业归属、统计、新股申购 |
| 数据源 | 通达信原始数据、同花顺前复权日线、扩展行情 TdxExHq |
| Web 界面 | 股票搜索、行情卡片、K 线图、分时图、成交明细、专业行情页 |
| 公式与选股 | 自定义公式、公式测试、股票池、选股任务、运行记录、选股结果中心 |
| 自动化 | 系统同步任务、选股任务、Cron 调度、任务模板、Webhook 通知 |
| 部署 | Docker Compose 单服务、本地源码运行、Windows/macOS/Linux 脚本 |

## 快速开始

### Docker

```bash
docker-compose up -d
```

访问 `http://localhost:8080`。Docker 镜像内部会同时启动 Go Web 服务和 Python 公式 worker，用户只需要启动这一个服务。

常用命令：

```bash
docker-compose logs -f
docker-compose restart
docker-compose down
```

### 源码运行

要求 Go 1.23+。

```bash
go mod download
python3 formula-worker/worker.py
cd web
go run .
```

访问 `http://localhost:8080`。

注意：Web 服务必须使用 `go run .`，不能只运行 `server.go`，否则扩展接口文件不会参与编译。

## 常用 API

所有标准 API 均返回：

```json
{"code": 0, "message": "success", "data": {}}
```

| 接口 | 说明 | 示例 |
| --- | --- | --- |
| `GET /api/quote` | 五档行情 | `/api/quote?code=000001` |
| `GET /api/kline` | K 线 | `/api/kline?code=000001&type=day` |
| `GET /api/minute` | 分时 | `/api/minute?code=000001` |
| `GET /api/trade` | 分笔成交 | `/api/trade?code=000001` |
| `GET /api/search` | 股票搜索 | `/api/search?keyword=平安` |
| `GET /api/stock-info` | 综合信息 | `/api/stock-info?code=000001` |
| `POST /api/batch-quote` | 批量行情 | `{"codes":["000001","600519"]}` |
| `GET /api/kline-all/tdx` | 通达信全量 K 线 | `/api/kline-all/tdx?code=000001&type=day` |
| `GET /api/kline-all/ths` | 同花顺前复权 K 线 | `/api/kline-all/ths?code=000001&type=day` |
| `GET /api/workday` | 交易日 | `/api/workday?date=2026-06-05` |
| `GET /api/gbbq` | 股本变迁 | `/api/gbbq?code=600519` |
| `GET /api/finance` | 财务信息 | `/api/finance?code=600519` |
| `GET /api/block` | 板块成分 | `/api/block?file=gn&with_index=true` |
| `GET /api/exhq/markets` | 扩展行情市场 | `/api/exhq/markets` |
| `GET /api/formulas` | 公式列表 | `/api/formulas` |
| `POST /api/formulas/{id}/test` | 公式测试 | `{"symbol":"000001"}` |
| `GET /api/stock-pools` | 股票池列表 | `/api/stock-pools` |
| `GET /api/automations` | 自动化任务列表 | `/api/automations` |
| `POST /api/automations/templates` | 创建系统任务模板 | `{"template":"evening_kline"}` |
| `POST /api/automations/{id}/run` | 手动运行任务 | `{}` |
| `GET /api/selection-results` | 选股命中结果 | `/api/selection-results?limit=100` |
| `GET /api/webhooks` | Webhook 列表 | `/api/webhooks` |
| `GET /api/formula/health` | 公式 worker 状态 | `/api/formula/health` |
| `POST /api/formula/run` | 直接执行公式 | `{"symbol":"000001","script":"T:MA(C,5);"}` |
| `GET /api/hqchart/kline` | Web 专业行情 K 线适配 | `/api/hqchart/kline?symbol=000001&period=day` |
| `GET /api/hqchart/history` | HQChart 原生历史 K 线适配 | `/api/hqchart/history?symbol=000001&period=day` |

完整接口说明见 [docs/API.md](docs/API.md)。

## 文档索引

| 文档 | 说明 |
| --- | --- |
| [docs/API.md](docs/API.md) | REST API 参考 |
| [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) | Docker、本地运行和排障 |
| [docs/WEB.md](docs/WEB.md) | Web 页面使用说明 |
| [docs/gbbq_除权除息与复权算法.md](docs/gbbq_除权除息与复权算法.md) | gbbq 与复权算法说明 |
| [docs/HISTORY.md](docs/HISTORY.md) | 历史文档合并摘要 |

## 项目结构

```text
tdx-api/
├── client.go                  # 通达信客户端核心
├── protocol/                  # 协议帧、模型和解析
├── extend/                    # 扩展爬取、拉取、收益计算
├── formula-worker/            # 容器内公式计算服务
├── web/                       # REST API 与 Web 静态资源
├── scripts/                   # Python 策略、回测、接口检查脚本
├── deploy/                    # Docker/本地部署辅助脚本
├── docs/                      # 长期维护文档
├── Dockerfile
└── docker-compose.yml
```

## 开发验证

```bash
GOPROXY=https://goproxy.cn,direct go test ./...
cd web
GOPROXY=https://goproxy.cn,direct go test ./...
GOPROXY=https://goproxy.cn,direct go build -o /tmp/tdx-api-web .
```

## 免责声明

本项目仅供学习和研究使用。数据来自通达信公共服务器及相关公开接口，可能存在延迟或不完整，不构成任何投资建议。

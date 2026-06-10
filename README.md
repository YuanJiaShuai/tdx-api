# TDX API

一个放在本地运行的 A 股行情与策略工作台。

它从通达信公共行情服务取数，把报价、K 线、分时、分笔、财务、板块、除权除息等数据整理成 REST API；同时带一个 Web 界面，用来查看行情、管理公式、维护股票池、运行选股任务和复盘结果。项目的重点不是做一个大而全的平台，而是把日常研究里那些散落的动作收拢到一套能反复运行、能看见结果、能留痕的本地流程里。

> 本项目仅供学习和研究使用。行情数据可能延迟、缺失或出错，不构成任何投资建议。

## 为什么做它

很多量化或半自动选股脚本，最开始都只是几段临时程序：今天拉 K 线，明天补一个复权，后天再写个定时任务。脚本越来越多以后，真正麻烦的不是某一个指标怎么算，而是数据入口不统一、任务跑没跑不知道、结果散在不同文件里、策略复盘也缺少上下文。

`tdx-api` 想解决的是这类“本地研究工作流”的秩序问题：

| 你想做的事 | 项目提供的入口 |
| --- | --- |
| 快速查一个股票的行情、K 线和分时 | Web 页面与 REST API |
| 把通达信数据稳定拉到本地 | Go 客户端、批量接口、同步任务 |
| 测试公式或批量选股 | 公式 worker、股票池、任务记录 |
| 每天自动同步和筛选 | Cron 自动化、系统模板、Webhook |
| 复盘某次选股命中 | 运行记录、命中结果、决策备注 |

如果只需要一个协议库，可以直接使用上游能力；如果你想要一个“启动后就能查、能跑、能复盘”的本地小系统，这个项目更接近那个方向。

## 现在能做什么

| 模块 | 能力 |
| --- | --- |
| 行情接口 | 五档报价、日/周/月/分钟 K 线、分时、分笔成交、指数、ETF、交易日 |
| 扩展数据 | 集合竞价、股本变迁、财务/F10、板块、行业归属、市场统计、新股申购 |
| 多数据源 | 通达信原始数据、同花顺前复权日线、扩展行情 TdxExHq |
| Web 工作台 | 股票搜索、行情卡片、K 线/分时图、成交明细、专业行情页 |
| 公式选股 | 自定义公式、公式测试、股票池、手动选股、定时选股、运行记录 |
| 自动化 | 系统同步任务、选股任务、Cron 调度、任务模板、Webhook 通知 |
| 部署 | Docker Compose 单服务、本地源码运行、Windows/macOS/Linux 脚本 |

## 一分钟启动

### Docker Compose

```bash
docker-compose up -d
```

然后打开：

```text
http://localhost:8080
```

容器内会同时启动 Go Web 服务和 Python 公式 worker。常用操作：

```bash
docker-compose logs -f
docker-compose restart
docker-compose down
```

### 源码运行

要求 Go 1.23+，Python 用于公式 worker。

```bash
go mod download
python3 formula-worker/worker.py
cd web
go run .
```

访问 `http://localhost:8080`。

注意：Web 服务必须使用 `go run .`，不要只运行 `server.go`，否则同目录下的扩展接口文件不会参与编译。

## API 速览

标准接口统一返回：

```json
{"code": 0, "message": "success", "data": {}}
```

常用接口：

| 接口 | 说明 | 示例 |
| --- | --- | --- |
| `GET /api/quote` | 五档行情 | `/api/quote?code=000001` |
| `GET /api/kline` | K 线，日线默认前复权 | `/api/kline?code=000001&type=day` |
| `GET /api/minute` | 分时走势 | `/api/minute?code=000001` |
| `GET /api/trade` | 分笔成交 | `/api/trade?code=000001` |
| `GET /api/search` | 股票搜索 | `/api/search?keyword=平安` |
| `GET /api/stock-info` | 行情、K 线、分时综合信息 | `/api/stock-info?code=000001` |
| `POST /api/batch-quote` | 批量行情 | `{"codes":["000001","600519"]}` |
| `GET /api/kline-all/tdx` | 通达信全量 K 线 | `/api/kline-all/tdx?code=000001&type=day` |
| `GET /api/kline-all/ths` | 同花顺前复权 K 线 | `/api/kline-all/ths?code=000001&type=day` |
| `GET /api/workday` | 交易日判断 | `/api/workday?date=2026-06-05` |
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

完整接口见 [docs/API.md](docs/API.md)。

## 工作流示例

一个典型的本地研究流程可以是这样：

1. 用 `/api/search` 或 Web 搜索找到标的。
2. 查看报价、K 线、分时和成交明细，确认数据是否正常。
3. 在公式中心写一个条件，比如均线、突破、回撤或量价结构。
4. 选择股票池，手动运行一次选股，观察命中列表。
5. 把稳定的公式保存成自动化任务，设置 Cron 和 Webhook。
6. 每天查看运行记录、命中结果和复盘备注。

这个流程并不替你做判断，它只是把取数、执行、记录、回看这些重复动作变得更可靠。

## 目录结构

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

## 文档

| 文档 | 说明 |
| --- | --- |
| [docs/API.md](docs/API.md) | REST API 参考 |
| [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) | Docker、本地运行和排障 |
| [docs/WEB.md](docs/WEB.md) | Web 页面使用说明 |
| [docs/gbbq_除权除息与复权算法.md](docs/gbbq_除权除息与复权算法.md) | gbbq 与复权算法说明 |
| [docs/HISTORY.md](docs/HISTORY.md) | 历史文档合并摘要 |

## 开发验证

```bash
GOPROXY=https://goproxy.cn,direct go test ./...
GOPROXY=https://goproxy.cn,direct go vet ./...

cd web
GOPROXY=https://goproxy.cn,direct go test ./...
GOPROXY=https://goproxy.cn,direct go vet ./...
GOPROXY=https://goproxy.cn,direct go build -o /tmp/tdx-api-web .
```

## 开源组件

| 项目 | 用途 | 说明 |
| --- | --- | --- |
| [oficcejo/tdx-api](https://github.com/oficcejo/tdx-api) | 原始项目基础 | 本项目在其基础上继续扩展 Web、API、自动化和部署能力 |
| [injoyai/tdx](https://github.com/injoyai/tdx) | 上游协议库 | 提供通达信协议相关能力 |
| [jones2000/HQChart](https://github.com/jones2000/HQChart) | 专业行情展示 | 用于专业 K 线、指标和图表展示 |
| [jones2000/hqchartPy2](https://github.com/jones2000/hqchartPy2) | 公式计算引擎 | 用于接入通达信/麦语法风格公式解析与批量选股 |

相关开源组件的版权与许可证归原作者及对应项目所有。Docker 公式 worker 会自动检测 `HQChartPy2`：检测到时报告 `engine=hqchartpy2`，未安装时使用内置 fallback 公式执行器，保证本地流程仍能跑通。

## 免责声明

本项目仅用于学习、研究和个人本地工具构建。数据来自通达信公共服务器及相关公开接口，可能存在延迟、不完整、接口变动或解析误差。项目中的接口、页面、公式和自动化任务都不构成投资建议，也不保证任何策略收益。请自行核验数据并独立承担使用风险。

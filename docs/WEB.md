# Web 使用说明

Web 页面入口：`http://localhost:8080`。

## 基本流程

1. 在搜索框输入股票代码或名称，例如 `000001`、`600519`、`平安银行`。
2. 点击搜索后进入股票详情。
3. 在标签页查看五档行情、K 线图、分时图和分时成交。
4. 在 K 线图上切换日 K、周 K、月 K、30 分、15 分、5 分。

顶部工作区还提供：

- `专业行情`：面向 HQChart 接入预留的数据展示页，当前使用同一套 K 线适配接口绘制专业 K 线。
- `公式`：新增、编辑、删除自定义公式，并可选择股票做公式测试。
- `选股结果`：查看选股任务命中的股票，可按公式、股票代码和最近一次运行筛选，并一键打开专业行情。
- `自动化`：维护股票池、创建选股任务、手动运行任务、查看运行记录。
- `Webhook`：维护可选通知地址，任务完成或失败时按事件发送 JSON 通知。

## 页面数据

| 区域 | 内容 |
| --- | --- |
| 基本信息 | 最新价、涨跌额、涨跌幅、成交量、成交额、开高低 |
| 五档行情 | 买一到买五、卖一到卖五 |
| K 线图 | 蜡烛图、成交量、多周期切换 |
| 分时图 | 当日或指定日期分时走势 |
| 分时成交 | 最近或指定日期逐笔成交 |
| 专业行情 | `/api/hqchart/kline` 适配后的页面数据，以及 `/api/hqchart/history` 返回的 HQChart 原生 K 线数组 |
| 公式管理 | `formulas` 表，保存名称、类型、脚本、周期、复权参数 |
| 自动化任务 | `automation_tasks` 表，保存 Cron、任务类型、payload、Webhook 关联 |

## 公式和选股

Docker 镜像会从随项目包含的 hqchartPy2 源码编译 `HQChartPy2` 扩展。公式 worker 启动后会优先使用 HQChartPy2 的 C++ 引擎执行公式；如果模块不可用或单次执行失败，会回退到内置 Python 执行器，继续保证本地闭环。

内置 fallback 执行器支持常见函数：`MA`、`EMA`、`SMA`、`REF`、`LLV`、`HHV`、`CROSS`、`SUM`、`STD`、`IF`、`MAX`、`MIN`。

示例公式：

```text
CROSS(MA(C,5),MA(C,20));
```

选股任务需要选择一个公式和一个股票池，可以手动运行，也可以填写六段 Cron 表达式定时运行，例如 `0 30 21 * * *`。

命中的股票会写入选股结果中心，不需要再从运行记录的 JSON 里翻结果。

选股任务会按 `batch_size` 分批执行，默认 50 只一批；`continue_on_error=true` 时，单批失败会降级为逐只执行并记录失败股票，不会轻易中断整个任务。

## 系统任务

自动化页面提供系统任务模板：

- 早盘基础同步：同步代码库和交易日。
- 晚盘日 K 同步：同步日 K 数据。
- 晚盘完整同步：同步基础数据、日 K、除权、板块、行业、统计、新股申购，并按 `max_codes` 抓取财务快照。

模板默认不启用，创建后可以检查 Cron 和 payload，再手动启用。

系统同步支持的常用 `scope`：

```text
basic, codes, workday, kline, gbbq, finance, f10, block, industry, stat, stat2, xgsg, all
```

`finance`、`f10`、`block`、`industry`、`stat`、`stat2`、`xgsg` 会写入 `data/database/snapshots/` 下的 JSON 快照。

自定义任务支持：

```json
{"action":"noop","data":{"note":"仅记录一次运行"}}
```

```json
{"action":"system_sync","sync":{"scope":"block","block_files":["gn"],"with_index":true}}
```

```json
{"action":"http_request","method":"POST","url":"http://127.0.0.1:8080/api/health","body":{}}
```

## Webhook

Webhook 使用 `POST` JSON：

```json
{
  "event": "stock_selection.finished",
  "task_name": "晚间选股",
  "status": "success",
  "matched_count": 3,
  "run_at": "2026-06-06T21:30:00+08:00"
}
```

## 使用建议

- 日/周/月 K 线默认使用同花顺前复权数据。
- Docker 模式会自动启动公式 worker；源码模式需要先运行 `python3 formula-worker/worker.py`。
- 可通过 `/api/formula/health` 查看当前公式引擎，`engine=hqchartpy2` 表示已经使用 HQChartPy2。
- 原始通达信 K 线可调用 `/api/kline-all/tdx`。
- 如果遇到分时为空，先确认查询日期是否为交易日。
- 浏览器控制台报错时，先刷新页面并确认服务仍在运行。

## 相关 API

常用接口见 [API.md](API.md)。新增的财务、F10、板块、扩展行情等能力也已在 API 文档中列出。

# TDX 上游能力补充接口

这些接口用于暴露当前 `tdx` 上游较新的协议能力，均沿用统一响应格式：

```json
{"code": 0, "message": "success", "data": {}}
```

## 标准行情扩展

| 接口 | 说明 | 示例 |
| --- | --- | --- |
| `GET /api/call-auction` | 集合竞价 | `/api/call-auction?code=000001` |
| `GET /api/gbbq` | 股本变迁/除权除息事件 | `/api/gbbq?code=600519` |
| `GET /api/finance` | 财务/基本面信息 | `/api/finance?code=600519` |
| `GET /api/company/categories` | F10 公司资料目录 | `/api/company/categories?code=600519` |
| `GET /api/company/content` | F10 公司资料正文 | `/api/company/content?code=600519&filename=xxx&start=0&length=1024` |

## 板块与配置

| 接口 | 说明 | 参数 |
| --- | --- | --- |
| `GET /api/block` | 板块成分，可选补板块指数代码 | `file=gn/fg/zs/hy/block`，`with_index=true/false` |
| `GET /api/tdx-hy` | 通达信/申万行业归属 | 无 |
| `GET /api/tdx-stat` | 个股综合统计 | 无 |
| `GET /api/tdx-stat2` | 资金流向与板块归属统计 | 无 |
| `GET /api/xgsg` | 新股申购列表 | 无 |

## 扩展行情 TdxExHq

扩展行情使用通达信 7727 端口，适合期货、港股、外盘等市场。

| 接口 | 说明 | 示例 |
| --- | --- | --- |
| `GET /api/exhq/markets` | 扩展市场代码表 | `/api/exhq/markets` |
| `GET /api/exhq/count` | 扩展品种数量 | `/api/exhq/count` |
| `GET /api/exhq/instruments` | 扩展品种列表分页 | `/api/exhq/instruments?start=0&count=100` |
| `GET /api/exhq/quote` | 单品种五档行情 | `/api/exhq/quote?market=31&code=HK00700` |
| `GET /api/exhq/bars` | 扩展 K 线 | `/api/exhq/bars?market=31&code=HK00700&category=9&start=0&count=100` |
| `GET /api/exhq/trade` | 扩展分笔成交 | `/api/exhq/trade?market=31&code=HK00700&start=0&count=200` |

`category` 使用通达信 K 线类型值，例如 `9` 为日线，`5` 为周线，`6` 为月线，`7` 为 1 分钟。

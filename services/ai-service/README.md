# AI 分析服务

AI 分析服务提供统一的模型调用和股票研究分析接口。第一版使用 OpenAI-compatible 适配器，可先接入 DeepSeek，也为 OpenAI、通义千问、智谱 GLM 和自定义兼容接口预留 provider。

## 运行方式

Docker Compose 会单独构建并启动 `services/ai-service`：

```bash
docker compose up -d --build ai-service
```

服务端口：

```text
容器内：8083
宿主机：18083
```

## 环境变量

| 变量 | 说明 |
| --- | --- |
| `AI_CREDENTIAL_SECRET` | 本地加密 AI key 的密钥，建议生产/长期使用时固定设置 |
| `AI_SERVICE_TOKEN` | 可选，服务间访问令牌；Web 代理会自动转发 |
| `DEEPSEEK_API_KEY` | 可选，DeepSeek 环境变量凭据 |
| `DEEPSEEK_BASE_URL` | 可选，默认 `https://api.deepseek.com` |
| `MARKET_SERVICE_URL` | 行情服务地址，用于单股/自选分析补充行情上下文 |

也可以通过接口写入凭据，服务会加密保存到 `data/database/ai.db`，接口只返回脱敏 key。

## 常用接口

```text
GET  /api/ai/providers
GET  /api/ai/credentials
POST /api/ai/credentials
POST /api/ai/credentials/{id}/test
POST /api/ai/test-connection
POST /api/ai/chat
POST /api/ai/analyze/stock
POST /api/ai/analyze/watchlist
```

DeepSeek 连接测试示例：

```bash
curl -X POST http://localhost:18083/api/ai/test-connection \
  -H 'Content-Type: application/json' \
  -d '{"provider":"deepseek","model":"deepseek-chat"}'
```

单股分析示例：

```bash
curl -X POST http://localhost:18083/api/ai/analyze/stock \
  -H 'Content-Type: application/json' \
  -d '{"provider":"deepseek","model":"deepseek-chat","symbol":"603171"}'
```

# Xinghai Router

支持 OpenAI 与 Anthropic 格式的 LLM 网关与运营后台。管理员可管理用户、密钥、渠道、路由和模型价格；用户通过一个 API Key 调用模型，并获取自己的用量、余额和账本。

## Included

- PostgreSQL migrations for users、哈希 API Key、加密渠道凭据、不可变钱包账本、用量、路由和审计记录。
- 基于用户会话、管理员角色和细粒度权限保护的管理 API。
- OpenAI-compatible `GET /v1/models`、`POST /v1/chat/completions`，以及 Anthropic-compatible `POST /v1/messages`。
- 透明 SSE、上游超时、每 Key 每分钟基础限流、请求 ID、安全响应头、请求体大小限制、panic 恢复、模型别名和同优先级权重路由。
- 对可重试上游错误自动切换备用渠道；连续失败三次的渠道冷却一分钟。管理员可在站点设置中开启故障渠道自动检测：系统会重试检测三次，全部失败后自动停用渠道。
- “路由可靠性”分组支持独立的请求重试配置（重试次数 0-10、逗号分隔的状态码与包含性范围）、后台渠道健康检查（定时全量测试或仅被动恢复、可配置频率、渠道 ID 白名单、检查成功后自动恢复上线），以及自动禁用规则（测试失败禁用、慢响应秒数阈值、状态码和上游错误关键字匹配，关键字不区分大小写）。
- 请求日志关联用户、Key、模型和最终渠道；非流式请求记录 token、按定价结算，并在上游调用前预留余额以避免并发透支。
- 支持彩虹易支付兼容接口自助充值；管理员可配置平台并动态管理支付渠道，异步通知验签后幂等入账。
- 套餐订阅系统：管理员可定义按月/年计费的套餐，支持赠送额度、模型白名单与每周期上限；订阅支付复用易支付，激活后自动发放额度并加入分组，订阅期内对白名单内模型的调用跳过钱包扣费。

当前限流器仍是进程内实现，水平扩容前必须替换为 Redis 的滑动窗口。流式响应透明透传；由于不同上游的 SSE 用量事件不一致，流式请求目前不结算 token 费用，仅记录请求日志，不应据此执行生产计费。

## Run locally

1. Create local infrastructure: `docker compose up -d`.
2. Create configuration: `cp .env.example .env`, then replace both secrets with unique random values.
3. Export the environment variables in `.env` using your shell or an environment loader.
4. Run: `go run ./cmd/router`.
5. Check: `curl http://localhost:8080/healthz`.

## Docker deployment

Copy `.env.example` to `.env`, replace `ENCRYPTION_KEY` and `POSTGRES_PASSWORD` with unique URL-safe secrets, then start the complete stack:

```sh
cp .env.example .env
docker compose up -d --build
```

The web console is available at `http://localhost:3000`; the OpenAI/Anthropic gateway is available at `http://localhost:8080`. PostgreSQL and Redis are internal-only and persist data in the `postgres-data` and `redis-data` volumes. Migrations run automatically when the router starts. Follow startup logs with `docker compose logs -f router` and stop the stack with `docker compose down` (use `docker compose down -v` only when intentionally deleting data).

### Admin web console

The Vue 3 management console is in `web/`. Start the Go service first, then run:

```sh
cd web
npm install
npm run dev
```

Open `http://localhost:5173/auth` and create an account or sign in with email and password. The first registered account becomes an administrator; administrators can promote users or grant individual permissions. Browser sessions are retained only in session storage. Nuxt proxies browser requests from `/api/*` to `http://127.0.0.1:8080/*`, so this development setup does not require a CORS policy. `npm run generate` emits prerendered HTML for the public home and authentication pages; deploy the Nuxt `.output` directory for the full application.

The service performs migrations automatically at startup. `base_url` for a channel must be an HTTPS origin or path prefix without `/v1`; for example, `https://api.openai.com`. Loopback HTTP URLs are also accepted for local services such as Ollama, for example `http://127.0.0.1:11434`. Provider secrets are encrypted in the database using `ENCRYPTION_KEY`, so keep this value stable and securely backed up.

### 易支付充值

本项目使用彩虹易支付兼容的页面支付协议。在线充值是可选功能，由具有 `system.manage` 权限的管理员在控制台“支付设置”页面配置，不使用支付环境变量。管理员可以设置启用状态、易支付平台地址、控制台公网地址、商户 ID 和商户密钥；商户密钥使用 `ENCRYPTION_KEY` 加密保存，不会通过查询 API 返回。

管理员还可以自行新增、修改、停用和删除支付渠道。渠道代码会原样作为易支付的 `type` 参数提交，例如可添加 `alipay / 支付宝`、`wxpay / 微信支付`，具体代码以使用的易支付平台文档为准。删除渠道不会删除已产生的支付订单。

假设控制台公网地址配置为 `https://router.example.com`，易支付商户后台应允许服务端通知地址：

```text
https://router.example.com/api/payments/epay/notify
```

Nuxt 会将 `/api/*` 转发给 Go 服务。若绕过 Nuxt 直接暴露 Go 服务，则通知路径为 `/payments/epay/notify`。生产环境的平台地址和控制台公网地址必须使用 HTTPS；只有 `localhost` 和 `127.0.0.1` 可使用 HTTP。

支付管理 API 为 `GET|PUT /admin/payment-settings`、`POST /admin/payment-methods`、`PUT /admin/payment-methods/{id}` 和 `DELETE /admin/payment-methods/{id}`，均要求 `system.manage` 权限。

用户可在钱包页面发起充值，也可使用账户 API：

```sh
curl -X POST http://localhost:8080/account/payments \
  -H "Authorization: Bearer $SESSION_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"amount":"10.00","type":"alipay"}'
```

响应中的 `pay_url` 用于跳转易支付收银台。可通过 `GET /account/payments` 查询最近订单，或通过 `GET /account/payments/{order_no}` 查询单笔状态。浏览器同步返回不会触发入账；只有签名、商户 ID、成功状态和订单金额均通过校验的异步通知才会入账。重复通知会返回 `success`，但不会重复增加余额。

## 订阅系统

管理员可在“订阅套餐”页面定义按月或按年计费的套餐：套餐价格、计费周期、赠送额度（订阅激活时一次性充值进用户钱包，可叠加现有按量计费）、自动加入分组、模型白名单（留空表示订阅期内可调用全部模型）、每周期最大请求数/Token 数，以及对外展示顺序与启用状态。

用户在“订阅”页面选择套餐后跳转易支付收银台，异步通知通过签名、商户 ID、金额校验后激活订阅：设置当前周期起止时间（月套餐 1 个月、年套餐 1 年），将赠送额度充值进钱包并写入账本（`subscription_topup`），如套餐绑定了分组则将用户加入该分组；同一订单重复通知幂等返回 `success` 但不重复发放。订阅期内，对套餐白名单内（或白名单为空时的全部模型）的调用会跳过钱包结算（订阅期内免费），并在达到套餐每周期上限时回退到按量计费。

订阅相关 API：

```sh
# 浏览对外可见的套餐（无需登录）
curl http://localhost:8080/subscription-plans

# 用户订阅当前套餐
curl -X POST http://localhost:8080/account/subscriptions \
  -H "Authorization: Bearer $SESSION_TOKEN" -H 'Content-Type: application/json' \
  -d '{"plan_id":"PLAN_UUID","payment_type":"alipay","auto_renew":false}'

# 查询我的订阅与订单
curl http://localhost:8080/account/subscriptions -H "Authorization: Bearer $SESSION_TOKEN"
curl http://localhost:8080/account/subscription-orders -H "Authorization: Bearer $SESSION_TOKEN"
# 取消订阅（不再续费，当前周期内仍有效）
curl -X POST http://localhost:8080/account/subscriptions/{id}/cancel -H "Authorization: Bearer $SESSION_TOKEN"
```

管理员 API：`GET /admin/subscription-plans`、`POST /admin/subscription-plans`、`PUT /admin/subscription-plans/{id}`、`DELETE /admin/subscription-plans/{id}`（`system.manage`），以及 `GET /admin/subscriptions`（`users.read`）查看全站订阅。套餐请求体示例：

```json
{
  "name": "标准月度",
  "description": "适合个人开发者",
  "price": "29.00",
  "currency": "CNY",
  "billing_period": "month",
  "credit_amount": "30",
  "group_id": "",
  "model_whitelist": [],
  "max_requests_per_period": null,
  "max_tokens_per_period": null,
  "sort_order": 10,
  "enabled": true
}
```

## Administration API

All `/admin` endpoints require an authenticated account session: `Authorization: Bearer $SESSION_TOKEN`. An `admin` user has every permission. Other users must be individually granted the permission required by each endpoint.

| Permission | Access |
| --- | --- |
| `users.read` | List users |
| `keys.manage` | Create, list, and revoke API keys |
| `channels.read`, `channels.manage` | View or manage upstream channels |
| `logs.read`, `audit.read` | View request or audit logs |
| `pricing.read`, `pricing.manage` | View or edit pricing |
| `wallets.manage`, `routes.manage`, `quotas.manage` | Manage balances, model routes, or quotas |
| `system.manage` | Promote users and assign permissions |

Promote a user or grant permissions using `POST /admin/users/{id}/role` with `{"role":"admin"}`, and `PUT /admin/users/{id}/permissions` with `{"permissions":["channels.read","logs.read"]}`. These operations require `system.manage`.

## Account API

Register an account. Passwords must have at least eight characters; the service stores only bcrypt password hashes. A successful registration or login returns a seven-day bearer session token.

```sh
curl -X POST http://localhost:8080/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","name":"Example User","password":"a-strong-password"}'
```

Log in with `POST /auth/login` using `{"email":"user@example.com","password":"a-strong-password"}`. Use the returned token with `Authorization: Bearer $SESSION_TOKEN` for `GET /account/me`, and revoke the current session using `POST /auth/logout`.

Create an API key. The full `key` in the response is displayed only at creation time:

```sh
curl -X POST http://localhost:8080/admin/keys \
  -H "Authorization: Bearer $SESSION_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"user_id":"USER_UUID","name":"development"}'
```

Create an OpenAI-compatible upstream channel:

```sh
curl -X POST http://localhost:8080/admin/channels \
  -H "Authorization: Bearer $SESSION_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"openai","base_url":"https://api.openai.com","api_key":"PROVIDER_KEY","models":["kimi-k3-mini"],"priority":100}'
```

创建渠道时可选 `provider`：`openai`、`ollama`、`kimi`、`opencode_go` 或 `anthropic`。Ollama、Kimi 和 OpenCode Go 使用各自的 OpenAI-compatible 接口；Anthropic 渠道会转换为 Messages API。`base_url` 不要包含末尾的 `/v1`：

```sh
# 本机 Ollama，API Key 会被 Ollama 忽略
curl -X POST http://localhost:8080/admin/channels \
  -H "Authorization: Bearer $SESSION_TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"ollama","provider":"ollama","base_url":"http://127.0.0.1:11434","api_key":"ollama","models":["qwen3-coder:30b"],"priority":100}'

# Kimi / Moonshot
curl -X POST http://localhost:8080/admin/channels \
  -H "Authorization: Bearer $SESSION_TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"kimi","provider":"kimi","base_url":"https://api.moonshot.cn","api_key":"MOONSHOT_API_KEY","models":["kimi-k2.6"],"priority":100}'
```

OpenCode Go 使用相同方式创建渠道并设置 `"provider":"opencode_go"`，填写其 OpenAI-compatible API origin、订阅 API Key 和可用模型 ID。Anthropic 上游使用 `"provider":"anthropic"`、`https://api.anthropic.com` 和 Anthropic API Key。

List management data with `GET /admin/users`, `GET /admin/keys`, `GET /admin/channels`, `GET /admin/request-logs`, `GET /admin/pricing`, and `GET /admin/audit-logs`. Revoke a user key with `POST /admin/keys/{id}/revoke`; enable or disable a channel with `POST /admin/channels/{id}/status` and `{"enabled":true}` or `{"enabled":false}`.

Set a model price (currency units per million tokens), then top up or adjust a user's balance:

```sh
curl -X POST http://localhost:8080/admin/pricing \
  -H "Authorization: Bearer $SESSION_TOKEN" -H 'Content-Type: application/json' \
  -d '{"model":"kimi-k3-mini","input_per_million":0.15,"cached_input_per_million":0.075,"output_per_million":0.60,"multiplier":1}'

# 从 NewAPI 同步 token 模型定价；price_per_quota_unit 是 quota_per_unit 配额对应的本地货币价格
curl -X POST http://localhost:8080/admin/pricing/newapi/sync \
  -H "Authorization: Bearer $SESSION_TOKEN" -H 'Content-Type: application/json' \
  -d '{"base_url":"https://newapi.example.com","api_key":"NEWAPI_SESSION_OR_TOKEN","price_per_quota_unit":0.000002}'

curl -X POST http://localhost:8080/admin/wallets/adjustments \
  -H "Authorization: Bearer $SESSION_TOKEN" -H 'Content-Type: application/json' \
  -d '{"user_id":"USER_UUID","amount":10,"note":"initial credit"}'
```

同步会读取 NewAPI 的 `/api/status` 与 `/api/pricing`，将 token 计费模型的 `model_ratio`、`completion_ratio` 和 `cache_ratio` 换算为每百万 token 的本地价格。`price_per_quota_unit` 是 `quota_per_unit` 个 NewAPI 配额的本地货币价格，例如 NewAPI 的 `quota_per_unit` 为 500,000，且 500,000 配额兑换 1 元时填 `1`。按次计费模型不会被导入；已存在规则的结算倍率保持不变。

Create a public-model alias for a specific channel, or apply a request quota to a user/API key:

```sh
curl -X POST http://localhost:8080/admin/model-routes \
  -H "Authorization: Bearer $SESSION_TOKEN" -H 'Content-Type: application/json' \
  -d '{"public_model":"kimi-k3","upstream_model":"provider-kimi-k3","channel_id":"CHANNEL_UUID","priority":10,"weight":100}'

curl -X POST http://localhost:8080/admin/quota-limits \
  -H "Authorization: Bearer $SESSION_TOKEN" -H 'Content-Type: application/json' \
  -d '{"user_id":"USER_UUID","window":"day","max_requests":1000}'
```

## Gateway API

Call the gateway with the API key returned by `/admin/keys`:

```sh
curl http://localhost:8080/v1/models -H "Authorization: Bearer $XINGHAI_API_KEY"

curl -N http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer $XINGHAI_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"model":"kimi-k3-mini","messages":[{"role":"user","content":"Hello"}],"stream":true}'
```

Anthropic 客户端（包括将 OpenCode 的 Anthropic provider 指向本服务）可使用 `x-api-key` 调用 `/v1/messages`。请求、非流式响应、SSE 和工具调用会在 Anthropic Messages 与上游 OpenAI Chat Completions 格式之间转换：

```sh
curl -N http://localhost:8080/v1/messages \
  -H "x-api-key: $XINGHAI_API_KEY" \
  -H 'anthropic-version: 2023-06-01' \
  -H 'Content-Type: application/json' \
  -d '{"model":"kimi-k2.6","max_tokens":1024,"messages":[{"role":"user","content":"Hello"}],"stream":true}'
```

OpenCode 配置示例：

```json
{
  "$schema": "https://opencode.ai/config.json",
  "provider": {
    "xinghai": {
      "npm": "@ai-sdk/anthropic",
      "name": "Xinghai Router",
      "options": { "baseURL": "http://localhost:8080/v1", "apiKey": "sk-xh-your-key" },
      "models": { "kimi-k2.6": { "name": "Kimi K2.6" } }
    }
  }
}
```

The router selects an enabled channel advertising the requested model. It tries the lowest numeric priority first, distributes equal-priority traffic by weight, and retries a different eligible channel for connection errors and responses matching the configured retry status codes (by default every status except `2xx`, `408`, and `504`, up to 3 retries). Upstream errors matching the configured auto-disable status codes or keywords disable the channel immediately, and the optional background health check probes channels on a schedule (`scheduled_all`) or only after automatic disabling (`passive_recovery`), bringing recovered channels back online when configured. Manage these options through `GET|PUT /admin/reliability-settings` (`system.manage`).

## Verify

Run `go test ./...` and `go vet ./...`.

# Xinghai Router

支持 OpenAI、Anthropic 与 JEV / TypeSafe System One 格式的 LLM 网关与运营后台。管理员可管理用户、密钥、渠道、路由和模型价格；用户通过一个 API Key 调用模型，并获取自己的用量、余额和账本。

## Included

- PostgreSQL migrations for users、哈希 API Key、加密渠道凭据、不可变钱包账本、用量、路由和审计记录。
- 基于用户会话、管理员角色和细粒度权限保护的管理 API。
- OpenAI-compatible `GET /v1/models`、`POST /v1/chat/completions`、`POST /v1/responses`，Anthropic-compatible `POST /v1/messages`，以及 TypeSafe-compatible `POST /v1/systemone`。
- 透明 SSE、每 Key 每分钟基础限流、请求 ID、安全响应头、panic 恢复、模型别名和同优先级权重路由。
- 对可重试上游错误自动切换备用渠道；连续失败三次的渠道冷却一分钟。管理员可在站点设置中开启故障渠道自动检测：系统会重试检测三次，全部失败后自动停用渠道。
- “路由可靠性”分组支持独立的请求重试配置（重试次数 0-10、逗号分隔的状态码与包含性范围）、后台渠道健康检查（定时全量测试或仅被动恢复、可配置频率、渠道 ID 白名单、检查成功后自动恢复上线），以及自动禁用规则（测试失败禁用、慢响应秒数阈值、状态码和上游错误关键字匹配，关键字不区分大小写）。
- 请求日志关联用户、Key、模型和最终渠道；非流式请求记录 token、按定价结算，并在上游调用前预留余额以避免并发透支。
- 支持彩虹易支付兼容接口自助充值；管理员可配置平台并动态管理支付渠道，异步通知验签后幂等入账。
- 套餐订阅系统：管理员可定义按月/年计费的套餐，支持赠送额度、模型白名单与每周期上限；订阅支付复用易支付，激活后自动发放额度并加入分组，订阅期内对白名单内模型的调用跳过钱包扣费。

设置 `REDIS_URL` 后，API Key 限流使用 Redis 固定窗口计数（每 Key 每分钟）。默认 `DEPLOYMENT_MODE=single`，用户/分组并发只在单进程内计数，Redis 故障默认回退内存；这不等于多实例行为一致。多副本必须显式使用 `DEPLOYMENT_MODE=cluster`，详见下文的共享租约、配置通知和故障策略。流式响应解析上游 SSE 用量并结算；完全缺少用量的成功文本流保留请求估算兜底，并标记 `usage_source=request_estimate`，明确返回零用量则不再替换成估算。多维计量、图片计费配置与账单复算规则见 [统一用量与计费](docs/billing.md)。

## Run locally

1. Create local infrastructure: `docker compose up -d`.
2. Create configuration: `cp .env.example .env`, then replace both secrets with unique random values.
3. Export the environment variables in `.env` using your shell or an environment loader.
4. Run: `go run ./cmd/router`.
5. Check: `curl http://localhost:8080/healthz`.

## Docker deployment

Copy `.env.example` to `.env`, set unique values for `ENCRYPTION_KEY` (≥24 chars, not a docs placeholder), `POSTGRES_PASSWORD` (URL-safe), and `REDIS_PASSWORD`, then start the complete stack:

```sh
cp .env.example .env
# edit .env — compose refuses to start if required secrets are missing
docker compose up -d --build
```

The web console is available at `http://localhost:3000`; the OpenAI/Anthropic gateway is available at `http://localhost:8080`. PostgreSQL and Redis are internal-only (no published ports). Data paths default to `/mnt/data/AI-Router/{postgres,redis}` and can be overridden with `POSTGRES_DATA_PATH` / `REDIS_DATA_PATH`. Redis requires a password (`REDIS_PASSWORD`); the router receives `REDIS_URL=redis://:${REDIS_PASSWORD}@redis:6379/0` by default. Migrations run automatically when the router starts. User id 1 is reserved for the administrator; claim it by setting `BOOTSTRAP_ADMIN_EMAIL` (and optionally `BOOTSTRAP_ADMIN_PASSWORD`) in `.env` before the first start. Self-registrations always create normal user accounts and can never claim admin.

For an external reverse-proxy network (e.g. Baota `baota_net`):

```sh
docker network create baota_net   # once
docker compose -f docker-compose.yml -f docker-compose.baota.yml up -d --build
```

Stop the stack with `docker compose down` (use `docker compose down -v` only when intentionally deleting named volumes).

### 单实例与多副本部署边界

| 配置 | 单实例 `single`（默认） | 集群 `cluster` |
| --- | --- | --- |
| 用户/分组并发 | 进程内计数，副本之间不共享 | Redis 原子获取用户与分组租约，全副本共享 |
| Redis 故障策略 | 默认 `memory` 保可用；可选 `deny` | 必须 `deny`，不允许悄悄切换本地计数 |
| 配置缓存失效 | 本地失效 + TTL | PostgreSQL 事务提交通知 + 重连清空 + TTL |
| 本地 prompt 折扣缓存 | 可选 | 必须关闭，避免副本命中差异影响计费 |

集群模式示例（所有副本使用相同数据库、Redis 数据库、密钥和模式配置）：

```dotenv
DEPLOYMENT_MODE=cluster
REDIS_URL=redis://:your-secret@your-redis:6379/0
REDIS_FAILURE_POLICY=deny
CONCURRENCY_LEASE_TTL=30s
LOCAL_PROMPT_CACHE=false
```

- `CONCURRENCY_LEASE_TTL` 支持 `5s`–`5m`，默认 `30s`。每个请求使用唯一令牌，在一个 Lua 操作中检查并占用用户/分组槽位，以 Redis 时间计算过期；每 TTL/3 续租，完成后按令牌幂等释放。崩溃或释放失败的槽位在租约过期后回收；降低上限不驱逐现有请求，但阻止继续超额准入。集群模式每次准入从 PostgreSQL 读取并发上限，读取失败返回 503，不把故障解释为无限制。
- `deny` 优先保障共享限额：Redis 启动检查失败即拒绝启动；运行时 API Key 限流或并发后端故障返回 503，确实达到限制才返回 429。续租失败或租约丢失会取消正在进行的上游请求；已发送响应头的流式请求只能中断流，不能再改写为 503。`/readyz` 在此策略下同时检查 PostgreSQL 和 Redis；`/healthz` 仍是进程存活检查。
- `memory` 只用于单实例可用性优先场景：故障时使用独立的本地固定窗口，恢复后重新连接 Redis。两套窗口不合并，故障/恢复切换会改变限额语义，不能当作严格全局额度。日志 `event=redis_degraded` / `event=redis_recovered` 标明组件和策略，按状态变化发出；应在日志平台配置告警，日志本身不是外部告警投递。`event=concurrency_lease_lost` 表示在途请求因失去租约被取消。
- 迁移 `098_config_invalidation.sql` 为定价、渠道与密钥、路由/分组、并发上限、配额定义、站点/内容策略及订阅配置安装事务通知触发器。回滚不发送通知；各集群副本使用专用 PostgreSQL 连接监听，通知到达时清空相关缓存，重连成功后再次清空，加载中的旧数据不会重新填回已失效缓存。通知不包含配置值或秘密。数据库代理必须支持会话级 `LISTEN`（不能使用事务池模式），每副本额外预留一个连接。
- **配置通知是最终一致，不是跨副本同步屏障。** 通知处理前、网络断开或加载已经在途时仍可能读到旧配置；现有缓存 TTL（通常 5–30 秒）作为兜底，TTL 从加载完成时起算，嵌套的渠道列表/密钥缓存可能叠加到约 60 秒再加查询耗时，不承诺自提交起严格固定延迟。停用渠道、改价等需要立即一致的操作，应先排空流量并等待所有副本刷新，再恢复接入。静态环境变量不经通知更新，须一致地重启副本。
- **租约不是上游执行的 fencing 保证。** Redis 丢失数据、异步复制故障切换、管理员清空/驱逐键、进程长暂停或上游忽略取消，都可能造成短暂重叠执行。Redis 应使用独立数据库、`noeviction` 与适当持久化/高可用策略；丢失限流状态后先停止准入并排空旧请求，再恢复流量。客户端使用一个 Redis endpoint，不自动发现 Sentinel，也不处理 Redis Cluster 的 MOVED/ASK 跳转；这里的 `cluster` 指应用多副本，不表示支持 Redis Cluster 拓扑。
- 每个受限在途请求占用一个 Redis 连接，应为最大并发预留连接/文件描述符并压测。未设并发上限的请求不创建租约。不要混跑 `single` 与 `cluster`，升级时先排空旧实例；不同项目应使用不同 Redis 数据库，避免固定命名空间冲突。
- 本地会话 JSON 文件、公开榜单/性能聚合、后台定时任务、订阅与渠道软额度仍有各自的多实例边界；本次不承诺调度器单主、文件跨副本可见或所有软额度都变为严格额度。现有 Compose 是单机启动模板（固定宿主端口），不是直接 `--scale` 的生产集群编排。生产扩容仍需负载均衡、共享数据方案与独立压测验收。

可用独立测试基础设施验证（`TEST_DATABASE_URL` 对应数据库会被集成测试清空，切勿指向业务库）：

```sh
go build ./...
go vet ./...
go test ./...
TEST_REDIS_URL=redis://127.0.0.1:6379/15 go test ./internal/app -run 'TestConcurrencyLease' -count=1
TEST_DATABASE_URL='postgres://test:password@localhost:5432/router_test?sslmode=disable' go test -tags integration ./internal/app -run 'TestIntegration.*ConfigInvalidation' -count=1
```

### Admin web console

The Vue 3 management console is in `web/`. Start the Go service first, then run:

```sh
cd web
npm install
npm run dev
```

Open `http://localhost:5173/auth` and create an account or sign in with email and password. The first registered account is a normal user; the administrator is bootstrapped from `BOOTSTRAP_ADMIN_EMAIL`/`BOOTSTRAP_ADMIN_PASSWORD` in the environment (see `.env.example`). Later public registrations create normal user accounts; administrators can promote users or grant individual permissions. Browser sessions use server-managed seven-day HttpOnly cookies (Secure by default); JavaScript never receives the session credential. Local HTTP development requires explicitly setting `SESSION_COOKIE_SECURE=false` on the router. Upgrades revoke old browser sessions once; see [security migration](SECURITY_MIGRATION.md). Nuxt proxies browser requests from `/api/*` to `http://127.0.0.1:8080/*`, so this development setup does not require a CORS policy. `npm run generate` emits prerendered HTML for the public home and authentication pages; deploy the Nuxt `.output` directory for the full application.

The service performs migrations automatically at startup. `base_url` for a channel must be an HTTPS origin or path prefix without `/v1`; for example, `https://api.openai.com`. Loopback HTTP URLs are also accepted for local services such as Ollama, for example `http://127.0.0.1:11434`. Channel credential storage remains `plaintext` by default for operational compatibility. Opt in with `CHANNEL_CREDENTIAL_STORAGE=encrypted` and explicitly migrate existing rows as described in [SECURITY_MIGRATION.md](SECURITY_MIGRATION.md). Encrypted channel, merchant and service secrets depend on `ENCRYPTION_KEY`; keep it stable and securely backed up.

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
  -b cookies.txt -c cookies.txt -H 'X-Xinghai-Request: 1' \
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
  -b cookies.txt -c cookies.txt -H 'X-Xinghai-Request: 1' -H 'Content-Type: application/json' \
  -d '{"plan_id":"PLAN_UUID","payment_type":"alipay","auto_renew":false}'

# 查询我的订阅与订单
curl http://localhost:8080/account/subscriptions -b cookies.txt -c cookies.txt -H 'X-Xinghai-Request: 1'
curl http://localhost:8080/account/subscription-orders -b cookies.txt -c cookies.txt -H 'X-Xinghai-Request: 1'
# 取消订阅（不再续费，当前周期内仍有效）
curl -X POST http://localhost:8080/account/subscriptions/{id}/cancel -b cookies.txt -c cookies.txt -H 'X-Xinghai-Request: 1'
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
  "max_credit_per_period": null,
  "overage_policy": "allow_wallet",
  "model_quotas": [],
  "sort_order": 10,
  "enabled": true
}
```

`max_requests_per_period` 限制每个计费周期的请求数；`max_credit_per_period` 按成本限制周期内订阅覆盖请求的总消耗（数值来自定价规则）。`overage_policy` 为 `allow_wallet`（默认，额度耗尽后按钱包计费）或 `block`（耗尽后拒绝请求，返回 402 `subscription_quota_exceeded`）。`model_quotas` 可按模型单独设置 `max_requests_per_period` / `max_credit_per_period`，未填写的维度继承套餐全局值。

## Administration API

All `/admin` endpoints require an authenticated account session cookie. Console writes also require `X-Xinghai-Request: 1`; use a private curl cookie jar (`-b cookies.txt -c cookies.txt`), not an account Bearer token. An `admin` user has every permission. Other users must be individually granted the permission required by each endpoint, except authorization changes remain admin-only.

| Permission | Access |
| --- | --- |
| `users.read` | List users |
| `users.manage` | Manage ordinary user profiles, not roles or permissions |
| `users.authorize` | Change other users' roles/permissions; admin-only, never self-service |
| `keys.manage` | Create, list, and revoke API keys |
| `channels.read`, `channels.manage` | View or manage upstream channels |
| `logs.read`, `audit.read` | View request or audit logs |
| `pricing.read`, `pricing.manage` | View or edit pricing |
| `wallets.manage`, `routes.manage`, `quotas.manage` | Manage balances, model routes, or quotas |
| `system.manage` | Manage system settings, plans and groups; not authorization |

Promote a user or grant permissions using `POST /admin/users/{id}/role` with `{"role":"admin"}`, and `PUT /admin/users/{id}/permissions` with `{"permissions":["channels.read","logs.read"]}`. These operations require `users.authorize`, an admin actor, a different target account and password reauthentication within five minutes. `users.manage` does not grant authorization authority, and profile edits cannot be used by ordinary managers to take over privileged accounts.

## Account API

Register an account. Passwords must have at least eight characters; the service stores only bcrypt password hashes. A successful registration or login sets a seven-day HttpOnly session cookie and returns only its expiry. The default Secure flag requires HTTPS; set `SESSION_COOKIE_SECURE=false` only for local HTTP development.

```sh
umask 077
curl -c cookies.txt -X POST http://localhost:8080/auth/register \
  -H 'X-Xinghai-Request: 1' -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","name":"Example User","password":"a-strong-password"}'
```

Log in with `POST /auth/login` using `{"email":"user@example.com","password":"a-strong-password"}` and `X-Xinghai-Request: 1`, saving `Set-Cookie` in a private cookie jar. Use that cookie for `GET /account/me`, and revoke the current session using `POST /auth/logout`. Before revealing stored keys or changing users, call `POST /auth/reauthenticate` with `{"password":"a-strong-password"}` and save its rotated cookie (`-b cookies.txt -c cookies.txt`); verification lasts five minutes. OAuth-only users need a local password (via verified-email password reset) before these sensitive operations.

Create an API key. The full `key` in the response is displayed only at creation time:

```sh
curl -X POST http://localhost:8080/admin/keys \
  -b cookies.txt -c cookies.txt -H 'X-Xinghai-Request: 1' \
  -H 'Content-Type: application/json' \
  -d '{"user_id":"USER_UUID","name":"development"}'
```

Create an OpenAI-compatible upstream channel:

```sh
curl -X POST http://localhost:8080/admin/channels \
  -b cookies.txt -c cookies.txt -H 'X-Xinghai-Request: 1' \
  -H 'Content-Type: application/json' \
  -d '{"name":"openai","base_url":"https://api.openai.com","api_key":"PROVIDER_KEY","models":["kimi-k3-mini"],"priority":100}'
```

创建渠道时可选 `provider`：`openai`、`ollama`、`kimi`、`opencode_go`、`anthropic` 或 `jev` 等。Ollama、Kimi 和 OpenCode Go 使用各自的 OpenAI-compatible 接口；Anthropic 渠道会转换为 Messages API。JEV / TypeSafe 可直接使用 `provider:"jev"`，省略 `upstream_format`（管理后台保留“自动”）即可使用 JEV 协议，无需选择 Custom。旧的 `provider:"custom"` + `upstream_format:"jev"` 配置仍然兼容。`base_url` 不要包含末尾的 `/v1`：

```sh
# 本机 Ollama，API Key 会被 Ollama 忽略
curl -X POST http://localhost:8080/admin/channels \
  -b cookies.txt -c cookies.txt -H 'X-Xinghai-Request: 1' -H 'Content-Type: application/json' \
  -d '{"name":"ollama","provider":"ollama","base_url":"http://127.0.0.1:11434","api_key":"ollama","models":["qwen3-coder:30b"],"priority":100}'

# Kimi / Moonshot
curl -X POST http://localhost:8080/admin/channels \
  -b cookies.txt -c cookies.txt -H 'X-Xinghai-Request: 1' -H 'Content-Type: application/json' \
  -d '{"name":"kimi","provider":"kimi","base_url":"https://api.moonshot.cn","api_key":"MOONSHOT_API_KEY","models":["kimi-k2.6"],"priority":100}'
```

OpenCode Go 使用相同方式创建渠道并设置 `"provider":"opencode_go"`，填写其 OpenAI-compatible API origin、订阅 API Key 和可用模型 ID。Anthropic 上游使用 `"provider":"anthropic"`、`https://api.anthropic.com` 和 Anthropic API Key。TypeSafe JEV 渠道示例：

```sh
curl -X POST http://localhost:8080/admin/channels \
  -b cookies.txt -c cookies.txt -H 'X-Xinghai-Request: 1' -H 'Content-Type: application/json' \
  -d '{"name":"typesafe-jev","provider":"jev","base_url":"https://api.typesafe.ai","api_keys":"TYPESAFE_API_KEY","models":["jev-latest"],"test_model":"jev-latest","priority":100}'
```

JEV 用户端调用 `POST /v1/systemone`，使用网关 API Key 的 `Authorization: Bearer` 认证。请求体保持 TypeSafe System One JSON：`model`、`state` 和 `questions`；`questions` 可混合 `noul`、`choice`、`score`。网关只选择有效上游格式为 JEV 的渠道（`provider:"jev"` 默认使用 JEV，也兼容显式 `upstream_format:"jev"`），转发到 `/v1/systemone`，使用上游 Bearer Key，并原样返回同步 JSON 响应和上游错误（包括状态码和 `Retry-After`）。不做 OpenAI / Anthropic 与 JEV 的语义转换，其他协议的请求也不会选择 JEV 渠道。

配置与兼容说明：

- 创建渠道后，须关联用户 API Key 使用的分组（或设置个人渠道归属），配置 `jev-latest` 的启用定价规则，并确保余额或订阅额度可用；只创建渠道并不会绕过网关的路由、鉴权与计费检查。
- 管理后台直接选择 JEV / TypeSafe 提供商，空白接口地址会自动填入 `https://api.typesafe.ai`；上游格式保留“自动”即可，无需选择 Custom 或手动切换格式。“拉取模型”可读取 TypeSafe 原生模型列表；填写 `test_model` 后渠道测试会发送真实 JEV 请求。`upstream_path` 可覆盖默认路径，模型映射与请求参数覆写仍生效，JSON 数值精度保持不变。修改必填字段的覆写配置可能被上游拒绝。
- 以当前 [TypeSafe HTTP API](https://docs.typesafe.ai/api) 和 [服务端 OpenAPI](https://api.typesafe.ai/openapi.json) 为准：`model`、非 null 的 `state`（字符串、对象或数组）与非空 `questions` 必填；SDK 的默认模型在 SDK 端处理。Score 的 `criteria` 为非空数组且元素不能为 null；建议按官方示例使用至少两级评分。
- 官方 JEV 不提供 SSE 流式协议。省略 `stream` 或设为 `false`；其他值返回 422，转发时移除该字段。JEV 请求体上限 2 MiB，超限返回 413，参数校验失败返回网关 422 错误。
- 重试、Key 轮换、自动禁用、配额与内容审查复用网关配置，不额外套用 TypeSafe SDK 的重试策略；按 `usage.input_tokens` / `usage.output_tokens` 结算。客户端应设置请求超时；本次没有修改现有的全局超时策略。
- 本次支持 TypeSafe SDK 的 System One 调用，但网关公共 `GET /v1/models` 仍是 OpenAI 列表格式；不能直接用于 TypeSafe SDK 的原生 `models.list()`。可手动指定渠道配置的模型名。

```sh
curl http://localhost:8080/v1/systemone \
  -H "Authorization: Bearer $XINGHAI_API_KEY" -H 'Content-Type: application/json' \
  -d '{"model":"jev-latest","state":"I was charged twice","questions":{"refund":{"type":"noul","instructions":"Does the user request a refund?"},"department":{"type":"choice","instructions":"Which department?","criteria":{"billing":"Payments and refunds","technical":"Software bugs"}},"urgency":{"type":"score","instructions":"How urgent?","criteria":["Routine","Urgent","Emergency"]}}}'
```

List management data with `GET /admin/users`, `GET /admin/keys`, `GET /admin/channels`, `GET /admin/request-logs`, `GET /admin/pricing`, and `GET /admin/audit-logs`. Revoke a user key with `POST /admin/keys/{id}/revoke`; enable or disable a channel with `POST /admin/channels/{id}/status` and `{"enabled":true}` or `{"enabled":false}`.

Set a model price (currency units per million tokens), then top up or adjust a user's balance:

```sh
curl -X POST http://localhost:8080/admin/pricing \
  -b cookies.txt -c cookies.txt -H 'X-Xinghai-Request: 1' -H 'Content-Type: application/json' \
  -d '{"model":"kimi-k3-mini","input_per_million":0.15,"cached_input_per_million":0.075,"output_per_million":0.60,"multiplier":1}'

# 从 NewAPI 同步 token 模型定价；price_per_quota_unit 是 quota_per_unit 配额对应的本地货币价格
curl -X POST http://localhost:8080/admin/pricing/newapi/sync \
  -b cookies.txt -c cookies.txt -H 'X-Xinghai-Request: 1' -H 'Content-Type: application/json' \
  -d '{"base_url":"https://newapi.example.com","api_key":"NEWAPI_SESSION_OR_TOKEN","price_per_quota_unit":0.000002}'

curl -X POST http://localhost:8080/admin/wallets/adjustments \
  -b cookies.txt -c cookies.txt -H 'X-Xinghai-Request: 1' -H 'Content-Type: application/json' \
  -d '{"user_id":"USER_UUID","amount":10,"note":"initial credit"}'
```

同步会读取 NewAPI 的 `/api/status` 与 `/api/pricing`，将 token 计费模型的 `model_ratio`、`completion_ratio` 和 `cache_ratio` 换算为每百万 token 的本地价格。`price_per_quota_unit` 是 `quota_per_unit` 个 NewAPI 配额的本地货币价格，例如 NewAPI 的 `quota_per_unit` 为 500,000，且 500,000 配额兑换 1 元时填 `1`。按次计费模型不会被导入；已存在规则的结算倍率保持不变。

Create a public-model alias for a specific channel, or apply a request quota to a user/API key:

```sh
curl -X POST http://localhost:8080/admin/model-routes \
  -b cookies.txt -c cookies.txt -H 'X-Xinghai-Request: 1' -H 'Content-Type: application/json' \
  -d '{"public_model":"kimi-k3","upstream_model":"provider-kimi-k3","channel_id":"CHANNEL_UUID","priority":10,"weight":100}'

curl -X POST http://localhost:8080/admin/quota-limits \
  -b cookies.txt -c cookies.txt -H 'X-Xinghai-Request: 1' -H 'Content-Type: application/json' \
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

OpenAI Responses 客户端可调用 `/v1/responses`。`instructions`、字符串或数组形式的 `input`（含 `message`、`function_call`、`function_call_output` 条目）、`tools`、`tool_choice`、`max_output_tokens` 和 `text.format` 会转换为上游 Chat Completions 请求；非流式响应和 SSE 事件（`response.created`、`response.output_text.delta`、`response.function_call_arguments.delta`、`response.completed` 等）会转换回 Responses 格式。Response Lite 使用同一路径并携带 `X-OpenAI-Internal-Codex-Responses-Lite: true`；其请求会自动补齐 `reasoning.context=all_turns` 和 `parallel_tool_calls=false`，不支持的 hosted tool 类型会被拒绝。

Responses 客户端也可以通过 `GET /v1/responses`（或 `/responses`、`/backend-api/codex/responses`）建立 WebSocket。首条消息使用 `{"type":"response.create","model":"MODEL","input":"Hello"}`；后续消息可以省略 `model` 并使用 `previous_response_id`。本系统接收客户端 WebSocket 后通过现有 HTTP/SSE 上游桥接，并直接发送 JSON Responses 事件；`response.append` 不支持。

```sh
curl -N http://localhost:8080/v1/responses \
  -H "Authorization: Bearer $XINGHAI_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"model":"kimi-k3-mini","input":"Hello","instructions":"Be brief","stream":true}'
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

The router selects an enabled channel advertising the requested model. It tries the highest numeric priority first, distributes equal-priority traffic by weight, and retries a different eligible channel for connection errors and responses matching the configured retry status codes (by default every status except `2xx`, `408`, and `504`, up to 3 retries). Upstream errors matching the configured auto-disable status codes or keywords disable the channel immediately, and the optional background health check probes channels on a schedule (`scheduled_all`) or only after automatic disabling (`passive_recovery`), bringing recovered channels back online when configured. Manage these options through `GET|PUT /admin/reliability-settings` (`system.manage`).

## 安全升级

权限拆分、HttpOnly 会话、敏感操作再认证、渠道明文凭据的显式加密迁移，以及文本/图片/WS 资源上限见 [SECURITY_MIGRATION.md](SECURITY_MIGRATION.md)。上线前请确认 HTTPS、重新登录窗口和旧 SQL 运维流程，不能只替换后端二进制而保留旧前端。

## Production checklist

Use this before exposing the stack on a public host.

### Secrets and identity

1. Copy `.env.example` to `.env` and set unique values for `ENCRYPTION_KEY` (≥24 characters, not a documented placeholder), `POSTGRES_PASSWORD` (URL-safe), and `REDIS_PASSWORD`. Compose and the router refuse insecure/missing secrets. **Never rotate `ENCRYPTION_KEY` without re-encrypting provider and payment secrets** — lost keys make ciphertext unrecoverable.
2. Set `BOOTSTRAP_ADMIN_EMAIL` (and ideally `BOOTSTRAP_ADMIN_PASSWORD`) in `.env` before the first start to claim the reserved administrator account. Self-registrations create `role=user` accounts and can never claim admin. Later registrations also create `role=user` accounts unless an administrator promotes them.
3. Prefer enabling Geetest and/or SMTP email verification for public registration (`GEETEST_*`, `SMTP_*` or admin site settings).

### Network and TLS

1. Terminate TLS at a reverse proxy (Nginx, Caddy, cloud LB). Forward `X-Forwarded-Proto: https` so the router can emit HSTS on API responses. Set `TRUSTED_PROXIES` to the proxy CIDRs (or `loopback,private`) so auth rate limits and audit logs use the real client IP from `X-Forwarded-For` / `X-Real-IP`; leave it empty to ignore spoofable proxy headers.
2. Expose only the web console and/or the gateway ports you need. Keep PostgreSQL and Redis off the public network (compose already binds them internally).
3. Channel `base_url` values must be HTTPS (HTTP only for loopback). Payment `base_url` / `public_base_url` must be HTTPS in production.

### Rate limiting and scale

1. Compose requires `REDIS_PASSWORD` and injects `REDIS_URL=redis://:${REDIS_PASSWORD}@redis:6379/0` so API-key and auth rate limits are shared across router replicas. Without Redis the limiter is process-local.
2. Horizontal scaling: run multiple router replicas behind the proxy only when Redis-backed limiting is enabled; wallet reservation already lives in PostgreSQL.
	3. Auth endpoints are rate-limited by IP and email with stricter per-minute budgets than API keys: login 10, register 5, email-code 5 (API-key gateway still uses `RateLimitPerMinute`, default 60).

### Operations

1. Liveness: `GET /healthz` (process up). Readiness: `GET /readyz` (PostgreSQL ping). Compose healthchecks use `/readyz`.
2. Back up PostgreSQL regularly. Redis AOF is enabled in compose for limiter state durability, but Postgres is the source of truth for accounts and billing.
3. After deploy: confirm the bootstrap administrator can sign in, and check `docker compose logs -f router` for migration errors; run `go test ./...` and `go vet ./...` in CI.

### Known production limits

- Streaming (SSE) responses are not settled against the wallet and do not hold wallet `reserved` balance; only non-stream requests reserve and bill tokens. Do not rely on stream traffic for metered revenue until upstream usage events are normalized.
- Browser payment return URLs never credit balances; only the signed `epay/notify` callback does (idempotent on `order_no`).

## Verify

Run `go test ./...` and `go vet ./...`.

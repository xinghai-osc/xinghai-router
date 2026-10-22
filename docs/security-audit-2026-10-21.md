# Xinghai Router 安全审查报告

审查日期：2026-10-21（Asia/Shanghai）。审查基线：`f3d8b30379498c322870c4389af71c8a066eb4df`，以及审查时工作区内的项目源码。

收尾检查发现其他并行工作修改了 [accounts.go](</home/yeyingxingchen/Xinghai/xinghai-router/internal/app/accounts.go>)、[usagelog.go](</home/yeyingxingchen/Xinghai/xinghai-router/internal/app/usagelog.go>) 和 [query_performance_integration_test.go](</home/yeyingxingchen/Xinghai/xinghai-router/internal/app/query_performance_integration_test.go>)，内容涉及模型筛选及通配符转义。已检查差异，不影响本报告列出的认证/配额问题；这些修改不是本次审查产生的，未撤销或覆盖。下列全量构建/测试结果对应其执行时的状态，不作为对后续并行修改的测试背书。

本次确认 **7 项需要处理的安全问题：3 项高危（其中 2 项有明确配置前提），4 项中危**。另外列出已披露的计费边界、依赖漏洞告警和部署加固事项。严重程度按满足所列前提后的影响评估，不表示当前线上部署必然可被利用。

审查覆盖 Go 服务的路由、中间件、认证、OAuth、密码重置、用户授权、workspace、API Key、网关代理、WebSocket、支付、订阅、钱包、奖励、兑换与签到，以及 Nuxt 前端/代理、迁移工具、LLM adapter、Docker/CI 和锁定依赖。采用静态调用链与 SQL 审查、现有测试、受控本地复现和公开漏洞数据库扫描。未访问业务数据库、生产接口或真实上游，未发送邮件，未读取真实环境密钥，未修改业务实现。现有未跟踪文件保持原样。

**主要发现**

| 编号 | 严重程度 | 问题 | 关键前提 | 验证方式 |
| --- | --- | --- | --- | --- |
| S-01 | 高 | 未认证请求在鉴权前被无上限读入内存 | 能访问服务，入口未施加足够的请求体限制 | 真实路由本地复现 |
| S-02 | 高，条件 | 未验证本地邮箱与 OAuth 自动合并导致账户预劫持 | 存在未验证邮箱注册，随后同邮箱首次 OAuth 登录 | 完整注册、OAuth、登录调用链交叉审查 |
| S-03 | 高，条件 | Host 污染密码重置邮件，泄露重置 token | SMTP 开启、可信公开地址缺失、入口允许恶意 Host、用户点击链接 | 真实链接生成函数本地复现 |
| S-04 | 中 | 普通用户可删除/覆盖管理员 Key 配额 | 拥有被管理员限制的 Key 及普通账户会话 | 两次独立权限/SQL 审查 |
| S-05 | 中 | 用户总额度实际按单 Key 统计，可轮换 Key 绕过 | 管理员配置用户级额度，用户能创建多个 Key | 规则匹配、聚合 SQL 与创建 Key 调用链审查 |
| S-06 | 中 | WebSocket 仅在握手限流，后续生成请求不计入 Key 请求频率 | 有效 API Key、可使用 Responses WebSocket | 握手与逐消息处理调用链审查 |
| S-07 | 中，条件 | 跨源重定向继续发送上游 X-API-Key，且允许 HTTPS 降级 | 上游返回跨源/降级重定向 | 使用真实 HTTP client 策略与内存 transport 复现 |

**S-01：未认证请求在鉴权前被无上限读入内存（CWE-400 / CWE-770）**

位置：[sessions.go:53](</home/yeyingxingchen/Xinghai/xinghai-router/internal/app/sessions.go#L53>)、[request_limits.go:91](</home/yeyingxingchen/Xinghai/xinghai-router/internal/app/request_limits.go#L91>)、[routes.go:283](</home/yeyingxingchen/Xinghai/xinghai-router/internal/app/routes.go#L283>)。

`consoleSecurity` 包裹整个路由，在 `/auth/`、`/account/`、`/admin/` 的非安全方法请求上，先调用 `readRequestBody`，之后才执行内部的认证、权限或登录 IP 限流。`readRequestBody` 使用无字节上限的 `io.ReadAll`。非浏览器客户端可以自行设置 `X-Xinghai-Request: 1`，不提供 Origin/Cookie 即可进入该分支；该请求头是 CSRF 防护的一部分，不能当作身份凭据。

本地测试向三个路径分别输入 16 MiB 的生成式请求体，未提供任何凭据：`/admin/channels` 和 `/account/keys` 在完整读取 16,777,216 字节后返回 401；不存在的 `/admin/nonexistent` 同样完整读取后返回 404。没有实施内存耗尽压测。实现中没有总字节上限，故更大请求和并发请求可放大内存占用。30 秒读取超时不能阻止攻击者在期限内快速上传大量数据。

建议：在第一次读取之前施加分路由的 `http.MaxBytesReader` 或等效硬上限，对超限请求返回 413；认证/限流应尽可能早于完整缓冲。大图片上传可采用独立且有界的流式处理。边缘代理与应用都应限制请求体；也应复查支付表单与网关上传的读取路径。[Compose:44](</home/yeyingxingchen/Xinghai/xinghai-router/docker-compose.yml#L44>) 默认发布 Go 端口，因此仅在 Web 入口限大小不足以覆盖所有部署。

**S-02：未验证本地账户与 OAuth 自动合并导致预劫持（CWE-287）**

位置：[accounts.go:64](</home/yeyingxingchen/Xinghai/xinghai-router/internal/app/accounts.go#L64>)、[accounts.go:95](</home/yeyingxingchen/Xinghai/xinghai-router/internal/app/accounts.go#L95>)、[site.go:68](</home/yeyingxingchen/Xinghai/xinghai-router/internal/app/site.go#L68>)、[oauth.go:264](</home/yeyingxingchen/Xinghai/xinghai-router/internal/app/oauth.go#L264>)、[oauth.go:297](</home/yeyingxingchen/Xinghai/xinghai-router/internal/app/oauth.go#L297>)。

注册仅在 SMTPHost 和 SMTPFrom 同时配置时要求邮箱验证码。关闭邮箱验证时，攻击者可以用受害者邮箱注册并设置自己知道的密码。之后受害者首次通过 GitHub 的已验证邮箱登录，系统只按 `users.email` 找到原账户并建立 OAuth 连接，保留原账户密码、既有会话及 API Key。受害者与攻击者因此进入同一账户，攻击者仍可访问受害者后续充值、用量及密钥等资源。

前提是攻击者先注册、邮箱验证当时未开启、GitHub OAuth 可用且受害者随后使用该邮箱登录；历史上未验证的账户也需要处理。该问题不需要伪造 GitHub 邮箱或绕过 OAuth state。已静态核实完整路径，未用数据库动态重放。

建议：持久化本地邮箱验证状态；禁止仅凭邮箱字符串将 OAuth 身份自动并入未经验证的密码账户。冲突应进入明确的再认证或安全账户恢复流程。如通过恢复确认邮箱真实归属，需同时处理旧密码、会话和 API Key。仅现在启用 SMTP 不足以清除历史预注册风险。

**S-03：密码重置链接信任未经验证的 Host（CWE-640）**

位置：[password_reset.go:19](</home/yeyingxingchen/Xinghai/xinghai-router/internal/app/password_reset.go#L19>)、[password_reset.go:93](</home/yeyingxingchen/Xinghai/xinghai-router/internal/app/password_reset.go#L93>)、[site.go:646](</home/yeyingxingchen/Xinghai/xinghai-router/internal/app/site.go#L646>)、[Nuxt 代理:53](</home/yeyingxingchen/Xinghai/xinghai-router/web/server/api/[...path].ts#L53>)、[Nuxt 代理:66](</home/yeyingxingchen/Xinghai/xinghai-router/web/server/api/[...path].ts#L66>)。

未配置 `site_settings.public_base_url`，或其查询失败时，`resetLink` 用请求的 `r.Host` 生成包含真实 token 的重置链接。Nuxt 代理原样保留 Host。攻击者可以使用恶意 Host 向合法服务为受害者请求重置邮件；受害者点击系统邮件中的链接后，token 会到达攻击者域名，攻击者再向合法服务提交 token 和新密码完成重置。

成立前提：SMTP 启用，受害者有可重置密码，可信公开地址不可用，入口未拒绝非本站 Host，且用户点击有效期内的邮件链接。若生产反向代理严格限定 Host 或应用始终使用固定公开 origin，此路径会被阻断。HTML 转义不能防止链接指向错误域名。

纯本地测试确认：真实 `consoleRequestAllowed` 接受无 Origin 的标记请求，真实 `resetLink` 返回 `http://attacker.invalid/auth/reset?token=fake-audit-reset-token`。未发送邮件或使用真实 token。

建议：密码重置与 OAuth callback 必须使用服务端配置的可信 origin，缺失或读取失败时拒绝生成安全链接；在应用和边缘入口设置 Host 白名单。

**S-04：普通用户可修改管理员强制配额（CWE-862）**

位置：[key_quota.go:153](</home/yeyingxingchen/Xinghai/xinghai-router/internal/app/key_quota.go#L153>)、[key_quota.go:163](</home/yeyingxingchen/Xinghai/xinghai-router/internal/app/key_quota.go#L163>)、[admin.go:3813](</home/yeyingxingchen/Xinghai/xinghai-router/internal/app/admin.go#L3813>)、[routes.go:72](</home/yeyingxingchen/Xinghai/xinghai-router/internal/app/routes.go#L72>)、[routes.go:241](</home/yeyingxingchen/Xinghai/xinghai-router/internal/app/routes.go#L241>)。

管理员通过需要 `quotas.manage` 的接口创建 Key 配额，但自助删除只检查 Key 属于当前用户/workspace，随后按 `api_key_id + model IS NULL + window` 删除所有匹配行。没有区分管理员规则与用户自设预算，甚至没有检查自助操作允许的窗口。

例如，管理员为 K 设置 `minute` 上限后，K 的普通所有者可用正常会话调用 `DELETE /account/keys/K/quota?window=minute` 删除该规则，包括管理员设置的 `user_id IS NULL` 行。`day/month/total` 同样受影响。自助 upsert 还可覆盖管理员创建的同 user+key+window 规则。纯用户级和 model-specific 规则不在该 DELETE 范围内。

影响是绕过请求、token 或花费限制；钱包仍正常扣费，不等于免费使用。无需并发。结论基于两次独立的权限和 SQL 审查，没有操作业务数据库。

建议：分离管理员强制上限和用户自设预算，或新增不可由普通用户修改的规则来源/管理者字段；自助操作仅作用于 self-service 行，强制校验 `day/month/total`，多层额度取共同约束。

**S-05：用户额度按单 Key 计数，轮换 Key 即可绕过（CWE-863 / CWE-840）**

位置：[gateway.go:1588](</home/yeyingxingchen/Xinghai/xinghai-router/internal/app/gateway.go#L1588>)、[gateway.go:1616](</home/yeyingxingchen/Xinghai/xinghai-router/internal/app/gateway.go#L1616>)、[gateway.go:1637](</home/yeyingxingchen/Xinghai/xinghai-router/internal/app/gateway.go#L1637>)、[accounts.go:440](</home/yeyingxingchen/Xinghai/xinghai-router/internal/app/accounts.go#L440>)。

`matching_limits` 选出用户级规则，但后续用量 CTE 永远使用 `rl.api_key_id=$2` 聚合当前 Key，未按规则的用户作用域聚合所有 Key。管理员为用户 U 设置每天 100 次时，K1 达到 100 后被拒绝，U 创建 K2 后，K2 的历史计数为零，又能使用该用户的一整份额度。花费与 token 限额同样受影响，不需要利用竞态或缓存。

建议：按每条规则实际作用域聚合。用户级规则统计该用户全部 Key；Key 级统计指定 Key；模型级再施加相应模型筛选。增加跨 Key 合计达到上限后新 Key 仍被拒绝的数据库回归测试。当前结论来自 SQL 与创建 Key 调用链；未执行数据库测试。

**S-06：WebSocket 后续消息绕过 API Key 频率限制（CWE-799）**

位置：[routes.go:319](</home/yeyingxingchen/Xinghai/xinghai-router/internal/app/routes.go#L319>)、[responses_websocket.go:38](</home/yeyingxingchen/Xinghai/xinghai-router/internal/app/responses_websocket.go#L38>)、[responses_websocket.go:60](</home/yeyingxingchen/Xinghai/xinghai-router/internal/app/responses_websocket.go#L60>)、[responses_websocket.go:79](</home/yeyingxingchen/Xinghai/xinghai-router/internal/app/responses_websocket.go#L79>)。

`api` 中间件在 HTTP 升级握手时消耗一次 Key 限流额度。升级后的循环逐条接收 `response.create`，重新加载 Key 后直接调用网关，没有对每条生成请求调用同一频率限制器。因此一个连接可承载超过 HTTP Key 请求频率政策的生成请求；上游足够快或使用多个连接时影响更明显。

这不绕过 Key 撤销检查、内容策略、计费或网关中的数据库配额/并发控制。现有逐帧 Key 重认证是有效防护，但不能替代逐请求限流。结论为静态调用链确认，未做完整 WebSocket+数据库负载复现。

建议：每个生成消息在上游调用前使用相同的 Key 限流策略，并处理 Redis fail-closed；明确握手是否单独计数，避免修复后首条生成重复计费于限流窗口。

**S-07：跨源/降级重定向泄露上游 API Key（CWE-200 / CWE-319）**

位置：[http_client.go:44](</home/yeyingxingchen/Xinghai/xinghai-router/internal/app/http_client.go#L44>)、[http_client.go:77](</home/yeyingxingchen/Xinghai/xinghai-router/internal/app/http_client.go#L77>)、[gateway.go:925](</home/yeyingxingchen/Xinghai/xinghai-router/internal/app/gateway.go#L925>)、[admin.go:73](</home/yeyingxingchen/Xinghai/xinghai-router/internal/app/admin.go#L73>)。

重定向检查只验证目标为 HTTP/HTTPS，允许跨主机和 HTTPS→HTTP。Go 对标准 Authorization 的跨域转发有内置处理，但不会自动将 `X-API-Key` 视为敏感字段移除。Anthropic 等路径使用该字段，因而重定向可把渠道密钥发送到非原始 origin，甚至明文 HTTP。

本地使用真实 `newHTTPClient` 和自定义内存 transport 模拟 `https://provider.example/v1/models` 返回 302 到 `http://unrelated.example/collect`，确认目标请求仍带测试渠道密钥。没有实际向任何域名发请求。

前提是合法上游、其开放重定向路径或中间设备产生该重定向；普通用户不能仅凭网关参数任意选择目标。此问题与管理员主动配置内网/HTTP 上游的支持能力不同。

建议：携带凭据的上游请求默认禁用重定向，或严格允许同 origin 的必要跳转；禁止 HTTPS 降级，跨源时清除全部认证头。保留必要内网上游配置时，也应单独限制重定向行为。

**已披露但仍需运营处理的计费风险**

1. **取消流且尚无 usage 时可不结算。** [错误流结算逻辑](</home/yeyingxingchen/Xinghai/xinghai-router/internal/app/gateway.go#L1117>)只在已有 usage 且非纯估算时结算；[预留释放](</home/yeyingxingchen/Xinghai/xinghai-router/internal/app/gateway.go#L1558>)随后释放余额。OpenAI/Responses 在最终 chunk 才给 usage 时，用户获得部分内容后主动断开，可落入该分支；订阅次数也可能不扣。已有 usage 的流仍会按已知用量结算，不能笼统认为所有取消免费。另有[直接转发顺序问题](</home/yeyingxingchen/Xinghai/xinghai-router/internal/app/gateway.go#L2097>)：先写下游，再解析 usage，写失败会丢掉已经从上游读到的 usage。离线提取函数测试输出 `output_received=true canceled=true has_usage=false would_settle=false`。[项目规则](</home/yeyingxingchen/Xinghai/xinghai-router/AGENTS.md#L121>)已披露缺 usage 不计费，本报告单独呈现此风险。建议先解析 usage，再写下游；为已产生输出但缺 usage 的请求设计可审计估算/待对账，订阅次数与 token 金额分开结算。

2. **订阅及日志型额度有在途并发超额窗口。** [订阅准入](</home/yeyingxingchen/Xinghai/xinghai-router/internal/app/subscriptions.go#L1525>)仅查询 remaining>0，[网关](</home/yeyingxingchen/Xinghai/xinghai-router/internal/app/gateway.go#L672>)对 Covered 请求不预留钱包，[完成结算](</home/yeyingxingchen/Xinghai/xinghai-router/internal/app/gateway.go#L1515>)才扣减并钳制到零。剩余一次时，多个允许并行的请求可以同时通过，超额取决于并发配置和在途数量，不能称作无限使用。[README](</home/yeyingxingchen/Xinghai/xinghai-router/README.md#L77>)已说明软额度边界。本次为静态确认，未做数据库并发复现。如 `block` 要承诺硬额度，需事务条件预扣、失败释放和幂等结算。

[计费文档](</home/yeyingxingchen/Xinghai/xinghai-router/docs/billing.md#L79>)还明确说明 token/工具/媒体预留不是精确成本上界，可能出现 `hold_capped`、`missing_usage` 等状态。应配置实际监控与对账，而不是将其视为正常完成收费。

**依赖扫描结果与适用性**

前端执行 `pnpm audit --prod --json --registry=https://registry.npmjs.org`。默认镜像不支持 audit API，改用官方接口后成功取得结果，退出码 1 表示发现告警。返回 **52 个告警条目：4 critical、33 high、14 moderate、1 low**。其中同一公告可能涉及多个版本；这不是 52 个已确认可从项目公网利用的漏洞。当前 dependencies 包含 lint/build 工具，`--prod` 不代表所有命中都进入生产运行产物。

Go 使用 `govulncheck v1.9.0`、漏洞数据库更新时间 `2026-10-21T00:27:29Z`，本地编译器为 `go1.26.4`。JSON 结果含 **46 个不同的受影响公告，其中 27 个存在静态符号调用路径**。JSON 模式退出码 0 不能解读为“无漏洞”。调用图可能过度近似接口分派，且部分漏洞需 HTTP/2、SSH 服务端、特定 SQL 模式等额外条件，因此这些数字不等于可利用漏洞数量。

| 组件 | 本次版本 | 建议修复线，按本次公告集合 | 适用性说明 |
| --- | --- | --- | --- |
| Go 标准库/工具链 | 本地 go1.26.4 | 至少 Go 1.26.10，或更新的受支持安全版本 | 涉及 TLS、MIME、HTTP 等；生产二进制编译版本未检查 |
| pgx | v5.8.0 | 至少 v5.9.2 | SQL 占位符相关公告存在调用图路径，但未确认项目的查询/运行模式满足利用前提 |
| golang.org/x/crypto | v0.47.0 | 至少 v0.56.0 | SSH 节点部署调用可达；不同公告需分别区分客户端与服务端条件 |
| golang.org/x/text | v0.33.0 | 至少 v0.41.0 | 风控名称归一化和数据库认证路径有静态调用；未证明外部输入满足特定触发字节条件 |
| Nuxt | 锁定 3.21.9 | 至少 3.21.10 | 命中 islands 等公告，但当前未启用 componentIslands，未发现 server island 组件，不报告为已确认 RCE |
| Vue server-renderer | 3.5.40 | 至少 3.5.42，保持 Vue 包版本一致 | 命中属性名处理 XSS 公告，未找到本项目攻击者控制属性名的链路 |

版本依据：[Go 依赖声明](</home/yeyingxingchen/Xinghai/xinghai-router/go.mod>)、[前端锁文件](</home/yeyingxingchen/Xinghai/xinghai-router/web/pnpm-lock.yaml>)。相关公告：[pgx GO-2026-5004](https://pkg.go.dev/vuln/GO-2026-5004)、[x/text GO-2026-5970](https://pkg.go.dev/vuln/GO-2026-5970)、[SSH GO-2026-6354](https://pkg.go.dev/vuln/GO-2026-6354)、[Go GO-2026-6809](https://pkg.go.dev/vuln/GO-2026-6809)、[Nuxt GHSA-9473-5f9j-94wq](https://github.com/advisories/GHSA-9473-5f9j-94wq)、[Vue GHSA-g2v6-rqmx-r4w6](https://github.com/advisories/GHSA-g2v6-rqmx-r4w6)。x/text 公告明确要求无效 UTF-8 等触发条件，不能仅因名称归一化调用可达就认定用户可触发死循环。

其他前端告警涉及 glob、brace-expansion、simple-git、devalue、seroval、shell-quote 等。建议按实际依赖链升级兼容版本、同步锁文件并重新构建；不要对不兼容大版本盲目强制 override。CI 目前仅构建/测试/lint，应加入依赖审计和可达性复核。未自动升级依赖。

**额外加固与待验证事项**

- **对话缓存本地权限过宽。** [conversation_cache.go:135](</home/yeyingxingchen/Xinghai/xinghai-router/internal/app/conversation_cache.go#L135>)创建 0755 目录、0644 明文对话文件；本地测试确认文件权限 0644。若父目录可被其他本机用户遍历，其他 UID 可以读取用户对话。缓存需管理员开启且用户允许数据使用；这不是公网未授权读取。建议目录 0700、文件 0600、明确保留周期与备份权限。
- **GitHub 错误响应应拒绝登录。** [oauth.go:360](</home/yeyingxingchen/Xinghai/xinghai-router/internal/app/oauth.go#L360>)未先检查 `/user` 的 HTTP 状态和非零 ID；JSON 错误响应可被解码为 `provider_user_id=0`。本地 mock 403 验证了该行为。跨用户混淆还需多个回调进入相同异常状态，未证明攻击者可诱导指定受害者进入该状态，因此未计为已确认接管漏洞。建议检查 token 非空、HTTP 2xx、ID>0、必要邮箱获取失败时明确失败。
- **system.manage 实际属于高信任权限。** [节点部署](</home/yeyingxingchen/Xinghai/xinghai-router/internal/app/node_deployment.go#L514>)向所配置 SSH 节点下发数据库、Redis 与加密主密钥。该功能需要共享凭据，但意味着可配置自有节点的人实际可获得后端全部数据权限。应把该权限当作超级运维权限，或拆出更严格的部署权限并要求近期认证，不能仅依据角色名称将其当作低权限运维。
- **部署边界。** [Compose](</home/yeyingxingchen/Xinghai/xinghai-router/docker-compose.yml#L44>)发布 Go/Web 宿主端口，需确认生产防火墙、TLS 终止、Host 白名单和代理信任范围。此处未读取线上配置，不能认定生产已暴露明文服务。渠道凭据默认明文属于当前设计，可按环境启用加密存储并规划迁移。
- **构建供应链。** [Dockerfile:59](</home/yeyingxingchen/Xinghai/xinghai-router/Dockerfile#L59>)安装未固定版本的 pnpm，[Dockerfile:65](</home/yeyingxingchen/Xinghai/xinghai-router/Dockerfile#L65>)开放所有依赖构建脚本。建议固定工具链与基础镜像、仅允许必要构建脚本。没有证据证明当前包被植入恶意代码。已提交的镜像归档仅检查外层成员清单，未执行镜像或完成所有 OCI 层/历史秘密审计。

**已核实的有效防护**

会话随机 token 哈希存储、HttpOnly/Secure/Lax Cookie、控制台 CSRF 检查、近期重新认证、权限变更与密码重置撤销会话、workspace 所有权/成员校验、参数化 SQL，以及支付回调的签名/商户/金额检查和行锁幂等均有实现。本次没有确认普通用户直接提升角色、跨 workspace 访问他人 Key、无商户密钥伪造充值、负数充值、常规钱包预留双花、前端 XSS 或跨用户 SSR 状态泄漏的路径。这些是审查范围内的结论，不是不存在其他漏洞的证明。

**验证记录与局限**

- `go build ./...`、`go vet ./...`、`go test ./...`：通过；执行时移除数据库/Redis/密钥环境变量，未连接业务数据库。全量单元测试使用了 Go 的有效缓存。
- 主审查临时 Go overlay：4 项测试，验证鉴权前无界读取、跨源密钥转发、恶意 Host 重置链接和对话文件权限；通过，使用 `-count=1`。
- 认证子审查：14 项定向 Go 纯测试、2 项临时 overlay 测试通过；26 项前端转发头测试通过。
- 计费子审查：内存流+失败 writer 的离线复现确认已发送内容但 usage 丢失的分支；不依赖服务或数据库。
- 依赖审计：保留完整官方 npm audit JSON、govulncheck JSON 和归并后的调用链结果。
- 未运行数据库集成测试、真实 OAuth/SMTP/支付往返、生产渗透测试、拒绝服务压测或数据库并发压测；未核对生产反向代理、镜像内编译器版本、全 Git 历史秘密和所有二进制层；未对 LLM adapter 单独完成依赖数据库审计。静态确认项与动态复现项已分别注明。

建议先修复 S-01，并立即核实 S-02/S-03 的配置前提；随后修复 S-04/S-05 的额度权限与作用域问题、S-06/S-07 的协议差异，并并行完成依赖升级。软额度与中断计费需要明确产品承诺和运营对账政策，再实现对应的硬性保证。

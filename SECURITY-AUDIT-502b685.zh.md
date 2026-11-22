# Xinghai Router 安全审计

审计对象为 `main` 的当前工作树，基线提交 `502b685`，包含本地修改与新增源码。下述等级按触发条件成立后的影响评估。权限与计费结论来自静态代码路径，生产环境的权限分配、日志访问、密钥模式以及历史账目需要由部署方核对。

## 本轮修复状态

节点部署、恢复通道、一次性管理员引导、密码重置令牌、流式缺量计费、订阅查询失败处理、钱包日结恢复、OAuth 解绑、渠道路由权限、系统安全配置 fail-closed 和 SMTP STARTTLS 要求已实现并通过默认 Go 测试。OAuth 解绑和路由权限改动新增了数据库迁移。钱包历史孤立预留仍没有请求存活状态，系统不会按账龄自动释放这类预留。Web 控制台权限改动已通过 lint、生产构建和节点权限回归测试。

## 优先修复

| 风险 | 成立条件与影响 | 修复及验收 |
|---|---|---|
| 高 · 节点部署可向其他信任域交付集群密钥 | `system.manage` 可以指定节点及镜像并启动部署，无需近期认证。服务把数据库连接串、Redis 连接串和全局 `ENCRYPTION_KEY` 写入远端配置。目标 SSH 可达、身份验证成功时，目标主机即可获得这些值；主机指纹也由登记节点的人提供。更换现有节点的 SSH 目标时，已有密码还会沿用。[路由](internal/app/routes.go#L266) · [创建](internal/app/node_deployment.go#L216) · [更新凭证](internal/app/node_deployment.go#L284) · [部署配置](internal/app/node_deployment.go#L514) | 将节点注册与部署权限和站点管理分离。仅允许由部署方预先批准的目标、主机指纹与镜像摘要。目标变化后重新录入凭证，限制节点使用的数据库和 Redis 身份。验证仅获站点管理权限的会话无法触发密钥分发。 |
| 高 · 账户恢复邮件配置可由普通系统管理权限改写 | `system.manage` 可以修改 SMTP 主机和找回链接域名，无需近期认证。找回流程向该中继交付有效令牌，适用于启用的管理员账号。变更主机还可沿用旧 SMTP 密码。[设置权限](internal/app/routes.go#L179) · [写入](internal/app/site.go#L492) · [邮件发送](internal/app/email_verify.go#L26) · [使用令牌](internal/app/password_reset.go#L175) | 将恢复通道交由独立的高权限配置控制，要求近期认证和变更告警。绑定获准 SMTP 主机和域名，更换主机时重新配置凭据。 |
| 高 · 引导管理员会接管同邮箱已有账号 | 配置 `BOOTSTRAP_ADMIN_EMAIL` 后，每次启动都会按邮箱启用账户并提升为管理员。现有会话仍可用，鉴权会即时读取新角色。保留显式引导密码还会在重启后覆盖之后轮换的密码。[启动入口](internal/app/app.go#L196) · [引导逻辑](internal/app/bootstrap.go#L67) · [会话鉴权](internal/app/accounts.go#L890) | 仅在未初始化的实例上原子执行一次引导，已有同邮箱账号必须独立核验。验收时覆盖旧会话、主动降权、禁用账号和改密后的重启。 |
| 高 · 引导密码进入日志 | 创建新引导管理员且未提供密码时，普通服务日志同时记录邮箱和可登录的临时明文密码。首次改密前，有日志读取权限的人即可获得管理身份。[生成](internal/app/bootstrap.go#L40) · [日志](internal/app/bootstrap.go#L98) | 通过受限的一次性交付渠道提供初始化凭据。部署方应检查历史日志的访问范围，并轮换仍在使用的临时凭据。 |
| 高 · 流式请求可能按输出上限扣费 | 流正常结束、已输出内容而上游未完整报告输出用量时，代码把请求允许的最大输出填入使用量并结算，短回复也可能多扣。异常中断且尚未收到上游用量时则可能跳过结算，已发生的上游成本无法逐笔对账。[补齐用量](internal/app/billing_gateway.go#L61) · [估算公式](internal/app/gateway.go#L2242) · [结算分支](internal/app/gateway.go#L1217) | 分别存储预留上限、实际报告与估算用量。缺失用量时用观测值计量或登记待对账。覆盖短回复、部分用量、正常结束及中途断开四种回归用例。 |
| 高 · 钱包预留和日结缺少可靠恢复 | 请求预留已写入数据库后，若进程在生成结算单前退出，释放依赖的请求结束逻辑不会运行。释放失败也没有重试。日结只处理 UTC 昨日，停机错过窗口会遗漏旧单；存在失败条目时仍能把批次标记完成。可用余额可能长期冻结。[预留](internal/app/gateway.go#L1469) · [释放](internal/app/gateway.go#L1708) · [选日](internal/app/wallet_settlement.go#L19) · [封账](internal/app/wallet_settlement.go#L72) · [单条失败](internal/app/wallet_settlement.go#L119) | 每笔预留按请求 ID 持久记录状态，后台核对超时未决项。扫描全部到期账单，保留失败项重试。以预留后重启、跨两日停机、批内部分失败检验最终余额和流水。 |

## 其他代码风险

| 影响 | 证据与修复 |
|---|---|
| 恢复链接在改密后仍有效，邮箱重新分配时还能指向另一启用账户 | 令牌只绑定邮箱，确认时重新查找当前邮箱持有人，改密只清理会话。绑定不可变用户 ID 与凭据版本，在改密或改邮箱的事务中作废在途令牌。[令牌表](internal/app/migrations/055_password_reset_tokens.sql#L1) · [令牌确认](internal/app/password_reset.go#L175) · [改密事务](internal/app/accounts.go#L362) |
| 已解绑的 GitHub 身份可能重新登录 | 解绑只删除关联。外部身份仍能提供与本地相同的已验证邮箱时，后续登录会自动恢复绑定，现有会话也继续有效。主动解绑后应要求本地近期认证才能再绑定，并撤销相应登录来源的会话。[解绑](internal/app/oauth.go#L593) · [邮箱匹配](internal/app/oauth.go#L288) · [重新关联](internal/app/oauth.go#L324) |
| 人机验证或邮箱验证可在配置故障时关闭 | 保护只配于数据库、单次读取失败且随后其他业务查询仍可用时，系统回退到环境默认值。挑战密钥解密失败也可能产生相同效果。区分“明确关闭”与“配置不可用”，读取或解密异常时使用有效快照或拒绝本次请求。[配置读取](internal/app/site.go#L121) · [选择挑战](internal/app/site.go#L37) · [注册](internal/app/accounts.go#L64) |
| 渠道维护可跨过独立的路由管理权限 | 专用路由接口要求 `routes.manage`，但渠道新增和更新可仅凭 `channels.manage` 写入路由。更新还是先提交渠道、再逐条替换路由，失败会留下部分配置。统一权限判断并纳入同一事务。[独立权限](internal/app/routes.go#L240) · [创建写入](internal/app/admin.go#L2545) · [更新替换](internal/app/admin.go#L2905) |
| 订阅查询异常时可能改扣用户钱包 | 覆盖查询失败回退为 `allow_wallet`。若钱包查询随后成功，即使套餐设定超额阻止，也可能从钱包扣费。应区分确定无订阅、政策允许和查询失败，仅在明确授权时使用钱包。[查询错误](internal/app/subscriptions.go#L1883) · [钱包选择](internal/app/gateway.go#L728) |
| 同一份订阅额度可被多个在途请求使用 | 当前有意使用 10 秒软配额。请求结束才扣减且最低为零，在途请求还可能跨越续期边界。若产品承诺硬上限，应按订阅、周期和请求原子预留，仅缓存静态配置。[缓存语义](internal/app/cache.go#L25) · [准入](internal/app/subscriptions.go#L1899) · [扣减](internal/app/gateway.go#L1668) |
| 多币种套餐可能实收错误 | 非结算币种套餐的易支付下单直接使用套餐面额。余额购买在缺少启用汇率时按相同面额计点。应在创建订单时固化原币、结算币种、汇率与最终应收金额，缺汇率时拒绝下单。[套餐报价](internal/app/subscriptions.go#L1027) · [支付金额](internal/app/subscriptions.go#L1434) |
| 集群组并发上限可在数据库局部异常时失效 | 查询失败返回 `0`，上层将其当作无限制并省略该组租约。应将错误返回给网关，以现有 503 分支处理，并为组查询失败单独加测试。[查询回退](internal/app/cache.go#L404) · [集群调用](internal/app/concurrency_lease.go#L328) · [租约选择](internal/app/concurrency_lease.go#L267) |
| 支付通知迟到时可能无法自动入账，重复通知还会重复触发副作用 | 超时清理后，已付款但通知迟到的订单会被状态检查拒绝。已付款的订阅订单收到有效重复通知时，会再次通知管理员并清缓存。充值金额和订阅权益在已付款分支不会再次增加。持久化经验证的迟到事件以便对账，将通知改为与首次状态迁移同事务的唯一事件。[过期清理](internal/app/maintenance.go#L111) · [充值状态](internal/app/payments.go#L366) · [订阅状态](internal/app/subscriptions.go#L1749) · [通知](internal/app/payments.go#L416) |

## 部署相关暴露

- 默认 Compose 以明文模式保存上游渠道密钥。具有数据库或备份读取权限的人可以直接使用这些密钥。项目已有 `encrypted` 模式与历史记录迁移，应核对写入模式、执行迁移并确认旧备份和轮换策略。[Compose 配置](docker-compose.yml#L26) · [存储实现](internal/app/channel_credentials.go#L15) · [迁移入口](internal/app/channel_credentials.go#L164)
- 同时开启会话缓存与用户数据使用选项后，请求及回复正文以 JSON 文件落盘。目录与文件权限分别受限于 `0700`、`0600`，但应用内 `logs.read` 可查看缓存正文。建议拆分读取元数据与阅读正文的权限，记录正文访问审计。[采集条件](internal/app/conversation_cache.go#L218) · [文件写入](internal/app/conversation_cache.go#L282) · [访问权限](internal/app/routes.go#L188) · [响应字段](internal/app/conversation_cache.go#L385)
- 邮件使用非 465 端口且中继未提供 STARTTLS 时，代码可继续使用明文连接。如果中继无需 SMTP 认证，恢复邮件及验证码正文可能明文传输。应要求远程中继使用经证书验证的 TLS。[SMTP 连接与发送](internal/app/email_verify.go#L40)

## Web 依赖与验证

Web 的 22 项本地依赖回归测试全部通过。项目自带的在线审计脚本返回失败，注册表报告 35 条依赖记录，高 8、中 17、低 10、严重 0。计数包括同一公告落到多个包的情况，且其中一条 `node-forge` 公告已由本地补丁覆盖。脚本遇到的首条未覆盖公告为 Nuxt 路由规则大小写不一致。当前项目相关规则主要用于预渲染和响应头，管理 API 另有 Go 后端授权。优先升级锁定依赖并核对生产构建产物的实际调用路径。[项目核查逻辑](web/scripts/security-audit.mjs#L17) · [Nuxt 路由配置](web/nuxt.config.ts#L75) · [Nuxt 公告](https://github.com/advisories/GHSA-vjh5-4x84-xrqp) · [node-forge 示例公告](https://github.com/advisories/GHSA-7wch-p9px-956x)

本地 `go build -mod=vendor ./...`、`go vet -mod=vendor ./...` 与 `go test -mod=vendor -count=1 ./...` 均通过。测试范围是默认 Go 测试集，未连接生产服务或运行带外部数据库的集成测试。`govulncheck` 的模块校验镜像与官方 Go 模块代理均超时，Go 依赖的在线漏洞可达性检查未完成。生产暴露、账务差异规模和是否已有未授权访问仍需依据实际部署审计记录判断。

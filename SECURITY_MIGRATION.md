# 安全边界升级与迁移

本次升级收紧的是已登录账号的委派权限、浏览器会话与凭据处理，以及客户端请求资源消耗；不是把原有 `users.manage` 问题描述为匿名用户可直接越权。

## 1. 上线顺序与兼容性

1. 备份 PostgreSQL、现用 `ENCRYPTION_KEY` 和部署配置，先在隔离副本上演练。不要把数据库备份、Cookie jar 或明文 Key 提交到 Git。
2. 同步部署 Go 服务与 Nuxt Web 构建，不混跑旧版会话前端/后端。迁移 `097_secure_sessions.sql` **一次性删除旧 `user_sessions`**，所有控制台用户需要重新登录；API Key 不受影响。回滚不恢复已注销会话，不要恢复旧会话备份。
3. 生产环境使用 HTTPS，并保留默认 `SESSION_COOKIE_SECURE=true`。本地 HTTP 开发需明确设为 `false`；不要在公网部署关闭 Secure。此设置不依据可伪造的 `X-Forwarded-Proto` 自动降级。
4. 评估网关请求大小与上传速度，按下表调整超时、WS 消息和请求头限额。HTTP 请求体不再设置应用层大小限制，反向代理配置需与上传需求一致。
5. 渠道凭据的默认写入策略仍是 `plaintext`，**不会在启动时静默改写历史明文**。按第 4 节显式切换与迁移。既有明文 SQL 运维流程在启用加密后必须停用或改造。

## 2. 用户管理与权限授予

- `users.manage`：管理普通用户资料，不再意味着能变更角色或授予任意权限。
- `users.authorize`：角色/权限接口的独立授权边界；当前采取保守策略，**还必须是 `role=admin`**。把该字符串写入非管理员的权限列表也不能向上委派。
- `POST /admin/users/{id}/role`、`PUT /admin/users/{id}/permissions` 均禁止修改自己的角色或权限。创建/更新用户接口中的角色与权限字段使用同样规则，不存在旁路。
- 非管理员资料管理者只能修改 `role=user` 且没有显式权限的目标账号，不能通过邮箱、密码、启停、名称或 ID 修改接管高权限账号。用户 ID 调整限管理员；分组/余额分别要求 `system.manage` / `wallets.manage`。
- 用户变更在同一事务中重新检查操作者与目标账号权限，串行化授权写操作，并保护最后一名启用管理员。角色、权限、密码、邮箱等敏感变化按处理逻辑撤销目标会话。
- 数据导入 `POST /admin/migrate` 可能导入管理员及密码，也要求 `users.authorize`、真实管理员与近期再认证；普通 `system.manage` 不能通过可控来源数据库导入管理员绕过边界。
- 不把旧 `users.manage` 自动升级为 `users.authorize`。上线后复核现有权限清单、管理员列表和审计记录，按最小权限重新分配。

## 3. 服务端会话与二次验证

- 登录、注册、OAuth 回调由服务端签发 `xinghai.session`，属性为 `HttpOnly; Secure; SameSite=Lax; Path=/`（Secure 仅可通过明确配置关闭）。数据库保存 SHA-256 token hash，服务端有效期七天，JS 不再读取或写入 Bearer Cookie，登录 JSON 只包含 `expires_at`。
- `/auth/*`、`/account/*`、`/admin/*` 的修改请求必须带 `X-Xinghai-Request: 1`。有 Origin 时必须与入站 Host 一致；Fetch Metadata 不允许跨站或同站跨源请求。不要为这些接口配置宽泛 CORS，代理必须保留真实 Host、Cookie、多个 Set-Cookie 和 OAuth 302，不要信任客户端提供的 X-Forwarded-Host。
- 网关仍使用 API Key 的 `Authorization: Bearer` / `x-api-key`，不会改为 Cookie。支付异步回调仍使用自身签名验证，不套控制台 CSRF 规则。
- 三类已有凭据揭秘接口（账户 API Key、管理员 API Key、渠道 Key）、用户管理写操作、数据导入和渠道配置修改（防止把已有 Key 重定向到新上游）要求最近五分钟内通过 `POST /auth/reauthenticate` 再次提交密码。验证成功会轮换会话 token，不延长原七天有效期；旧 token 立即失效。验证按账号限制每分钟五次；返回及审计日志不包含密码或 Key，响应 `Cache-Control: no-store`。
- 控制台会显示密码对话框，成功后最多重试原请求一次。此机制是敏感操作再认证，**不是 MFA，也不能完全防止已执行在同源页面中的恶意脚本借用有效会话**。
- OAuth-only 账号没有可校验的本地密码时会被拒绝；须先通过已验证邮箱的密码重置流程设置密码。没有可恢复邮箱的账号应走可信运维恢复流程，不能绕过二次验证。

脚本访问账户/管理接口也改用 Cookie jar，避免把凭据放在 URL 或 shell 历史。下面使用明确的假密码展示接口，实际应从受控秘密输入生成 JSON：

```sh
umask 077
BASE=https://router.example.com/api
curl -c cookies.txt -H 'X-Xinghai-Request: 1' -H 'Content-Type: application/json' \
  "$BASE/auth/login" -d '{"email":"admin@example.com","password":"EXAMPLE-ONLY-password"}'
curl -b cookies.txt "$BASE/account/me"
curl -b cookies.txt -c cookies.txt -H 'X-Xinghai-Request: 1' -H 'Content-Type: application/json' \
  "$BASE/auth/reauthenticate" -d '{"password":"EXAMPLE-ONLY-password"}'
```

验证成功返回新的 Set-Cookie，调用方必须保存更新后的 jar。退出用 `POST /auth/logout` 并带 jar 与请求头；服务端删除会话并清 Cookie。敏感查询也需要 `X-Xinghai-Request: 1`。

## 4. 渠道凭据显式加密迁移

### 格式与边界

- `CHANNEL_CREDENTIAL_STORAGE=plaintext`：保持历史明文写入策略，兼容旧运维流程。
- `CHANNEL_CREDENTIAL_STORAGE=encrypted`：创建、替换、复制、单 Key 编辑、旧渠道 Key 搬迁等写入口使用 AES-GCM，格式带 `xh-credential:v1:` 前缀，密钥由 `ENCRYPTION_KEY` 派生。
- 两种模式都能读取新格式。带前缀的密文损坏、密钥错误或版本未知时拒绝使用，不会降级成“明文 Key”发给上游。
- **旧无前缀格式存在无法消除的歧义**：历史数据库混有明文与旧密文。读取时保留原有“先尝试旧格式解密，再按明文处理”的兼容方式。若旧密文的密钥已经丢失，程序不能可靠判断它是密文还是普通 Key。迁移前必须核对原密钥、测试旧渠道，必要时重新录入上游 Key；新前缀只能解决迁移后的歧义。

### 操作步骤

1. 备份并验证恢复；记录迁移前渠道数量，不导出 Key 到日志。
2. 停止旧版/明文模式写入者和直接 SQL Key 写入任务。将**全部实例**设为 `CHANNEL_CREDENTIAL_STORAGE=encrypted` 并重启。新写入已加密，但历史行不会自动改写。
3. 管理员登录、完成五分钟内的密码再认证，保存轮换后的 Cookie。
4. 显式调用：

   ```sh
   curl -b cookies.txt -c cookies.txt -H 'X-Xinghai-Request: 1' -H 'Content-Type: application/json' \
     "$BASE/admin/channels/credentials/migrate" -d '{"confirm":true}'
   ```

   此接口仅管理员可用，仅接受 encrypted 模式，必须 `confirm:true`。在事务中锁定 `channels` 与 `channel_api_keys`，迁移 `channels.api_key` 和 `channel_api_keys.key_encrypted`。响应只有统计，不返回秘密；任何解密/更新错误都会回滚。可重复调用，新格式不会重复加密。大表迁移会阻塞渠道写入，应在维护窗口执行。
5. 验证所有非空渠道凭据都有新格式；可只查计数：

   ```sql
   select count(*) as remaining_legacy_channels
   from channels where api_key<>'' and api_key not like 'xh-credential:v1:%';
   select count(*) as remaining_legacy_keys
   from channel_api_keys where key_encrypted<>'' and key_encrypted not like 'xh-credential:v1:%';
   ```

   随后做渠道测试和少量真实网关请求，观察只含错误类型的日志、渠道健康与审计结果。
6. 更新备份保密策略：原备份、数据库 WAL、复制节点和历史 SQL 导出仍可能含明文；此次逻辑迁移不等同磁盘安全擦除。对曾暴露的 Key 应在上游轮换，而不是只加密旧值。

### 回滚与密钥轮换

- 回滚应用版本必须选支持新前缀的版本。旧版会把新密文误当 Key，不能直接降级。
- 切回 `plaintext` 不会自动解密历史行，只会影响后续写入。不要在加密迁移后混跑两种写策略。
- 不直接改 `ENCRYPTION_KEY`。使用已支持新前缀的 `cmd/rotate-encryption-key` 工具，先在备份副本验证、维护窗口内统一更新密钥与所有实例。新格式损坏/未知版本应使轮换失败回滚，不能跳过后宣布成功。
- 数据导入工具也必须配置相同存储策略；`cmd/migrate` 默认保持 plaintext，显式 encrypted 模式才加密新导入凭据。离线 `cmd/encrypt` 同样要求 `CHANNEL_CREDENTIAL_STORAGE=encrypted`，使用事务与表锁转换历史渠道凭据；运行前遵循同样的备份/停写步骤。不要在加密实例旁运行旧版导入器或直接复制明文列。

## 5. 请求资源限制与长连接

所有以下值必须为正数（字节整数或 Go duration）；无效、零、负值拒绝启动，不提供 `-1` 无限模式。

| 环境变量 | 默认值 | 范围 |
| --- | --- | --- |
| `WS_MAX_MESSAGE_BYTES` | `2097152`（2 MiB） | WS 单条消息，含解压后的大小限制 |
| `REQUEST_BODY_TIMEOUT` | `30s` | 读取请求体，不是上游生成总时长 |
| `WS_IDLE_TIMEOUT` | `2m` | 等待下一条完整客户端 WS 请求 |
| `HTTP_READ_HEADER_TIMEOUT` | `10s` | 读取 HTTP 请求头 |
| `HTTP_IDLE_TIMEOUT` | `2m` | HTTP keep-alive 空闲时间 |
| `HTTP_MAX_HEADER_BYTES` | `1048576`（1 MiB） | Go HTTP 请求头限制 |

- HTTP 请求体不设应用层大小上限，适用于网关、图片 JSON / multipart、控制台、密码二次验证、工作区及集群接口；支持无 Content-Length / chunked 请求。旧 `GATEWAY_MAX_BODY_BYTES`、`IMAGE_MAX_BODY_BYTES` 配置不再读取，可从部署环境删除。读取超时仍返回 `408 request_timeout`。
- 请求体仍完整缓冲以保留重试能力，multipart 解析可能产生额外内存副本；大请求的内存占用随请求大小和并发数增加。
- WS 超限关闭码为 1009；空闲计时只覆盖客户端消息读取，不包裹上游活动响应。请求体成功读取后清除读 deadline，不用全局 ReadTimeout / WriteTimeout 简单截断 SSE/WS。上游生成超时仍由既有可靠性设置控制。
- 本次不是整个系统的全局内存/连接配额；边缘代理仍需连接数、并发、慢客户端、响应写入和 TLS 防护。不要为了兼容长流完全关闭边缘资源限制。

## 6. 验证

```sh
go build ./...
go vet ./...
go test ./...
cd web && pnpm run lint && pnpm run build
```

真实会话与授权事务回归还需隔离 PostgreSQL：`TEST_DATABASE_URL=... go test -tags integration ./internal/app`。部分现有 integration 测试会删除 public schema，**绝不能指向业务数据库**。网关边界测试包含真实 HTTP/1、HTTP/2 慢请求、WS 压缩消息上限和活动长响应不被请求读取时限截断的检查。

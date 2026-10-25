# 双副本滚动更新与应用迁移

[独立 Compose 模板](../docker-compose.rolling.yml) 在同一宿主机运行 `router-a`、`router-b` 和 HAProxy，共用外部 PostgreSQL、Redis 与 `ENCRYPTION_KEY`。[HAProxy 配置](../deploy/haproxy.cfg) 提供稳定入口。新增依赖只有部署层的 HAProxy 镜像，不增加 Go 或前端依赖，也不启动新的数据库、Redis 或前端。

目标是在计划内逐台替换应用时保持新请求可用，让已接收的 HTTP/SSE 请求和结算完成。WebSocket 当前生成结束后通过关闭码 **1012** 提示重连；客户端必须支持重连，连接本身不会迁移到另一进程。单机故障、HAProxy 重启、上游异常、资源不足和超时仍可能中断请求，不能把此模板理解为任意流无限存活或基础设施高可用保证。

## 停机协议与时间预算

| 阶段或配置 | 行为 |
| --- | --- |
| 收到 `SIGTERM` | `/readyz` 立即返回 503；监听仍在时 `/healthz` 保持 200。存活探针不能用于负载均衡摘流。 |
| `SHUTDOWN_DELAY=10s` | 给负载均衡传播摘流状态的时间，期间继续处理普通 HTTP 请求；到期后停止接收新的应用请求。 |
| HAProxy 主动检查 | `inter 1s`、`fall 2`、`rise 2`、检查超时 1s；默认 10s 延迟预留检查和调度余量。摘流不会主动断开既有连接，配置不启用 `on-marked-down shutdown-sessions`。 |
| 排空阶段 | 等待普通处理器、SSE、接管的连接及计费/预留释放完成，再关闭数据库、Redis 等资源。 |
| WebSocket | 当前生成及其结算完成后关闭 1012；空闲连接等待 `WS_IDLE_TIMEOUT`，默认 `2m`。应用排空后不再开启下一次生成。 |
| `SHUTDOWN_TIMEOUT=0s` | 应用内部不设请求排空期限；正数表示从收到信号起的请求排空预算，**包含**传播延迟，必须大于 `SHUTDOWN_DELAY`。它不包含后续资源关闭等清理的全部耗时，不能当作进程必定退出的总时限。 |
| `stop_grace_period=24h` | Docker 等待的上限，超过后仍会发 `SIGKILL`。应用无限等待不取消此限制。 |
| HAProxy 长连接超时 | `timeout client/server/tunnel` 均为 `24h`，是连接活动相关的空闲超时，不是无限承诺。外层 CDN、TLS 代理和客户端也必须允许业务所需时长。 |

为最大预期请求时长、传播延迟、结算和资源关闭留出预算；若设置正数应用超时，应让 Docker 停止宽限期更长。业务需要超过 24 小时时，发布前统一调整 `ROUTER_STOP_GRACE_PERIOD`、HAProxy 及外层代理超时，并验证宿主维护窗口。发布命令可能等待排空数小时；不要用 `docker compose stop -t 10`、强制删除或流水线超时把它截断。

## 使用前提

- 两个副本都必须支持 `DEPLOYMENT_MODE=cluster`、Redis 共享租约、配置失效通知和本次排空协议，固定使用 `REDIS_FAILURE_POLICY=deny`、`LOCAL_PROMPT_CACHE=false`。不要混跑 `single` 与 `cluster`。
- PostgreSQL、Redis 的实例和逻辑数据库、Redis 数据、`ENCRYPTION_KEY`、渠道凭据存储模式必须保持不变。加密密钥至少 24 字符，沿用现有密钥，不要为新副本重新生成。共享服务须能从容器网络访问；容器内 `127.0.0.1` 不是宿主机。
- 为两副本的数据库连接、Redis 连接及续租预留容量。**任一副本排空时，剩余副本须独立承载全部新流量**，且宿主仍承担旧副本在途请求。提前按峰值压测。
- 应用版本、schema、计费/租约语义及静态配置要支持新旧同时运行；提前保留可兼容新增 schema 的旧镜像。启动自动迁移不等于任意版本兼容。
- 每副本有独立命名卷，挂载镜像中可写的 `/tmp`，保存 `/tmp/sessions` 会话绑定快照及 `/tmp/conversations`。保持 Compose 项目名和卷名不变，不要 `down -v`，也不要把同一目录挂给两个副本。绑定是进程本地状态，轮询分流或换机不会保证将绑定转给另一个副本，也不保证每次请求命中同一上游缓存。
- 此处仍保留公开聚合、后台任务、软额度等既有集群边界，详见 [README 的多副本边界](../README.md#单实例与多副本部署边界)。

## 配置与首次启动

使用近期 Docker Compose 插件（须支持 `up --wait`）。以下命令在仓库根目录执行。此文件是**独立入口**，不要与 [单实例 Compose](../docker-compose.yml) 合并，也不要对其使用 `--scale`。

在本地创建受限权限的 `.env.rolling`（不要提交），填写真实配置，并迁入原有 SMTP、验证码、管理员引导、IP 限流、Cookie 与请求大小/超时等自定义设置，避免用模板默认值覆盖现有行为。模板已固定集群模式、Redis 拒绝策略、本地 prompt 缓存关闭及副本内目录；这些值不能靠环境文件切回单实例。这里只展示格式，示例凭据不能用于生产：

```dotenv
ROUTER_IMAGE=registry.example.com/xinghai-router:release-2026-10-24
DATABASE_URL=postgres://router:EXAMPLE_ONLY@db.internal:5432/router?sslmode=require
REDIS_URL=redis://:EXAMPLE_ONLY@redis.internal:6379/0
ENCRYPTION_KEY=EXAMPLE_ONLY_KEEP_THE_EXISTING_SECRET
ROLLING_BIND_ADDR=127.0.0.1
ROLLING_PORT=8080
SHUTDOWN_DELAY=10s
SHUTDOWN_TIMEOUT=0s
ROUTER_STOP_GRACE_PERIOD=24h
WS_IDLE_TIMEOUT=2m
```

```sh
chmod 600 .env.rolling
```

`ROUTER_IMAGE` 对两副本使用同一发布标签或摘要，不使用会漂移的 `latest`。拉取预构建镜像时推荐 `registry/repo@sha256:...`；本地构建用每版唯一、不会复写的标签，摘要形式不能作为本地构建目标。模板复用同一 `router` 构建目标，只需构建一次，禁止在逐台替换时分别构建两份不同产物。

在同一个 Bash 会话定义命令和检查函数。`config --quiet` 校验配置但不打印环境中的秘密；不要把完整渲染结果上传到日志。

```bash
set -euo pipefail

dc() {
  docker compose --env-file .env.rolling -p xinghai-rolling -f docker-compose.rolling.yml "$@"
}

wait_backend() {
  local node="$1"
  local attempt
  for attempt in {1..60}; do
    if dc exec -T "$node" wget -q -O - 'http://haproxy:8404/stats;csv' |
      awk -F, -v node="$node" '$1 == "routers" && $2 == node && $18 == "UP" { ready=1 } END { exit !ready }'; then
      return 0
    fi
    sleep 2
  done
  printf 'HAProxy backend not ready: %s\n' "$node" >&2
  return 1
}

dc config --quiet
dc pull haproxy router-a
```

若从当前经过审核的发布源码本地构建，用 `dc build router-a` **替代**上述 `dc pull haproxy router-a` 中的应用拉取，并单独 `dc pull haproxy`。两个服务引用同一镜像，无需第二次构建。

先只做 HAProxy 语法检查，再启动全新集群；这些启动命令只用于首次上线，已有集群更新须使用下一节的逐台命令：

```bash
dc run --rm --no-deps haproxy haproxy -c -f /usr/local/etc/haproxy/haproxy.cfg
dc up -d --no-build --pull never --wait --wait-timeout 180
wait_backend router-a
wait_backend router-b
dc ps
curl --fail --silent --show-error http://127.0.0.1:8080/readyz
```

按实际 `ROLLING_BIND_ADDR` / `ROLLING_PORT` 修改 `curl` 地址。两副本都健康且在 HAProxy 中为 `UP` 后，才让业务入口切入。HAProxy 的内部统计端口 8404 不发布到宿主机；共享此 Docker 网络的容器仍能访问它，不要把不受信任的容器接入。

## 稳定入口、DNS 与可信转发

默认稳定入口是 `127.0.0.1:8080`，仅 HAProxy 发布宿主端口，两台 router 不发布端口。若旧单实例占用 8080，先以其他 `ROLLING_PORT` 建好新入口，再通过既有外层代理切换；不要为抢占端口直接杀掉旧实例。需要远端访问时绑定明确的宿主内网地址，并限制防火墙来源。

HAProxy 使用 Docker 内置 DNS `127.0.0.11:53`，有效与失败缓存为 1s；即使容器替换后 IP 改变，后端也会重新解析。`init-addr ... none` 允许某个副本尚不存在时代理正常启动，其恢复仍须通过 `/readyz` 主动检查。更新应用时保持 HAProxy 容器不变，不需要为每次 IP 变化重启代理。

默认私网 `172.30.240.0/24`，HAProxy 固定为 `172.30.240.2`，router 的 `TRUSTED_PROXIES` 只信任这个 IP。若与既有网络冲突，在首次启动前配套修改 `ROLLING_SUBNET` 和 `HAPROXY_IPV4`；不要扩大为信任所有私网或 `0.0.0.0/0`。运行中的网络改址属于单独维护操作。

默认配置删除客户端提供的 `Forwarded`、`X-Forwarded-*` 中相关身份/协议头和 `X-Real-IP`，以实际连接源地址重写 IP，以当前明文监听重写 `X-Forwarded-Proto: http`，避免任意客户端伪造身份。若 HAProxy 前还有 Nuxt、CDN 或 TLS 代理，默认记录的是这一跳代理 IP，也不会保留外部 HTTPS 元数据。需要真实客户端 IP/协议时，须按真实拓扑增加严格的来源 IP ACL，仅从明确受信的入口接收已清洗的转发信息，或在 HAProxy 终止 TLS 并根据实际 TLS 连接设置协议；不能直接透传公网请求的任意转发头。上线前验证真实 IP、限流、OAuth/重定向和 Secure Cookie 行为。

前端不包含在本模板中。现有 Nuxt 的 `API_INTERNAL_URL` 应指向稳定 HAProxy 地址，例如同一 Docker 网络内的 `http://haproxy:8080`，或可达的宿主内网入口；不要指向 `router-a` / `router-b`。远端前端不能访问此模板默认的宿主回环监听，须配置受限内网入口。前端及其外层代理的滚动发布另行安排。

## 顺序更新：一台就绪后再停止下一台

首次部署和每次更新前，都要验证两台当前副本在 HAProxy 中为 `UP`，真实 API 请求可用，并留存当前镜像标签/摘要、环境和发布记录。在测试环境用持续 SSE 与 WebSocket 请求排演：正常生成完成、用量/账本只结算一次、预留释放、新请求继续成功、WebSocket 收到 1012 后可重连。

发布前独立准备新镜像，不要让构建或下载占用单副本运行窗口。在上节 Bash 会话中，将以下标签替换为真实的不可变发布标识；环境变量覆盖本地环境文件中的镜像值：

```bash
export PREVIOUS_ROUTER_IMAGE=registry.example.com/xinghai-router:release-old
export ROUTER_IMAGE=registry.example.com/xinghai-router:release-new

dc config --quiet
dc pull router-a
wait_backend router-a
wait_backend router-b

dc up -d --no-deps --no-build --pull never --wait --wait-timeout 180 router-a
wait_backend router-a
```

本地构建时以 `dc build router-a` 替换 `dc pull router-a`。`up` 会先向旧 A 发送 `SIGTERM`，等其排空退出后才创建新 A；期间 B 接收新流量。`--wait-timeout 180` 用于新服务就绪等待，不应被当成缩短旧请求排空的方式；外层发布任务本身必须允许足够时长。

**任何命令失败或新 A 启动/迁移/就绪失败，都应立即停止发布，保留 B。** 不要继续执行 B 的命令。检查 `dc logs --tail=200 router-a`，确认 HAProxy 后端 A 为 `UP`，并通过 `dc exec -T router-a wget -q -O - http://127.0.0.1:8080/readyz` 验证 A 自身；对新 A 进行有代表性的认证请求、SSE/计费验证并观察错误率、延迟和租约日志。只有验收通过才执行：

```bash
wait_backend router-a
dc up -d --no-deps --no-build --pull never --wait --wait-timeout 180 router-b
wait_backend router-b
wait_backend router-a
dc ps
```

再次完成业务验收，再把已验证的 `ROUTER_IMAGE` 写回本地发布配置并记录镜像摘要。逐台命令使用 `--no-deps`，因此不会顺带替换 HAProxy 或另一台应用。更新时不要运行不带服务名的 `up --build`、`restart`、`down`，也不要同时停止两台；Compose 本身不会自动提供滚动发布或蓝绿切换。

## 失败与回滚

新 A 失败时让 B 继续服务，先查明迁移、依赖、配置或镜像问题。若预先验证旧镜像兼容已经应用的 schema，可仅恢复失败副本：

```bash
ROUTER_IMAGE="$PREVIOUS_ROUTER_IMAGE" dc up -d --no-deps --no-build --pull never --wait --wait-timeout 180 router-a
wait_backend router-a
```

若 B 更新失败，先保证已验收的 A 继续服务，再按同样方式仅回退 B。需要整体回滚时也逐台执行，每台经检查后再动下一台。保留旧镜像，不提前清理镜像或卷；回滚应用不会撤销数据库迁移，禁止自动执行破坏性 down migration。若旧镜像不兼容新 schema，停止回滚并采取前向修复或已演练的恢复方案，不要把健康副本一起停掉。

## 数据库 schema 与应用迁移

启动迁移通过 PostgreSQL advisory lock 串行执行，避免参与该协议的副本同时启动时争抢迁移；所有同时启动的迁移执行器都必须包含这一锁协议，首次升级时不要并行启动仍未加锁的旧版本。连接目标须保持数据库会话语义，不能经事务级连接池代理。DDL 的 `lock_timeout=5s` 限制等待表锁的时间。拿不到表锁会导致迁移失败、新副本不能就绪，应保留健康副本并处理长事务或锁冲突，随后重试。5s 是**锁等待**限制，不是整个迁移执行时间上限，也不保证持锁期间无请求延迟。

采用先扩展、再切换、最后收缩的发布顺序：先新增兼容列/表，让新旧代码均可运行；数据回填与新代码验证完成后，在后续版本移除旧字段/约束。禁止在滚动重叠期直接删列、改名或引入旧代码无法满足的约束。大表建索引、重写与回填须单独评估锁和负载；启动迁移当前在事务内执行，不能把需要事务外执行的操作直接塞入迁移文件。

现有 [130_fast_service_tier_prices.sql](../internal/app/migrations/130_fast_service_tier_prices.sql) 自带 `BEGIN`/`COMMIT`，会提前结束迁移执行器开启的事务；该文件的数据变更与迁移记录写入不具备同一事务的原子性。本次保留已发布迁移，发布前应评估中途失败后的重试及审计记录影响；新增迁移不要自行提交事务。

此处的“迁移”主要指保持同一 PostgreSQL、Redis 与密钥的**应用版本或应用部署位置迁移**。把 PostgreSQL 数据复制到新机器、改变数据库连接目标、迁移 Redis 数据/数据库或切换加密密钥，不会因为有两台 router 就自动零停机。物理数据迁移需要复制、写入切换、租约一致性和专门恢复方案；Redis 租约丢失或两个活跃实例使用不同 Redis 还可能破坏全局并发限制。不能在逐台应用更新中顺手改这些值。

跨宿主迁移应用也需让既有稳定负载均衡同时可达新旧宿主，先接入兼容且就绪的新节点，再摘除旧节点并排空。此模板的 Docker DNS 和私网仅适用于同一 Docker 宿主；它本身不会提供跨宿主发现或把既有 TCP 连接转移到新宿主。

## 首次从单实例切入与旧版升级

第一次切入的前提比常规滚动更新严格：单实例必须先具备兼容 schema、共享租约和 `cluster` 模式，且所有接流节点使用同一套依赖与模式配置。若现有实例仍是 `single` 或不支持共享语义，不能直接将新集群与它并行接流；应先停止新请求并排空旧实例，完成模式切换后再开放新入口。这一步可能需要维护窗口，不能承诺自动无中断。

**旧二进制曾使用固定 15 秒停机预算。** 第一次升级时发送信号的仍是旧进程，新镜像中的 `SHUTDOWN_DELAY`/无限等待逻辑不会补到旧进程里，Docker 的 24h 宽限期也不能延长旧二进制自身的 15 秒。长流可能被截断。先在外层代理摘除旧节点并观察业务在途请求与结算全部完成，再停止旧进程；只有 `/healthz` 或 `/readyz` 成功不能证明已经排空。没有可承载的新兼容节点时安排维护窗口。后续两副本都具备本次排空能力后，才适用上述常规滚动流程。

保留现有数据库/Redis 服务及数据卷，勿对旧完整栈直接 `down`，否则可能同时停止新集群仍在使用的基础设施。旧服务仅有 Docker 内网名字时，先解决新集群安全可达性；不要把数据库或 Redis 无保护地公开到公网。

管理后台现有“节点部署/更新”仍是 Compose 服务替换，不会因为新增模板就自动变为蓝绿或逐台发布。必须显式使用本页流程或实现等价的编排与验收；不要把后台一键替换视作已保证排空的集群更新。

## 只做离线配置校验

使用全假的变量并禁用默认环境文件读取即可校验模板，不连接数据库，也不部署任何服务：

```bash
DATABASE_URL='postgres://validation:validation@db.invalid:5432/validation' \
REDIS_URL='redis://redis.invalid:6379/0' \
ENCRYPTION_KEY='validation-only-not-a-real-secret-123456' \
ROUTER_IMAGE='xinghai-router:validation-only' \
docker compose --env-file /dev/null -p rolling-validation -f docker-compose.rolling.yml config --quiet
```

有本机 HAProxy 时运行 `haproxy -c -f deploy/haproxy.cfg`。或在获准的 Docker 环境中运行 `docker run --rm --network none -v "$PWD/deploy/haproxy.cfg:/usr/local/etc/haproxy/haproxy.cfg:ro" haproxy:3.0-alpine haproxy -c -f /usr/local/etc/haproxy/haproxy.cfg`，这只校验语法。配置校验不替代真实请求、DNS 换址、摘流时序、权限和结算的部署演练。

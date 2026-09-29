# 多工作空间 / Workspaces

通过控制台侧栏切换个人空间和团队空间；“工作空间”页面支持创建空间、编辑名称和标识、添加已注册成员以及管理成员角色。

| 资源 | 范围 |
| --- | --- |
| API 密钥及其配额 | 创建时绑定空间；成员只能查看和管理自己在当前空间的密钥 |
| 用量明细、月度汇总、每日统计 | 当前空间全体成员的请求与费用 |
| 钱包、充值、订阅 | 个人账户；密钥费用仍由创建者承担 |
| 渠道、模型定价、全站日志及系统设置 | 全站管理，使用原有平台权限 |

空间角色不授予平台管理权限。所有者管理空间与全部非所有者成员；管理员修改空间名称并管理普通成员；普通成员创建自己的密钥并查看空间用量。个人空间不可删除或添加成员。

移除成员会撤销该成员在空间内的密钥。删除团队空间执行归档并撤销所有密钥，保留历史请求和用量。重新加入空间不会恢复已撤销的密钥。

## API

管理接口需要账号会话，写请求沿用 `X-Xinghai-Request: 1` 和同源校验。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET / POST | `/account/workspaces` | 列出 / 创建空间（`name`, `slug`） |
| PUT / DELETE | `/account/workspaces/{id}` | 修改 / 归档空间 |
| POST | `/account/workspaces/{id}/select` | 验证访问权并返回 `workspace_id` |
| GET / POST | `/account/workspaces/{id}/members` | 列出 / 添加已注册成员（`email`, `role`） |
| PUT / DELETE | `/account/workspaces/{id}/members/{user_id}` | 修改角色 / 移除成员 |

对 `/account/keys`、`/account/keys/*`、`/account/usage`、`/account/usage/*`，通过 `X-Workspace-ID: <uuid>` 指定空间。省略时使用个人空间。无权限或归档空间会被拒绝，不回退到其他空间。控制台验证成员关系后才发起请求，切换时取消旧请求并重新挂载页面。

网关通过 API 密钥确定空间，不接受请求头覆盖。`/me/keys` 和 `/me/usage` 限于密钥所属空间及持有者。

## Migration and verification

服务启动时自动迁移：既有账号获得个人空间，既有密钥和可关联历史记录归入个人空间。新账号也自动获得个人空间。

```sh
go build ./...
go vet ./...
go test ./...
WORKSPACE_TEST_DATABASE_URL='<isolated-postgres-url>' go test ./internal/app -run Workspace
TEST_DATABASE_URL='<disposable-postgres-url>' go test -tags integration ./internal/app -run 'TestIntegrationWorkspaceResourceIsolation|TestIntegrationMigrateEmptyDatabaseAndIdempotency'
```

`TEST_DATABASE_URL` 集成测试会重建 `public` schema，必须使用可丢弃的独立数据库。

## English

Switch personal and team workspaces in the console sidebar. Settings support creation, renaming, registered-user membership and owner/admin/member roles. Keys and quotas belong to a workspace and remain private to their creator. Usage reports aggregate the selected workspace. Wallets and subscriptions remain personal, and keys charge their creator. Platform administration keeps its existing global permissions.

Removing members revokes their workspace keys permanently. Deleting a team workspace archives it and revokes its keys while retaining history. Personal workspaces cannot be deleted or shared.

Session-authenticated key and usage endpoints accept `X-Workspace-ID`; omitted headers select the personal workspace. Inaccessible spaces are rejected. Gateway requests derive the space from the API key. The console cancels stale requests and remounts pages when switching.

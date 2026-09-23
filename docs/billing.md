# 统一用量与可复算计费

## 用量契约

新请求在 `usage_records.usage_facts` 保存独立的计费事实：

- `input_tokens` / `output_tokens`：非缓存、非音频、非图片的文本 token；各维度互斥，不能再把它们当作上游的总输入/输出。
- `cache_read_tokens` / `cache_write_tokens`：缓存读取与写入；`cache_write_5m_tokens`、`cache_write_1h_tokens` 是写入总数的子集，不重复相加计价。
- `audio_input_tokens` / `audio_output_tokens`、`image_input_tokens` / `image_output_tokens`：拆分后的多模态 token。
- `image_count`、`image_size`、`image_quality`：图片输出数量及请求参数。
- `audio_seconds`、`video_seconds`、`tool_calls`：上游明确报告的时长和服务端工具使用次数。客户端 function-call 数组不等同于收费工具调用。
- `usage_source`：`upstream`、`response_count`、`upstream_and_response_count`、`request_estimate`、`upstream_and_request_estimate` 或 `missing`。
- `pricing_version`：包含有效价格、汇率和倍率的快照内容哈希。

Anthropic 的 `input_tokens` 不含缓存，OpenAI 的输入总数通常包含缓存和多模态 token，解析时分别归一化。SSE 累计数取合并后的累计值，不按事件重复相加；`message_start` 不能代表完整输出用量。本地提示词前缀相似不再冒充上游缓存命中参与结算。成功文本流仅报告部分用量时保留已知事实，并对缺失输入/输出采用请求估算，标记 `upstream_and_request_estimate`；中断流只结算已报告的上游用量，不对未完成部分追加估算。

旧的 `prompt_tokens`、`cached_prompt_tokens`、`completion_tokens` 字段继续保留供旧界面使用，其中 prompt/completion 为包含相应模态的总量。历史账单没有新快照，返回 `{}`，不伪造历史计价规则。

## 配置多维价格

`GET /admin/pricing` 返回 `dimension_prices`；通过已有 `POST /admin/pricing` 配置，仍要求 `pricing.manage` 权限。省略 `dimension_prices` 保留已有配置，显式 `{}` 清空，`null` 和未知字段被拒绝。示例为结构说明，**不是任何厂商的现行官方报价**：

```json
{
  "model": "example-model",
  "input_per_million": 3,
  "cached_input_per_million": 0.3,
  "output_per_million": 15,
  "multiplier": 1,
  "currency": "USD",
  "enabled": true,
  "dimension_prices": {
    "cache_write_per_million": 3.75,
    "cache_write_5m_per_million": 3.75,
    "cache_write_1h_per_million": 6,
    "audio_input_per_million": 10,
    "audio_output_per_million": 20,
    "image_input_per_million": 8,
    "image_output_per_million": 32,
    "audio_per_second": 0.001,
    "video_per_second": 0.02,
    "tool_per_call": 0.01,
    "images": [
      { "size": "1024x1024", "quality": "hd", "price": 0.08 },
      { "size": "*", "quality": "*", "price": 0.04 }
    ]
  }
}
```

价格单位为规则币种。token 价格按百万 token，时长按秒，工具按次，图片按张。未配置的缓存写入/音频/图片 token 价格兼容回退到相应文本价格；TTL 价格回退到通用缓存写入价格。实际采用的回退结果也写入快照，生产使用应显式配置正确费率。时长和工具维度不能用文本 token 价格代替：缺少对应费率会生成 `unpriced` 记录，不会伪装成正常零元结算。

本阶段只提供后台 API 配置和前端类型契约；价格编辑界面尚无新维度输入控件，旧界面保存不会清空它们。音视频/任务提交新接口不在本次变更范围内；现有响应明确报告的对应计量事实可以保存和计价。

## 图片接口

`/v1/images/generations` 与 `/v1/images/edits` 继续共用路由、鉴权、可靠性和钱包基础设施，但不再共用文本 token 计价：

1. 从 JSON 或 multipart 字段读取 `n`（默认 1，范围 1–100）、`size`、`quality`，忽略上传文件字节数；缺省尺寸/质量用 `default` 匹配价格。
2. 必须找到显式图片单价。完全匹配优先于通配 `*`；同等情况下尺寸完全匹配优先。未定价在调用上游前返回 `402 pricing_unavailable`，订阅请求也不能绕过该检查。
3. 按请求数量预留。响应必须包含非空 `data` 数组，每项具有 `url` 或 `b64_json`；按实际输出数收费，不按请求数量直接收费。返回数量超过请求数量或无效响应返回 502 并释放预留。
4. 图片响应即使没有 token usage，也以 `response_count` 记账。上游返回的 token 事实可以保留，但图片模式只按张计费，不同时叠加 token 费用。
5. 含有 `n`、`size`、`quality` 或 `stream` 请求覆盖配置的渠道不参与图片路由，防止调用时参数变更使预留和计费失配。此路径不支持图片 SSE；显式 `stream=true` 被拒绝。按图像 token 收费的模型需要另行定义图像 token 模式，不能把这里的按张单价当作其真实上游成本。

## 快照、汇率与舍入

请求开始时冻结时间窗口价格、当前汇率和分组倍率；结算根据实际 token 总量选择阶梯并保存最终有效价格。账单 `billing_snapshot` 包含：

- `usage`：统一事实及 `pricing_version`；
- `pricing`：生效时间、币种、版本、计费模式、有效 token 与多维价格；
- `exchange`：源币种、基准币种 `CNY`、当次 `rate_to_base`；
- `multipliers`：模型与分组倍率；
- `rounding`：`HALF_UP`，`scale=8`；
- `computed_amount`：根据事实计算的人民币金额；`amount` / `cost`：实际结算金额；
- `status`、`error`、可选 `adjustment`：结算状态与调整原因。

用十进制有理数计算各项费用，再乘汇率和倍率，**只在最终总额舍入一次**至 8 位小数，与数据库 `numeric(20,8)` 保持一致。`BillingSnapshot.Recompute()` 无需读取当前价格或系统时间，即可复算 `computed_amount`。

文本预留使用各阶梯、缓存 TTL、模态价格的保守上界；配置了正数时长/工具价格时，额外预留对应一秒/一次的最低准入估算，避免零余额调用绕过检查。结算优先尝试真实金额；余额不足以补足正数预留差额时，保留既有防透支行为，明确记录 `hold_capped`、原始计算金额和 `adjustment`，不再静默截断。正数费用不能截断为零元成功结算。它不是精确 tokenizer 或所有工具/媒体成本的上界保证；运营必须监控此状态。`unpriced`、`missing_usage`、`settlement_failed` 同样需要处理，不能视为正常完成收费。

快照与用量记录在钱包/订阅结算同一数据库语句中写入。重复请求结算受 request_id 唯一性保护，原账单不能被后续价格变更覆盖。`/me/usage`、`/account/usage` 和管理员用量日志接口返回 `usage_facts`、`billing_snapshot`；聚合统计仍保留原 token 指标。

## 迁移与验证

启动时自动应用 `099_unified_billing.sql`，只新增 JSONB 字段，不修改历史金额。部署前为图片模型配置 `dimension_prices.images`，否则原先未定价的图片调用将被拒绝。

```sh
go build ./...
go vet ./...
go test ./...
TEST_DATABASE_URL='postgres://.../isolated_test_db' go test -tags integration ./internal/app -run 'TestIntegration(ImageBilling|CacheTTLBilling|MigrateEmpty)'
```

集成测试会重建测试数据库的 public schema，**只能指向隔离测试库**，不得使用生产连接。

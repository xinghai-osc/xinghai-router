# MiniMax M Plan 视频接入

MiniMax 渠道使用 M Plan 订阅 Key。视频能力取决于套餐权益；官方目前将 H3 视频列为 Explore / Build 可用。订阅 Key 和按量 API Key 不通用。

## 渠道配置

1. 在渠道管理中选择 **MiniMax M Plan**。
2. Base URL 填 `https://api.minimax.cn`，不要附加 `/v1` 或 `/v2`。
3. 填入订阅 Key，添加 `MiniMax-H3` 或 `MiniMax-H3-Max`，并分配调用者可用的分组。
4. 上游格式使用“自动”。模型别名可以通过模型路由映射到上述官方模型。
5. 在价格配置中设置模型的 `dimension_prices.video_per_second`；没有有效价格时网关不会创建任务。

H3 渠道健康检查查询模型列表，不创建收费视频任务。该检查只验证连接和鉴权；实际视频权益由生成接口判断。

## MiniMax 原生请求

调用网关时使用星海 API Key，而不是 MiniMax 订阅 Key。

```sh
export XINGHAI_BASE_URL='https://your-router.example'
export XINGHAI_API_KEY='your-router-api-key'

curl --fail --show-error "$XINGHAI_BASE_URL/v2/video_generation" \
  -H "Authorization: Bearer $XINGHAI_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{
    "model":"MiniMax-H3",
    "content":[{"type":"text","text":"一只猫在花园中奔跑，镜头缓慢跟随"}],
    "resolution":"768P",
    "duration":5,
    "ratio":"16:9"
  }'
```

创建成功返回 `{"task_id":"video_..."}`。这是网关任务 ID，与上游任务关联；后续请求必须使用此 ID 和创建任务时的星海 API Key、工作空间。

```sh
TASK_ID='video_...'
curl --fail --show-error "$XINGHAI_BASE_URL/v2/query/video_generation/$TASK_ID" \
  -H "Authorization: Bearer $XINGHAI_API_KEY"

curl --fail --show-error -X DELETE "$XINGHAI_BASE_URL/v2/video_generation/$TASK_ID" \
  -H "Authorization: Bearer $XINGHAI_API_KEY"
```

查询保留 MiniMax 的 `task` 结构、`queued/running/succeeded/failed/cancelled` 状态及 `content.url` 等结果字段。任务成功后从 `task.content.url` 获取视频；下载地址可能过期，应及时保存。

删除接口由上游决定实际动作：排队中的任务可取消，成功或失败的任务可删除，运行中的任务不可取消。查询和删除始终使用创建任务时的渠道与 Key；渠道失效或 Key 被替换时返回不可用，不切换到其他账号。

`content[]` 支持文本、首尾帧图片以及参考图片、视频和音频。媒体项采用 `{"type":"image_url","image_url":{"url":"https://..."},"role":"first_frame"}` 等官方结构，参考媒体使用 `reference_image` / `reference_video` / `reference_audio`。首尾帧和参考媒体不能混用。请求中的额外字段会保留，但 `callback_url` 暂不支持，请通过查询接口轮询任务。

## OpenAI 视频入口

也可继续调用 `POST /v1/videos`。MiniMax 渠道将 JSON 或 multipart 的 `prompt`、`seconds`、`input_reference` 转为 H3 请求。JSON `input_reference` 支持 URL 字符串或 `{"url":"..."}`；multipart 支持图片文件。

推荐显式设置 `seconds: 5`。兼容入口默认值沿用已有接口的 4 秒，因此 H3-Max 必须显式提供至少 5 秒。可使用 `resolution` 和 `ratio`，或以下 `size` 映射：

| size | resolution | ratio |
| --- | --- | --- |
| 1280x720 / 1366x768 | 768P | 16:9 |
| 720x1280 / 768x1366 | 768P | 9:16 |
| 2048x1152 | 2K | 16:9 |
| 1152x2048 | 2K | 9:16 |

H3 支持 4–15 秒、768P/2K；H3-Max 支持 5–15 秒、480P/768P。图片首帧输入使用自适应比例。查询使用 `GET /v1/videos/{id}`，下载使用 `GET /v1/videos/{id}/content`，下载不会向视频 CDN 发送模型或客户端凭据；支持 Range，MiniMax 不支持 thumbnail/spritesheet 变体。

## 积分余额

保存渠道后，编辑页面提供独立的 **M Plan 积分查询凭据** 设置。填写 MiniMax 网页登录 Cookie（含 `_token`）和请求头 `x-group-id` 对应的 Group ID。两者与模型订阅 Key 独立。

Cookie 使用服务端 `ENCRYPTION_KEY` 加密保存，读取配置只返回是否已配置和 Group ID；更新时 Cookie 留空保留旧值，也可单独清除配置。此配置由 `channels.manage` 权限控制，修改和删除沿用控制台近期验证要求。

余额查询使用固定地址 `https://www.minimax.cn/backend/account/token_plan_credit`，发送 Cookie、Group ID、Origin 和 Referer，禁止跟随重定向。仅将 `remaining_credits`、`used_credits`、`total_credits` 映射为剩余、已用和总积分，包括值为 0 的情况；响应中的 `api_key` 不返回、不缓存。结果接入既有余额按钮与每 6 小时刷新。Cookie 过期时需更新网页登录凭据。

## 当前计费及验证范围

视频沿用现有媒体计费机制：上游接受任务后按请求时长计费；创建失败不产生用量，查询、下载和删除不重复计费。异步任务最终失败或取消暂不自动退款。列表、H3-Context-IR、视频再生成和回调不属于本次接入范围。

测试使用假凭据和模拟上游，不会创建真实收费视频。数据库集成测试需要独立测试库：

```sh
TEST_DATABASE_URL='postgres://.../isolated_test_database' \
  go test -tags integration ./internal/app -run TestIntegrationMiniMaxVideo -count=1
CHANNEL_CREDENTIAL_TEST_DATABASE_URL='postgres://.../isolated_test_database' \
  go test ./internal/app -run TestMiniMaxBalanceDatabase -count=1
```

视频测试会重建测试库的 public schema，必须使用专门的测试数据库。

参考：[M Plan 概览](https://platform.minimax.cn/docs/m-plan/intro)、[创建视频任务](https://platform.minimax.cn/docs/api-reference/video-generation-v2-create)、[查询任务](https://platform.minimax.cn/docs/api-reference/video-generation-v2-query)、[取消或删除任务](https://platform.minimax.cn/docs/api-reference/video-generation-v2-delete)。

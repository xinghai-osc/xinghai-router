# 音频与视频网关 / Audio and video gateway

## 中文

### 接口与接入

使用控制台提供的 SDK 基础地址和星海 Router API 密钥，鉴权方式为 `Authorization: Bearer $XINGHAI_API_KEY`。经 Web 代理时基础地址以 `/api/v1` 结尾；直接连接 Go 网关时以 `/v1` 结尾。下方示例中的 `XINGHAI_BASE_URL` 已包含此路径，请勿重复添加 `/v1`。

| 方法与路径（相对基础地址） | 请求 | 成功响应 |
| --- | --- | --- |
| `POST /audio/speech` | JSON，包含 `model`、`input`、`voice` | 音频二进制，请保存为文件 |
| `POST /audio/transcriptions` | multipart，包含 `model`、音频 `file` | `json` 或 `verbose_json` |
| `POST /audio/translations` | multipart，包含 `model`、音频 `file` | `json` 或 `verbose_json` |
| `POST /videos` | JSON 或 multipart，包含 `model`、`prompt` | 异步任务 JSON，包含任务 ID 和状态 |
| `GET /videos/{id}` | 使用创建任务的同一 API 密钥 | 任务状态 JSON |
| `GET /videos/{id}/content` | 使用创建任务的同一 API 密钥 | 视频内容二进制 |

模型名填写密钥所在分组可用的公共模型名，网关按模型路由映射到上游名称。模态元数据用于展示与筛选，不能证明上游支持某个接口、音色或文件格式。管理员应先核对上游文档与实际支持情况。

### 定价与额度预留

价格使用模型的定价币种，并遵循现有汇率、模型倍率和分组倍率。管理员在控制台定价编辑器配置以下维度。留空表示未配置，`0` 表示明确的零价格。

| 操作 | 必需或适用的计费维度 | 结算依据 |
| --- | --- | --- |
| 语音合成 | `audio_per_million_characters` | 输入文本的 Unicode 码点数，不是字节数或可见字符簇数 |
| 音频转写/翻译：时长模式 | `audio_per_second` | 上游计量的音频秒数 |
| 音频转写/翻译：Token 模式 | 明确配置 `audio_input_per_million`，且不配置 `audio_per_second`；输出音频价格为 `audio_output_per_million` | 上游报告的文本/音频 Token 用量 |
| 视频创建 | `video_per_second` | 请求的 `seconds`，省略时为 4 秒 |

音频转写和翻译支持网关专用 multipart 字段 `max_audio_seconds`：可选，默认 `600`，范围 `1`–`3600` 秒。该字段在转发上游前移除。网关会先根据上传文件结构计算时长并确认未超上限，再调用上游。请按录音的完整时长设置上限，不能只按希望处理的片段设置。

配置 `audio_per_second` 时，网关按上限秒数预留，再按实测秒数结算。未配置秒价时，必须明确配置音频输入 Token 价格：预留估算按每秒各 100 个文本输入、文本输出、音频输入、音频输出 Tokens 计算，并计入文本提示词，保守地合计这四项价格。结算后释放未使用的预留额度。缺失计量数据、实测时长超过上限或计量费用超过预留报价的响应会被拒绝。

视频在上游接受创建任务时按请求秒数计费；返回的任务尚未完成也会计费。状态轮询和内容下载不额外收费。任务之后失败暂不自动退款，需要运营人员按实际情况处理。网关不会自动重试视频创建 POST，避免重复创建和重复消耗；客户端收到超时或不确定结果时，也不应盲目重试。

### 能力边界与运营注意事项

- 语音合成目前要求非空 `input` 和 `voice`，输入最多 4096 个 Unicode 码点。返回的是音频二进制，不是 JSON 或 SSE。
- 音频文件仅支持普通 RIFF WAV 的 PCM（8/16/24/32 位）或 IEEE 浮点（32/64 位），以及 MPEG 1/2/2.5 Layer III MP3。网关根据 WAV 样本或 MP3 帧计算时长，不信任 MP3 标签中的时长声明。压缩或 extensible WAV、M4A、Ogg、FLAC、损坏/截断文件会在转发前被拒绝。MP3 支持 VBR、ID3v2 前置标签和 ID3v1 尾部标签；不支持 free-format 或中途改变采样率/声道的流。
- 音频转写/翻译仅支持 `json` 和 `verbose_json`，不支持 `text`、SRT、VTT 或流式响应。时长计费模型应使用能够返回时长的格式，通常为 `verbose_json`；Token 计费模型必须返回可计量用量。
- 不支持媒体 SSE。视频通过异步任务状态查询跟踪进度，不通过聊天流式接口获取结果。
- multipart 请求由客户端库生成 `Content-Type` 和 boundary。每个字段名唯一；转写/翻译使用 `file`，视频参考文件使用 `input_reference`，音频上传还受上述 WAV/MP3 格式限制；视频参考图格式由上游决定。
- 媒体上传沿用 `IMAGE_MAX_BODY_BYTES` 限制，默认 50 MiB；同时受请求读取超时约束。不能把录音秒数上限视为文件大小限制。
- 视频 `seconds` 必须为 1–3600 的整数；上游模型通常有更严格的时长与尺寸范围。
- 视频任务只允许创建它的同一用户、同一工作区和同一 API 密钥访问。同一账户下的另一把密钥也不能读取。保留网关返回的任务 ID，不要自行替换为上游内部 ID。
- 查询/下载绑定创建任务时的上游渠道和凭据，不会切换到其他渠道。禁用或删除原渠道/凭据、改变上游地址可能使既有任务不可访问。
- 状态值包括 `queued`、`in_progress`、`completed`、`failed`。仅在完成后下载内容。内容接口支持 `Range`/`If-Range`，并允许 `variant=video|thumbnail|spritesheet`；具体变体能力取决于上游。
- 应配置支持这些 OpenAI 兼容媒体接口的渠道。Anthropic、CommandCode、JEV 等专用协议渠道不用于此媒体路径；带请求字段覆盖的渠道也不会用于媒体请求，以保持计量参数与上游请求一致。渠道基础地址不要附加 `/v1`。

## English

### Endpoints and setup

Use the SDK base URL shown in the console and a Xinghai Router API key with `Authorization: Bearer $XINGHAI_API_KEY`. The Web proxy base ends in `/api/v1`; a direct Go gateway base ends in `/v1`. `XINGHAI_BASE_URL` in the examples already includes that prefix.

| Method and path, relative to the base URL | Request | Successful response |
| --- | --- | --- |
| `POST /audio/speech` | JSON with `model`, `input`, `voice` | Binary audio; save it to a file |
| `POST /audio/transcriptions` | Multipart with `model` and audio `file` | `json` or `verbose_json` |
| `POST /audio/translations` | Multipart with `model` and audio `file` | `json` or `verbose_json` |
| `POST /videos` | JSON or multipart with `model`, `prompt` | Asynchronous task JSON with ID and status |
| `GET /videos/{id}` | Same API key that created the task | Task status JSON |
| `GET /videos/{id}/content` | Same API key that created the task | Binary video content |

Choose a public model available to the API key's group. The gateway maps it to the configured upstream model. Catalog modalities describe inputs and outputs; they do not guarantee support for a particular endpoint, voice or file format. Operators must verify upstream capabilities.

### Pricing and reservations

Prices use the model currency and the existing exchange rate, model multiplier and group multiplier. Configure these dimensions in the console pricing editor. Blank means unconfigured; `0` is an explicit zero price.

| Operation | Pricing dimension | Metering |
| --- | --- | --- |
| Speech | `audio_per_million_characters` | Input Unicode code points, not bytes or grapheme clusters |
| Transcription/translation: duration mode | `audio_per_second` | Upstream-reported audio seconds |
| Transcription/translation: token mode | Explicit `audio_input_per_million`, with `audio_per_second` unset; `audio_output_per_million` applies to audio output | Upstream text/audio token usage |
| Video creation | `video_per_second` | Requested `seconds`, default 4 |

Transcription and translation accept the gateway-only multipart field `max_audio_seconds`. It is optional, defaults to `600`, accepts values from `1` to `3600` seconds, and is removed before forwarding upstream. The gateway calculates duration from the uploaded file structure and checks the ceiling before calling the upstream. Set it to cover the entire recording.

When `audio_per_second` is configured, the gateway reserves the ceiling duration and settles measured seconds. Otherwise an explicit audio input token price is required. The conservative token quote reserves 100 tokens per second in each of the text input, text output, audio input and audio output dimensions, sums their prices, and includes the text prompt. Unused reservation is released at final settlement. Missing metering, measured duration over the ceiling or measured cost above the quote causes the response to be rejected.

Video creation is billed when the upstream accepts the task, even while generation is pending. Polling and downloading add no charge. Later asynchronous failures do not trigger automatic refunds; operators must handle adjustments as appropriate. The gateway does not automatically retry video creation POST requests. Clients should also avoid blind retries after a timeout or ambiguous result, since another task could be created.

### Limits and operational behavior

- Speech requires nonempty `input` and `voice`; input is limited to 4096 Unicode code points. It returns binary audio, not JSON or SSE.
- Audio files must be ordinary RIFF WAV with PCM (8/16/24/32-bit) or IEEE float (32/64-bit), or MPEG 1/2/2.5 Layer III MP3. Duration comes from WAV samples or MP3 frames, not an MP3 metadata duration claim. Compressed/extensible WAV, M4A, Ogg, FLAC and corrupt/truncated files are rejected before forwarding. MP3 supports VBR, leading ID3v2 and trailing ID3v1 tags; free-format MP3 and streams that change sample rate or channel count are unsupported.
- Transcription/translation only support `json` and `verbose_json`, not text, SRT, VTT or streaming responses. Duration-priced models need a format that returns duration, usually `verbose_json`; token-priced models must return measurable usage.
- Media SSE is unsupported. Follow video progress through the task status endpoint.
- Let the client library set multipart `Content-Type` and boundary. Field names must be unique. Use `file` for transcription/translation and `input_reference` for video reference uploads. Audio uploads also have the WAV/MP3 restrictions above; supported video reference image formats depend on the upstream.
- Media uploads use `IMAGE_MAX_BODY_BYTES`, default 50 MiB, and the request-body timeout. The recording duration ceiling is separate from the upload size limit.
- Video `seconds` must be an integer from 1 to 3600. Individual upstream models may allow a much smaller set of durations and sizes.
- Task access requires the exact creating user, workspace and API key. Another key belonging to the same account cannot retrieve it. Keep the task ID returned by the gateway; do not substitute an upstream internal ID.
- Retrieval remains bound to the original upstream channel and credential. It does not fail over to another channel. Disabling/deleting the original channel or credential, or changing its base URL, can make existing tasks inaccessible.
- Status values include `queued`, `in_progress`, `completed` and `failed`. Download after completion. Content requests support `Range`/`If-Range` and `variant=video|thumbnail|spritesheet`; available variants depend on the upstream.
- Configure channels that support these OpenAI-compatible media endpoints. Dedicated Anthropic, CommandCode and JEV protocol channels are not used for this media path. Channels with request-field overrides are also excluded to keep metering aligned with the forwarded request. Do not append `/v1` to a channel base URL.

## 请求示例 / Request examples

设置基础地址与密钥，替换模型占位符。/ Set the base URL and key, then replace model placeholders.

```sh
export XINGHAI_BASE_URL='https://your-gateway.example.com/api/v1'
export XINGHAI_API_KEY='YOUR_API_KEY'
```

语音合成，保存 MP3。/ Generate speech and save the MP3.

```sh
curl --fail --show-error "$XINGHAI_BASE_URL/audio/speech" \
  -H "Authorization: Bearer $XINGHAI_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"model":"YOUR_SPEECH_MODEL","input":"Hello from Xinghai Router.","voice":"alloy","response_format":"mp3"}' \
  --output speech.mp3
```

转写不超过 60 秒的录音。翻译使用同样表单，将路径改为 `/audio/translations`。/ Transcribe a recording of at most 60 seconds. For translation, use the same form with `/audio/translations`.

```sh
curl --fail --show-error "$XINGHAI_BASE_URL/audio/transcriptions" \
  -H "Authorization: Bearer $XINGHAI_API_KEY" \
  -F 'model=YOUR_AUDIO_MODEL' \
  -F 'file=@input.mp3' \
  -F 'max_audio_seconds=60' \
  -F 'response_format=verbose_json'
```

JSON 创建视频。/ Create a video with JSON.

```sh
curl --fail --show-error "$XINGHAI_BASE_URL/videos" \
  -H "Authorization: Bearer $XINGHAI_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"model":"YOUR_VIDEO_MODEL","prompt":"Waves gently washing onto a beach.","seconds":"4"}'
```

multipart 上传参考图创建视频（需上游支持）。/ Create a video with a reference image, when the upstream supports it.

```sh
curl --fail --show-error "$XINGHAI_BASE_URL/videos" \
  -H "Authorization: Bearer $XINGHAI_API_KEY" \
  -F 'model=YOUR_VIDEO_MODEL' \
  --form-string 'prompt=Waves gently washing onto a beach.' \
  -F 'seconds=4' \
  -F 'input_reference=@reference.png'
```

从创建响应复制 ID，再查询状态。/ Copy the ID from the creation response and check status.

```sh
VIDEO_ID='YOUR_VIDEO_ID'
curl --fail --show-error "$XINGHAI_BASE_URL/videos/$VIDEO_ID" \
  -H "Authorization: Bearer $XINGHAI_API_KEY"
```

状态为 `completed` 后下载。/ Download after status becomes `completed`.

```sh
curl --fail --show-error "$XINGHAI_BASE_URL/videos/$VIDEO_ID/content" \
  -H "Authorization: Bearer $XINGHAI_API_KEY" \
  --output video.mp4
```

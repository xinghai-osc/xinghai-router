# 音频与视频网关 / Audio and video gateway

## 中文

### 接口与接入

使用控制台提供的 SDK 基础地址和星海 Router API 密钥，鉴权方式为 `Authorization: Bearer $XINGHAI_API_KEY`。经 Web 代理时基础地址以 `/api/v1` 结尾；直接连接 Go 网关时以 `/v1` 结尾。下方示例中的 `XINGHAI_BASE_URL` 已包含此路径，请勿重复添加 `/v1`。

| 方法与路径（相对基础地址） | 请求 | 成功响应 |
| --- | --- | --- |
| `POST /audio/speech`（TTS） | JSON，包含 `model`、`input`、`voice` | 二进制音频，可边接收边播放或保存 |
| `POST /audio/transcriptions`（ASR） | multipart，包含 `model`、音频 `file` | 按 `response_format` 返回 JSON、文本或字幕 |
| `POST /audio/translations` | multipart，包含 `model`、音频 `file` | 按 `response_format` 返回 JSON、文本或字幕 |
| `POST /videos` | JSON 或 multipart，包含 `model`、`prompt` | 异步任务 JSON，包含任务 ID 和状态 |
| `GET /videos/{id}` | 使用创建任务的同一 API 密钥 | 任务状态 JSON |
| `GET /videos/{id}/content` | 使用创建任务的同一 API 密钥 | 视频内容二进制 |

模型名填写密钥所在分组可用的公共模型名，网关按模型路由映射到上游名称。模态元数据用于展示与筛选，不能证明上游支持某个接口、音色或文件格式。管理员应先核对上游文档与实际支持情况。

### 定价与额度预留

管理员需为公共模型启用定价规则。使用钱包时，网关在转发前预留余额。TTS 按每 4 个输入字符估算 1 个输入 Token，向上取整。ASR 和翻译按 `max_audio_seconds` 每秒估算 100 个输入 Token，并加上提示词的字符数。字符按 Unicode 码点计数。

结算采用上游报告的 Token 用量，并释放未使用的预留。纯音频、文本和字幕响应通常没有 Token 用量，此时仍返回结果，账单记为 `missing_usage`，金额为零。需要按量计费的 ASR 模型应选用能返回 Token 用量的 JSON 格式。

音频转写和翻译可设置 `max_audio_seconds`，默认 600 秒，范围 1–3600 秒。网关校验上传文件时长后移除该字段。请按完整录音的时长设置上限。

视频任务在上游接受创建请求时结算。状态轮询和内容下载不额外收费。任务之后失败暂不自动退款，需要运营人员按实际情况处理。网关不会自动重试视频创建请求，以免重复创建任务。

### 能力边界与运营注意事项

- 语音合成的 `input` 为 1–4096 个 Unicode 码点，`voice` 可填上游音色名或自定义音色对象，如 `{"id":"voice_123"}`。`response_format` 指定输出音频格式，如 `mp3`。
- 音频文件仅支持普通 RIFF WAV 的 PCM（8/16/24/32 位）或 IEEE 浮点（32/64 位），以及 MPEG 1/2/2.5 Layer III MP3。网关根据 WAV 样本或 MP3 帧计算时长，不信任 MP3 标签中的时长声明。压缩或 extensible WAV、M4A、Ogg、FLAC、损坏/截断文件会在转发前被拒绝。MP3 支持 VBR、ID3v2 前置标签和 ID3v1 尾部标签；不支持 free-format 或中途改变采样率/声道的流。
- 转写和翻译的 `response_format` 可选 `json`（默认）、`verbose_json`、`diarized_json`、`text`、`srt` 或 `vtt`。具体格式取决于上游模型。JSON 响应中的 `text` 为转写或翻译结果，其他字段随上游返回。
- 不支持媒体 SSE。视频通过异步任务状态查询跟踪进度，不通过聊天流式接口获取结果。
- 由客户端库生成 multipart 的 `Content-Type` 和 boundary。转写和翻译使用 `file` 上传音频。时间戳粒度等数组选项可重复传入，例如 `-F 'timestamp_granularities[]=word' -F 'timestamp_granularities[]=segment'`。模型和文件各传一次。视频参考文件使用 `input_reference`。
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
| `POST /audio/speech` (TTS) | JSON with `model`, `input`, `voice` | Binary audio for playback or saving as it arrives |
| `POST /audio/transcriptions` (ASR) | Multipart with `model` and audio `file` | JSON, plain text or subtitles according to `response_format` |
| `POST /audio/translations` | Multipart with `model` and audio `file` | JSON, plain text or subtitles according to `response_format` |
| `POST /videos` | JSON or multipart with `model`, `prompt` | Asynchronous task JSON with ID and status |
| `GET /videos/{id}` | Same API key that created the task | Task status JSON |
| `GET /videos/{id}/content` | Same API key that created the task | Binary video content |

Choose a public model available to the API key's group. The gateway maps it to the configured upstream model. Catalog modalities describe inputs and outputs; they do not guarantee support for a particular endpoint, voice or file format. Operators must verify upstream capabilities.

### Pricing and reservations

Enable a pricing rule for the public model. For wallet-funded requests, the gateway reserves funds before forwarding. TTS reserves one input token per four input characters, rounded up. ASR and translation reserve 100 input tokens per second of `max_audio_seconds`, plus one per prompt character. Characters are counted as Unicode code points.

The gateway settles upstream-reported token usage and releases unused funds. Binary audio, plain text and subtitles usually carry no token usage. These responses are delivered and recorded as `missing_usage` with a zero charge. For metered ASR, use a model and JSON response format that report token usage.

Transcription and translation accept the optional multipart field `max_audio_seconds`. It defaults to 600 seconds and accepts values from 1 to 3600. The gateway checks the uploaded file's duration and removes the field before forwarding. Set it to cover the full recording.

Video creation is settled when the upstream accepts the task. Polling and downloading add no charge. Later asynchronous failures do not trigger automatic refunds. The gateway does not retry video creation automatically because a retry could create a second task.

### Limits and operational behavior

- Speech accepts 1–4096 Unicode code points in `input`. Set `voice` to an upstream voice name or a custom voice object such as `{"id":"voice_123"}`. Use `response_format` to request an audio format such as `mp3`.
- Audio files must be ordinary RIFF WAV with PCM (8/16/24/32-bit) or IEEE float (32/64-bit), or MPEG 1/2/2.5 Layer III MP3. Duration comes from WAV samples or MP3 frames, not an MP3 metadata duration claim. Compressed/extensible WAV, M4A, Ogg, FLAC and corrupt/truncated files are rejected before forwarding. MP3 supports VBR, leading ID3v2 and trailing ID3v1 tags; free-format MP3 and streams that change sample rate or channel count are unsupported.
- Transcription and translation accept `json` (default), `verbose_json`, `diarized_json`, `text`, `srt` and `vtt` as `response_format`, subject to upstream model support. JSON responses carry the transcript or translation in `text`, along with any additional fields from the upstream.
- Media SSE is unsupported. Follow video progress through the task status endpoint.
- Let the client library set multipart `Content-Type` and the boundary. Upload audio in `file` and send the model and file once each. Array options can be repeated, for example `-F 'timestamp_granularities[]=word' -F 'timestamp_granularities[]=segment'`. Video reference uploads use `input_reference`.
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
  -F 'response_format=json'
```

如需字幕，将最后一行改为 `-F 'response_format=srt' --output transcript.srt`。 / To save subtitles, change the last line to `-F 'response_format=srt' --output transcript.srt`.

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

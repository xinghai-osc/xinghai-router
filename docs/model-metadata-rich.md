# 富模型元数据配置记录

本次配置覆盖 127 个当前模型调用名。名称、厂商、模态、上下文、输出上限和推理档位来自四份审查数据；描述字段保持生产库原值。未核实字段保持空值，避免把路由默认值当成模型官方能力。

核验状态：partial=58, unverified=20, verified=49。API capability 未在官方资料中取得足够证据，全部保留现值。

| 模型 | 名称 | 厂商 | 输入 | 输出 | 上下文 | 最大输出 | 推理档位 | 来源 |
| --- | --- | --- | --- | --- | ---: | ---: | --- | --- |
| MiniMax-M2 | MiniMax-M2 | minimax | text | text | 204800 | 204800 | — | https://platform.minimax.io/docs/guides/models-intro；https://platform.minimax.io/docs/api-reference/text-anthropic-api；https://platform.minimax.io/docs/api-reference/text-chat-openai；https://platform.minimax.io/docs/api-reference/text-openai-api |
| MiniMax-M2.1 | MiniMax-M2.1 | minimax | text | text | 204800 | 204800 | — | https://platform.minimax.io/docs/guides/models-intro；https://platform.minimax.io/docs/api-reference/text-anthropic-api；https://platform.minimax.io/docs/api-reference/text-chat-openai；https://platform.minimax.io/docs/api-reference/text-openai-api |
| MiniMax-M2.1-highspeed | MiniMax-M2.1-highspeed | minimax | text | text | 204800 | 204800 | — | https://platform.minimax.io/docs/guides/models-intro；https://platform.minimax.io/docs/api-reference/text-anthropic-api；https://platform.minimax.io/docs/api-reference/text-chat-openai；https://platform.minimax.io/docs/api-reference/text-openai-api |
| MiniMax-M2.5 | MiniMax-M2.5 | minimax | text | text | 204800 | 204800 | — | https://platform.minimax.io/docs/guides/models-intro；https://platform.minimax.io/docs/api-reference/text-anthropic-api；https://platform.minimax.io/docs/api-reference/text-chat-openai；https://platform.minimax.io/docs/api-reference/text-openai-api |
| MiniMax-M2.5-highspeed | MiniMax-M2.5-highspeed | minimax | text | text | 204800 | 204800 | — | https://platform.minimax.io/docs/guides/models-intro；https://platform.minimax.io/docs/api-reference/text-anthropic-api；https://platform.minimax.io/docs/api-reference/text-chat-openai；https://platform.minimax.io/docs/api-reference/text-openai-api |
| MiniMax-M2.7 | MiniMax-M2.7 | minimax | text | text | 204800 | 204800 | — | https://platform.minimax.io/docs/guides/models-intro；https://platform.minimax.io/docs/api-reference/text-anthropic-api；https://platform.minimax.io/docs/api-reference/text-chat-openai；https://platform.minimax.io/docs/api-reference/text-openai-api |
| MiniMax-M2.7-highspeed | MiniMax-M2.7-highspeed | minimax | text | text | 204800 | 204800 | — | https://platform.minimax.io/docs/guides/models-intro；https://platform.minimax.io/docs/api-reference/text-anthropic-api；https://platform.minimax.io/docs/api-reference/text-chat-openai；https://platform.minimax.io/docs/api-reference/text-openai-api |
| MiniMax-M3 | MiniMax-M3 | minimax | text,image,video | text | 1000000 | 524288 | — | https://platform.minimax.io/docs/guides/models-intro；https://platform.minimax.io/docs/api-reference/text-anthropic-api；https://platform.minimax.io/docs/api-reference/text-chat-openai；https://platform.minimax.io/docs/api-reference/text-openai-api |
| codex-auto-review | Codex Auto Review | — | — | — | — | — | — | — |
| deepseek-flash | DeepSeek V4.1 Flash | deepseek | text,image | text | 1000000 | 393216 | low,high,max/high | https://api-docs.deepseek.com/quick_start/pricing/；https://api-docs.deepseek.com/api/create-chat-completion；https://api-docs.deepseek.com/guides/thinking_mode |
| deepseek-v4-flash | DeepSeek V4 Flash | deepseek | text,image | text | 1000000 | 393216 | low,high,max/high | https://api-docs.deepseek.com/quick_start/pricing/；https://api-docs.deepseek.com/api/create-chat-completion；https://api-docs.deepseek.com/guides/thinking_mode |
| deepseek-v4-flash-vision | DeepSeek V4 Flash Vision | deepseek | — | — | — | — | — | https://api-docs.deepseek.com/news/news260910/；https://api-docs.deepseek.com/quick_start/pricing/ |
| deepseek-v4-pro | DeepSeek V4 Pro | deepseek | text | text | 1000000 | 393216 | low,high,max/high | https://api-docs.deepseek.com/quick_start/pricing/；https://api-docs.deepseek.com/api/create-chat-completion；https://api-docs.deepseek.com/guides/thinking_mode |
| deepseek-v4-pro-0813 | DeepSeek V4 Pro (0813) | deepseek | text | text | 1000000 | 393216 | low,high,max/high | https://api-docs.deepseek.com/quick_start/pricing/；https://api-docs.deepseek.com/api/create-chat-completion；https://api-docs.deepseek.com/guides/thinking_mode |
| deepseek-v4.1-flash | DeepSeek V4.1 Flash | deepseek | text,image | text | 1000000 | 393216 | low,high,max/high | https://api-docs.deepseek.com/quick_start/pricing/；https://api-docs.deepseek.com/api/create-chat-completion；https://api-docs.deepseek.com/guides/thinking_mode |
| doubao-seed-2.0-lite | Doubao Seed 2.0 Lite | bytedance | — | — | — | — | — | https://docs.volcengine.com/docs/ark/model-list?lang=zh |
| doubao-seed-2.1-turbo | Doubao Seed 2.1 Turbo | bytedance | — | — | — | — | — | https://docs.volcengine.com/docs/ark/model-list?lang=zh |
| doubao-seed-evolving | Doubao Seed Evolving | bytedance | text,image,video,file | text | 1024000 | 256000 | minimal,low,medium,high,xhigh,max/high | https://docs.volcengine.com/docs/ark/latest-model?lang=zh |
| glm-4-32b | GLM-4-32B-0414 | zai | text | text | 32768 | — | — | https://huggingface.co/zai-org/GLM-4-32B-0414；https://modelscope.cn/models/ZhipuAI/GLM-4-32B-0414/resolve/master/config.json |
| glm-4.5 | GLM-4.5 | zai | text | text | 128000 | 98304 | — | https://docs.bigmodel.cn/cn/guide/models/text/glm-4.5.md；https://docs.bigmodel.cn/cn/guide/start/concept-param.md |
| glm-4.5-air | GLM-4.5-Air | zai | text | text | 128000 | 98304 | — | https://docs.bigmodel.cn/cn/guide/start/model-overview.md；https://docs.bigmodel.cn/cn/guide/start/concept-param.md |
| glm-4.5v | GLM-4.5V | zai | text,image,video,file | text | 64000 | 16384 | — | https://docs.bigmodel.cn/cn/guide/models/vlm/glm-4.5v.md；https://docs.bigmodel.cn/cn/guide/start/concept-param.md |
| glm-4.6 | GLM-4.6 | zai | text | text | 200000 | 131072 | — | https://docs.bigmodel.cn/cn/guide/start/model-overview.md；https://docs.bigmodel.cn/cn/guide/start/concept-param.md |
| glm-4.6v | GLM-4.6V | zai | text,image,video,file | text | 128000 | 32768 | — | https://docs.bigmodel.cn/cn/guide/models/vlm/glm-4.6v.md；https://docs.bigmodel.cn/cn/guide/start/concept-param.md |
| glm-4.7 | GLM-4.7 | zai | text | text | 200000 | 131072 | — | https://docs.bigmodel.cn/cn/guide/start/model-overview.md；https://docs.bigmodel.cn/cn/guide/start/concept-param.md |
| glm-4.7-flash | GLM-4.7-Flash | zai | text | text | 200000 | 131072 | — | https://docs.bigmodel.cn/cn/guide/start/model-overview.md；https://docs.bigmodel.cn/cn/guide/start/concept-param.md；https://docs.bigmodel.cn/cn/guide/models/free/glm-4.7-flash.md |
| glm-5 | GLM-5 | zai | text | text | 200000 | 131072 | — | https://docs.bigmodel.cn/cn/guide/start/model-overview.md；https://docs.bigmodel.cn/cn/guide/start/concept-param.md |
| glm-5-turbo | GLM-5-Turbo | zai | text | text | 200000 | 131072 | — | https://docs.bigmodel.cn/cn/guide/start/model-overview.md；https://docs.bigmodel.cn/cn/guide/start/concept-param.md |
| glm-5.1 | GLM-5.1 | zai | text | text | 200000 | 131072 | — | https://docs.bigmodel.cn/cn/guide/start/model-overview.md；https://docs.bigmodel.cn/cn/guide/start/concept-param.md |
| glm-5.2 | GLM-5.2 | zai | text | text | 1000000 | 131072 | low,medium,high,xhigh,max/max | https://docs.bigmodel.cn/cn/guide/start/model-overview.md；https://docs.bigmodel.cn/cn/guide/start/concept-param.md |
| glm-5.2-fast-preview | GLM-5.2 Fast Preview | zai | — | — | — | — | — | https://docs.z.ai/guides/llm/glm-5.2.md；https://docs.z.ai/release-notes/new-released.md |
| glm-5.3 | GLM-5.3 | zai | text | text | 1000000 | 131072 | low,high,max/max | https://docs.bigmodel.cn/cn/guide/start/model-overview.md；https://docs.bigmodel.cn/cn/guide/start/concept-param.md；https://docs.bigmodel.cn/cn/guide/models/text/glm-5.3.md |
| glm-5.3-flash | GLM-5.3-Flash | zai | text,image,video,file | text | 1000000 | 131072 | low,high,max/max | https://docs.bigmodel.cn/cn/guide/start/model-overview.md；https://docs.bigmodel.cn/cn/guide/start/concept-param.md；https://docs.bigmodel.cn/cn/guide/models/text/glm-5.3.md；https://docs.bigmodel.cn/cn/guide/models/vlm/glm-5.3-flash.md |
| glm-5.3-flashx | GLM-5.3-FlashX | zai | text,image,video,file | text | 1000000 | 131072 | low,high,max/max | https://docs.bigmodel.cn/cn/guide/start/model-overview.md；https://docs.bigmodel.cn/cn/guide/start/concept-param.md；https://docs.bigmodel.cn/cn/guide/models/text/glm-5.3.md；https://docs.bigmodel.cn/cn/guide/models/vlm/glm-5.3-flash.md |
| glm-5v-turbo | GLM-5V-TURBO | zai | text,image,video,file | text | 200000 | 131072 | — | https://docs.bigmodel.cn/cn/guide/models/vlm/glm-5v-turbo.md；https://docs.bigmodel.cn/cn/guide/start/concept-param.md |
| gpt-4o-audio-preview | GPT-4o Audio Preview | openai | text,audio | text,audio | 128000 | 16384 | — | https://developers.openai.com/api/docs/models/gpt-4o-audio-preview.md |
| gpt-4o-realtime-preview | GPT-4o Realtime Preview | openai | text,audio | text,audio | 32000 | 4096 | — | https://developers.openai.com/api/docs/models/gpt-4o-realtime-preview.md |
| gpt-5.2 | GPT-5.2 | openai | text,image | text | 400000 | 128000 | low,medium,high,xhigh/none | https://developers.openai.com/api/docs/models/gpt-5.2.md |
| gpt-5.2-2025-12-11 | GPT-5.2 (2025-12-11) | openai | text,image | text | 400000 | 128000 | low,medium,high,xhigh/none | https://developers.openai.com/api/docs/models/gpt-5.2.md |
| gpt-5.2-chat-latest | GPT-5.2 Chat | openai | text,image | text | 128000 | 16384 | — | https://developers.openai.com/api/docs/models/gpt-5.2-chat-latest.md |
| gpt-5.2-pro | GPT-5.2 Pro | openai | text,image | text | 400000 | 128000 | medium,high,xhigh | https://developers.openai.com/api/docs/models/gpt-5.2-pro.md |
| gpt-5.2-pro-2025-12-11 | GPT-5.2 Pro (2025-12-11) | openai | text,image | text | 400000 | 128000 | medium,high,xhigh | https://developers.openai.com/api/docs/models/gpt-5.2-pro.md |
| gpt-5.3-codex-spark | GPT-5.3-Codex-Spark | openai | text | text | 128000 | — | — | https://openai.com/index/introducing-gpt-5-3-codex-spark/ |
| gpt-5.4 | GPT-5.4 | openai | text,image | text | 1050000 | 128000 | low,medium,high,xhigh/none | https://developers.openai.com/api/docs/models/gpt-5.4.md |
| gpt-5.4-2026-03-05 | GPT-5.4 (2026-03-05) | openai | text,image | text | 1050000 | 128000 | low,medium,high,xhigh/none | https://developers.openai.com/api/docs/models/gpt-5.4.md |
| gpt-5.4-mini | GPT-5.4 Mini | openai | text,image | text | 400000 | 128000 | low,medium,high,xhigh/none | https://developers.openai.com/api/docs/models/gpt-5.4-mini.md |
| gpt-5.4-mini-openai-compact | GPT-5.4 Mini (OpenAI Compact) | openai | — | — | — | — | — | — |
| gpt-5.4-openai-compact | GPT-5.4 (OpenAI Compact) | openai | — | — | — | — | — | — |
| gpt-5.5 | GPT-5.5 | openai | text,image | text | 1050000 | 128000 | low,medium,high,xhigh/medium | https://developers.openai.com/api/docs/models/gpt-5.5.md |
| gpt-5.5-openai-compact | GPT-5.5 (OpenAI Compact) | openai | — | — | — | — | — | — |
| gpt-5.6 | GPT-5.6 | openai | text,image | text | 1050000 | 128000 | low,medium,high,xhigh,max/medium | https://developers.openai.com/api/docs/models/gpt-5.6-sol.md |
| gpt-5.6-luna | GPT-5.6 Luna | openai | text,image | text | 1050000 | 128000 | low,medium,high,xhigh,max/medium | https://developers.openai.com/api/docs/models/gpt-5.6-luna.md |
| gpt-5.6-sol | GPT-5.6 Sol | openai | text,image | text | 1050000 | 128000 | low,medium,high,xhigh,max/medium | https://developers.openai.com/api/docs/models/gpt-5.6-sol.md |
| gpt-5.6-terra | GPT-5.6 Terra | openai | text,image | text | 1050000 | 128000 | low,medium,high,xhigh,max/medium | https://developers.openai.com/api/docs/models/gpt-5.6-terra.md |
| gpt-6 | GPT-6 | openai | — | — | — | — | — | — |
| gpt-6-astra | GPT-6 Astra | openai | text,image | text | 1050000 | 128000 | low,medium,high,xhigh,max | https://developers.openai.com/api/docs/models/gpt-6-astra.md；https://developers.openai.com/api/docs/guides/latest-model.md?model=gpt-6-astra；https://developers.openai.com/api/docs/guides/reasoning.md |
| gpt-6-luna | GPT-6 Luna | openai | text,image | text | 1050000 | 128000 | low,medium,high,xhigh,max/medium | https://developers.openai.com/api/docs/models/gpt-6-luna.md |
| gpt-6-sol | GPT-6 Sol | openai | text,image | text | 1050000 | 128000 | low,medium,high,xhigh,max/medium | https://developers.openai.com/api/docs/models/gpt-6-sol.md |
| gpt-6.1-sol | GPT-6.1 Sol | openai | text,image | text | 1050000 | 128000 | low,medium,high,xhigh,max/medium | https://developers.openai.com/api/docs/models/gpt-6.1-sol.md |
| gpt-image-1 | GPT Image 1 | openai | text,image | image | — | — | — | https://developers.openai.com/api/docs/models/gpt-image-1.md |
| gpt-image-1.5 | GPT Image 1.5 | openai | text,image | image,text | — | — | — | https://developers.openai.com/api/docs/models/gpt-image-1.5.md |
| gpt-image-2 | GPT Image 2 | openai | text,image | image | — | — | — | https://developers.openai.com/api/docs/models/gpt-image-2.md |
| gpt-image-2.5 | GPT Image 2.5 | openai | — | — | — | — | — | — |
| gpt-image-2.5-flare | GPT Image 2.5 Flare | openai | text,image | image | — | — | — | https://developers.openai.com/api/docs/models/gpt-image-2.5-flare.md |
| gpt-image-2.5-sunburst | GPT Image 2.5 Sunburst | openai | text,image | image | — | — | — | https://developers.openai.com/api/docs/models/gpt-image-2.5-sunburst.md |
| gpt-reserve | GPT Reserve | openai | — | — | — | — | — | — |
| grok | Grok | xai | — | — | — | — | — | https://docs.x.ai/developers/models |
| grok-4 | Grok 4 | xai | text,image | text | 256000 | — | — | https://x.ai/news/grok-4 |
| grok-4-0709 | Grok 4 (0709) | xai | — | — | — | — | — | https://x.ai/news/grok-4；https://docs.x.ai/developers/models |
| grok-4.20-0309-non-reasoning | Grok 4.20 Non-Reasoning (0309) | xai | text,image | text | 1000000 | — | — | https://docs.x.ai/developers/models/grok-4.20-beta-0309-non-reasoning.md |
| grok-4.20-0309-reasoning | Grok 4.20 Reasoning (0309) | xai | text,image | text | 1000000 | — | — | https://docs.x.ai/developers/models/grok-4.20-reasoning.md |
| grok-4.20-multi-agent-0309 | Grok 4.20 Multi-Agent Beta (0309) | xai | text,image | text | 1000000 | — | low,medium,high,xhigh | https://docs.x.ai/developers/models/grok-4.20-multi-agent-0309.md；https://docs.x.ai/developers/model-capabilities/text/reasoning.md；https://docs.x.ai/developers/model-capabilities/text/multi-agent.md |
| grok-4.20-non-reasoning | Grok 4.20 Non-Reasoning | xai | text,image | text | 1000000 | — | — | https://docs.x.ai/developers/models/grok-4.20-beta-0309-non-reasoning.md |
| grok-4.20-reasoning | Grok 4.20 Reasoning | xai | text,image | text | 1000000 | — | — | https://docs.x.ai/developers/models/grok-4.20-reasoning.md |
| grok-4.3 | Grok 4.3 | xai | text,image | text | 1000000 | — | low,medium,high,xhigh/low | https://docs.x.ai/developers/models/grok-4.3.md |
| grok-4.5 | Grok 4.5 | xai | text,image | text | 500000 | — | low,medium,high/high | https://docs.x.ai/developers/models/grok-4.5.md；https://docs.x.ai/developers/model-capabilities/text/reasoning.md；https://docs.x.ai/developers/release-notes.md |
| grok-4.5-latest | Grok 4.5 Latest | xai | text,image | text | 500000 | — | low,medium,high/high | https://docs.x.ai/developers/models/grok-4.5.md；https://docs.x.ai/developers/model-capabilities/text/reasoning.md；https://docs.x.ai/developers/release-notes.md |
| grok-4.6 | Grok 4.6 | xai | text,image | text | 500000 | — | low,medium,high,xhigh/high | https://docs.x.ai/developers/models/grok-4.6.md；https://docs.x.ai/developers/release-notes.md |
| grok-build | Grok Build | xai | — | — | — | — | — | https://docs.x.ai/developers/models；https://docs.x.ai/developers/models/grok-build-0.1.md；https://docs.x.ai/developers/models/grok-4.5.md |
| grok-build-0.1 | Grok Build 0.1 | xai | text,image | text | 256000 | — | — | https://docs.x.ai/developers/models/grok-build-0.1.md |
| grok-build-latest | Grok Build Latest (Grok 4.5) | xai | text,image | text | 500000 | — | low,medium,high/high | https://docs.x.ai/developers/models/grok-4.5.md；https://docs.x.ai/developers/model-capabilities/text/reasoning.md；https://docs.x.ai/developers/release-notes.md |
| hy3 | Hy3 | tencent | text | text | 256000 | — | — | https://github.com/Tencent-Hunyuan/Hy3 |
| hy3-preview | Hy3 Preview | tencent | text | text | 256000 | — | — | https://github.com/Tencent-Hunyuan/Hy3-preview |
| hy4-preview | Hy4 Preview | tencent | text | text | 1000000 | — | — | https://github.com/Tencent-Hunyuan/Hy4-preview |
| kimi-k2.5 | Kimi K2.5 | moonshot | text,image,video | text | 262144 | — | — | https://www.kimi.com/blog/kimi-k2-5；https://platform.kimi.com/docs/models |
| kimi-k2.6 | Kimi K2.6 | moonshot | text,image,video | text | 262144 | — | — | https://platform.kimi.com/docs/guide/kimi-k2-6-quickstart |
| kimi-k2.7-code | Kimi K2.7 Code | moonshot | text,image,video | text | 262144 | — | — | https://platform.kimi.com/docs/guide/kimi-k2-7-code-quickstart |
| kimi-k2.8-preview | Kimi K2.8 Preview | moonshot | — | — | — | — | — | https://platform.kimi.ai/docs/models.md；https://platform.kimi.ai/docs/overview |
| kimi-k3 | Kimi K3 | moonshot | text,image,video | text | 1048576 | 1048576 | low,high,max/max | https://platform.kimi.com/docs/guide/kimi-k3-quickstart；https://platform.kimi.com/docs/api/chat.md；https://platform.kimi.com/docs/api/models-overview.md；https://platform.kimi.com/docs/guide/use-reasoning-effort.md |
| kimi-k3-256k | Kimi K3 256K | moonshot | — | — | — | — | — | https://platform.kimi.ai/docs/models.md；https://platform.kimi.ai/docs/overview |
| laguna-s-2.1 | Laguna S 2.1 | poolside | text | text | 1000000 | — | — | https://docs.poolside.ai/get-started/supported-models.md；https://docs.poolside.ai/api/overview.md |
| longcat-2.0 | LongCat 2.0 | meituan | text | text | 1000000 | — | — | https://huggingface.co/meituan-longcat/LongCat-2.0；https://longcat.chat/blog/longcat-2.0 |
| mimo-v2-omni | MiMo V2 Omni | xiaomi | text,image,audio,video | — | — | — | — | https://mimo.xiaomi.com/mimo-v2-omni；https://mimo.mi.com/docs/zh-CN/updates/deprecate |
| mimo-v2-pro | MiMo V2 Pro | xiaomi | text | text | 1000000 | — | — | https://mimo.xiaomi.com/mimo-v2-pro；https://mimo.mi.com/docs/zh-CN/updates/deprecate |
| mimo-v2.5 | MiMo V2.5 | xiaomi | text,image,audio,video | text | 1000000 | 128000 | — | https://mimo.mi.com/models/en-US/mimo-v2.5；https://mimo.mi.com/docs/en-US/quick-start/summary/model；https://mimo.mi.com/docs/en-US/quick-start/usage-guide/text-generation/deep-thinking |
| mimo-v2.5-pro | MiMo V2.5 Pro | xiaomi | text | text | 1000000 | 128000 | — | https://mimo.mi.com/models/en-US/mimo-v2.5-pro；https://mimo.mi.com/docs/en-US/quick-start/summary/model；https://mimo.mi.com/docs/en-US/quick-start/usage-guide/text-generation/deep-thinking |
| mimo-v2.6-flash | MiMo V2.6 Flash | xiaomi | text,image,audio,video | text | 1000000 | 128000 | — | https://mimo.mi.com/models/en-US/mimo-v2.6-flash；https://mimo.mi.com/docs/en-US/quick-start/summary/model；https://mimo.mi.com/docs/en-US/quick-start/usage-guide/text-generation/deep-thinking |
| mimo-v2.6-pro | MiMo V2.6 Pro | xiaomi | text,image,audio,video | text | 1000000 | 128000 | — | https://mimo.mi.com/models/en-US/mimo-v2.6-pro；https://mimo.mi.com/docs/en-US/quick-start/summary/model；https://mimo.mi.com/docs/en-US/quick-start/usage-guide/text-generation/deep-thinking |
| mistral-large-3 | Mistral Large 3 | mistral | text,image | text | 256000 | — | — | https://docs.mistral.ai/models/mistral-large-3-25-12 |
| muse-spark-1.2-contributor | Muse Spark 1.2 Contributor | meta | — | — | — | — | — | https://developer.meta.com/ai/；https://ai.meta.com/blog/introducing-muse-spark-msl |
| nemotron-3-super | NVIDIA Nemotron 3 Super | nvidia | text | text | 1000000 | — | — | https://build.nvidia.com/nvidia/nemotron-3-super-120b-a12b/modelcard；https://huggingface.co/nvidia/NVIDIA-Nemotron-3-Super-120B-A12B-BF16 |
| nemotron-3-ultra | NVIDIA Nemotron 3 Ultra | nvidia | text | text | — | — | — | https://huggingface.co/nvidia/NVIDIA-Nemotron-3-Ultra-550B-A55B-BF16 |
| qwen-vl-ocr | Qwen-vl-ocr | qwen | text,image | text | 38192 | 8192 | — | https://help.aliyun.com/en/model-studio/qwenvl-ocr |
| qwen3.5-omni-flash | Qwen3.5-omni-flash | qwen | text,image,audio,video | text,audio | 262144 | 65536 | — | https://help.aliyun.com/en/model-studio/qwen3-5-omni-flash |
| qwen3.5-omni-plus | Qwen3.5-omni-plus | qwen | text,image,audio,video | text,audio | 262144 | 65536 | — | https://help.aliyun.com/en/model-studio/qwen3-5-omni-plus |
| qwen3.5-plus | Qwen3.5-plus | qwen | text,image,video | text | 1000000 | 65536 | — | https://help.aliyun.com/en/model-studio/qwen3-5-plus |
| qwen3.6-flash | Qwen3.6-flash | qwen | text,image,video | text | 1000000 | 65536 | — | https://help.aliyun.com/en/model-studio/qwen3-6-flash |
| qwen3.6-max-preview | Qwen3.6-max-preview | qwen | text | text | 262144 | 65536 | — | https://help.aliyun.com/en/model-studio/qwen3-6-max |
| qwen3.6-plus | Qwen3.6-plus | qwen | text,image,video | text | 1000000 | 65536 | — | https://help.aliyun.com/en/model-studio/qwen3-6-plus |
| qwen3.7-flash | Qwen3.7-flash | qwen | text,image,video | text | 1000000 | 131072 | — | https://help.aliyun.com/en/model-studio/qwen3-7-flash |
| qwen3.7-max | Qwen3.7-max | qwen | text | text | 1000000 | 131072 | — | https://help.aliyun.com/en/model-studio/qwen3-7-max |
| qwen3.7-plus | Qwen3.7-plus | qwen | text,image,video | text | 1000000 | 131072 | — | https://help.aliyun.com/en/model-studio/qwen3-7-plus |
| qwen3.8-27b | Qwen3.8-27B | qwen | text,image,video | text | 262144 | — | low,medium,xhigh/xhigh | https://huggingface.co/Qwen/Qwen3.8-27B；https://modelscope.cn/models/Qwen/Qwen3.8-27B/resolve/master/README.md |
| qwen3.8-flash | Qwen3.8-flash | qwen | text,image,video | text | 1000000 | 131072 | — | https://help.aliyun.com/en/model-studio/qwen3-8-flash |
| qwen3.8-max | Qwen3.8-max | qwen | text,image,video | text | 1000000 | 131072 | — | https://help.aliyun.com/en/model-studio/qwen3-8-max |
| seed-2.1-pro | Doubao Seed 2.1 Pro | bytedance | — | — | — | — | — | https://docs.volcengine.com/docs/ark/model-list?lang=zh |
| seed-2.1-turbo | Doubao Seed 2.1 Turbo | bytedance | — | — | — | — | — | https://docs.volcengine.com/docs/ark/model-list?lang=zh |
| step-3.5-flash | Step 3.5 Flash | stepfun | text | text | 256000 | — | — | https://platform.stepfun.com/docs/zh/guides/models/step-3.5-flash |
| step-3.5-flash-2603 | Step 3.5 Flash (2603) | stepfun | text | text | 256000 | — | low,high | https://platform.stepfun.com/docs/zh/guides/models/step-3.5-flash.md；https://platform.stepfun.com/docs/zh/api-reference/chat/chat-completion-create.md |
| step-3.7-flash | Step 3.7 Flash | stepfun | text,image,video | text | 256000 | — | low,medium,high/medium | https://platform.stepfun.com/docs/zh/guides/models/step-3.7-flash |
| step-5-preview | Step 5 Preview | stepfun | text,image,video | text | 1000000 | 64000 | low,medium,high | https://platform.stepfun.com/docs/zh/guides/models/step-5-preview |
| step-image-edit-2 | Step Image Edit 2 | stepfun | text,image | image | — | — | — | https://platform.stepfun.com/docs/zh/guides/models/step-image-edit-2 |
| step-router-v1 | Step Router V1 | stepfun | — | — | — | — | — | https://platform.stepfun.com/docs/zh/guides/models/step-router |
| stepaudio-2.5-asr | StepAudio 2.5 ASR | stepfun | audio | text | — | — | — | https://platform.stepfun.com/docs/zh/guides/models/stepaudio-2.5-asr |
| stepaudio-2.5-chat | StepAudio 2.5 Chat | stepfun | text,audio | text | — | — | — | https://platform.stepfun.com/docs/zh/guides/models/stepaudio-2.5-chat |
| stepaudio-2.5-realtime | StepAudio 2.5 Realtime | stepfun | text,audio | text,audio | — | — | — | https://platform.stepfun.com/docs/zh/guides/models/stepaudio-2.5-realtime |
| stepaudio-2.5-tts | StepAudio 2.5 TTS | stepfun | text,audio | audio | — | — | — | https://platform.stepfun.com/docs/zh/guides/models/stepaudio-2.5-tts |

## 推理档位补齐

`/v1/models` 只返回数据库里已配置的 `reasoning_efforts`。下面这些调用名此前没有配置，本轮补齐。迁移 `109_model_metadata_reasoning_efforts.sql` 负责 GPT 侧，`110_model_metadata_qwen_reasoning_efforts.sql` 负责 Qwen3.8 云端型号，`111_model_metadata_reasoning_efforts_correction.sql` 撤销其中一条被官方证据否定的配置。三条迁移都只更新 `reasoning_efforts is null` 的行（或精确匹配写入值），不会覆盖人工修改。

GPT 侧要区分两类调用名。`gpt-5.4-openai-compact`、`gpt-5.4-mini-openai-compact`、`gpt-5.5-openai-compact`、`gpt-6`、`gpt-5.3-codex-spark` 在 OpenAI 官方文档中是 **404，不在官方模型目录**，属于平台或上游的内部命名；它们的档位按对应基础型号的已核实值填写，属于同族推断而不是逐 ID 的官方声明。

| 模型 | 档位 | 默认 | 依据 |
| --- | --- | --- | --- |
| `gpt-6` | low,medium,high,xhigh,max | medium | 内部别名，沿用 `gpt-6-luna`、`gpt-6-sol`、`gpt-6-astra`、`gpt-6.1-sol` 的已核实档位 |
| `gpt-5.5-openai-compact` | low,medium,high,xhigh | medium | 内部别名，沿用 `gpt-5.5` |
| `gpt-5.4-openai-compact` | low,medium,high,xhigh | none | 内部别名，沿用 `gpt-5.4` |
| `gpt-5.4-mini-openai-compact` | low,medium,high,xhigh | none | 内部别名，沿用 `gpt-5.4-mini` |
| `gpt-5.3-codex-spark` | low,medium,high,xhigh | — | 内部别名，按 Codex 系列档位，未声明默认值 |
| `qwen3.8-flash` | low,medium,xhigh | xhigh | 百炼 Chat Completions 参数说明（官方） |
| `qwen3.8-max` | low,medium,xhigh | xhigh | 百炼 Chat Completions 参数说明（官方） |

`gpt-5.2-chat-latest` 曾被按 `gpt-5.2` 同族写入档位，随后由官方模型页原文否定并撤销：该页面存在且可读，但正文与 Supported features 完全没有 reasoning / reasoning.effort 内容，而另一个 ID `gpt-5.2` 的页面才写明 `none (default), low, medium, high and xhigh`。chat 变体因此保持未配置。

仍未配置的 GPT 调用名及原因：`gpt-image-1`、`gpt-image-1.5`、`gpt-image-2`、`gpt-image-2.5`、`gpt-image-2.5-flare`、`gpt-image-2.5-sunburst` 是图像生成模型，没有文本推理档位；`gpt-4o-audio-preview`、`gpt-4o-realtime-preview` 的官方模型页明确未列推理档位；`gpt-reserve` 官方目录中没有该 ID，也没有可核实的来源。

xAI 侧同样保持未配置：官方 reasoning 能力页只对 `grok-4.7`、`grok-4.6`、`grok-4.5`（low/medium/high 默认/xhigh）和 `grok-4.20-multi-agent`（effort 控制 agent 数量）给出档位；`grok-4.20-0309-reasoning` 只写「Reasoning: Yes」而无档位，`grok-4.20-0309-non-reasoning` 明确「Reasoning: No」，`grok`、`grok-4`、`grok-4-0709`、`grok-build` 的页面为 404。

同样按官方文档确认「只有思考开关、没有离散档位」而保持未配置的还有：GLM-4.5/4.6/4.7/5/5.1 等（智谱仅 GLM-5.2 及以上支持 `reasoning_effort`）、Kimi K2.5/K2.6/K2.7-code（K2.x 用 `thinking`）、MiniMax M2.x/M3（`thinking` 开关）、MiMo v2.5/v2.6（`thinking.type`）、Qwen3.5/3.6/3.7 与 Omni 系列（`enable_thinking` 开关）、Step 3.5 Flash 基础版、StepAudio 与 Step Image 系列。

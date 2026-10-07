update model_catalog_metadata
set description = case model
  when 'MiniMax-M2' then 'MiniMax M2 是语言模型，官方列出 200K 上下文、128K 最大输出，并强调 Agent 能力、Function Calling、高级推理和实时流式输出。它适合工具驱动的多步任务与代码、推理工作流。'
  when 'MiniMax-M2.1' then 'MiniMax M2.1 为 230B 总参数、单次推理激活 10B 的模型，官方定位代码生成与重构，强调多语言编程、精准重构和增强推理。'
  when 'MiniMax-M2.1-highspeed' then 'MiniMax M2.1-highspeed 与 M2.1 保持相同效果，官方定位为更快、更低延迟的推理版本，同时保留多语言编程与代码重构能力。'
  when 'MiniMax-M2.5' then 'MiniMax M2.5 官方定位为代码生成与重构模型，强调在复杂任务中兼顾性能与成本效率，适合软件工程、代码理解、重构和修复。'
  when 'MiniMax-M2.5-highspeed' then 'MiniMax M2.5-highspeed 官方声明与 M2.5 效果相同但推理更快，面向多语言代码、精准重构和低延迟开发 Agent。'
  when 'MiniMax-M2.7' then 'MiniMax M2.7 以递归自我改进为定位，覆盖真实工程、专业办公交付和角色化交互；官方模型表未额外承诺视觉输入输出。'
  when 'MiniMax-M2.7-highspeed' then 'MiniMax M2.7-highspeed 官方说明与 M2.7 效果相同、推理更快，面向多语言代码、精准重构以及高吞吐工程和办公 Agent。'
  when 'MiniMax-M3' then 'MiniMax M3 是原生多模态编码模型，官方列出最高 1M 上下文、图像和视频输入，并支持桌面操作；重点覆盖 Bug 修复、前后端开发、性能优化及搜索和 Office Agent。'
  when 'codex-auto-review' then 'Xinghai 的代码审查路由，服务于变更检查、缺陷定位、风险说明和修复建议。它是平台路由名，底层模型由当前审查配置决定。'
  when 'deepseek-v4-flash' then 'DeepSeek 官方当前将这个旧标识路由到 DeepSeek-V4.1-Flash；它是更快、更经济的 V4 兼容入口，不应被展示为独立的现行版本。'
  when 'deepseek-v4-pro' then 'DeepSeek-V4-Pro 面向高强度推理、Agent Coding 与复杂任务，官方资料强调世界知识、数学与 STEM、编码和 Agent 能力；当前模型表列出 1M 上下文，输入为文本。'
  when 'deepseek-v4.1-flash' then 'DeepSeek-V4.1-Flash 是原生多模态的高吞吐 Flash 模型，官方 API 当前推荐使用 deepseek-flash；这个名称应标为兼容别名，而不是独立的稳定调用名。'
  when 'doubao-seed-2.0-lite' then '豆包 Seed 2.0 Lite 面向高频文本处理，把响应速度和调用成本放在优先位置。摘要、分类、改写和常规问答比长链路复杂推理更适合交给它。'
  when 'doubao-seed-2.1-turbo' then '豆包 Seed 2.1 Turbo 是偏实时交互的通用版本，适合信息抽取、内容生成和开发辅助等需要快速返回的请求。'
  when 'doubao-seed-evolving' then '豆包 Seed Evolving 是演进中的版本标识，具体能力随火山方舟上的上游配置变化。使用它适合不要求锁定某个固定快照的持续迭代场景。'
  when 'glm-4-32b' then '当前智谱官方模型概览未找到精确的 glm-4-32b 条目；这是平台别名或未核实名称，不能依据名称推断参数、模态或能力。'
  when 'glm-4.5' then 'GLM-4.5 将深度推理、代码生成和工具调用结合在同一个文本模型中，适合复杂分析和需要分步执行的工作流。'
  when 'glm-4.5-air' then 'GLM-4.5 Air 是 GLM-4.5 系列的轻量版本，面向日常对话、信息处理、代码辅助和更高并发的应用。'
  when 'glm-4.5v' then 'GLM-4.5V 是智谱的视觉语言模型，可把图片和视频内容纳入对话，适合图像问答、文档分析和视觉推理。'
  when 'glm-4.6' then 'GLM-4.6 侧重长文本理解、复杂推理、编程和工具使用，适合需要把资料分析与执行动作连起来的任务。'
  when 'glm-4.6v' then 'GLM-4.6V 面向视觉问答与多模态推理，可处理图片和视频中的文字、结构与语义信息。'
  when 'glm-4.7' then 'GLM-4.7 是 GLM 系列的旗舰文本路线，重点覆盖软件工程、复杂推理、工具调用和专业写作。'
  when 'glm-4.7-flash' then 'GLM-4.7 Flash 是偏速度的 GLM-4.7 路线，适合代码补全、实时问答、批量文本处理和轻量 Agent。'
  when 'glm-5' then 'GLM-5 面向复杂系统工程和长程 Agent 任务，适合连续代码修改、工具协作以及需要较强规划能力的请求。'
  when 'glm-5-turbo' then 'GLM-5 Turbo 是面向实时应用的高速路线，在对话、代码辅助和 Agent 调用之间取更低的交互延迟。'
  when 'glm-5.1' then 'GLM-5.1 面向推理与编程，适合复杂分析、软件工程和需要多轮拆解的知识工作。'
  when 'glm-5.2' then 'GLM-5.2 侧重长上下文下的项目级代码理解、复杂系统工程和持续 Agent 工作流。'
  when 'glm-5.2-fast-preview' then '当前智谱官方模型概览列出 GLM-5.2，但未找到精确的 GLM-5.2-Fast-Preview 条目；该名称应标为平台别名或官方资料无法核实，不应从 GLM-5.2 推断性能。'
  when 'glm-5.3' then 'GLM-5.3 面向复杂编程、长程推理和多步骤工具协作，定位更接近工程交付而非简单问答。'
  when 'glm-5.3-flash' then 'GLM-5.3 Flash 把 GLM-5.3 的使用场景压缩到更快的交互节奏，适合代码补全、批量处理和轻量 Agent。'
  when 'glm-5v-turbo' then 'GLM-5V Turbo 是偏实时的视觉语言路线，适合截图问答、文档识别、图表理解和视觉辅助编程。'
  when 'gpt-4o-audio-preview' then 'GPT-4o Audio Preview 是 OpenAI 的弃用预览模型，接收文本和音频并输出文本和音频，面向 Chat Completions 的音频输入输出。'
  when 'gpt-4o-realtime-preview' then 'GPT-4o Realtime Preview 是 OpenAI 的弃用预览模型，可通过 WebRTC 或 WebSocket 实时处理文本和音频，并提供 function calling。'
  when 'gpt-5.2' then 'GPT-5.2 是 OpenAI 的通用推理路线，面向复杂分析、代码工程、专业写作和工具调用。'
  when 'gpt-5.2-2025-12-11' then 'GPT-5.2 的固定日期快照，用于回归测试和生产流程中的版本锁定；它不会随着 latest 别名自动漂移。'
  when 'gpt-5.2-chat-latest' then 'GPT-5.2 Chat Latest 是面向对话场景的滚动别名，版本由上游持续更新，适合不要求固定快照的助手应用。'
  when 'gpt-5.2-pro' then 'GPT-5.2 Pro 面向高要求的研究、工程和长步骤分析，把更高质量的推理放在响应速度之前。'
  when 'gpt-5.2-pro-2025-12-11' then 'GPT-5.2 Pro 的固定日期快照，适合需要可复现推理结果的研究、审查和生产流程。'
  when 'gpt-5.3-codex-spark' then '官方模型目录未列出这个精确 ID；当前记录应视为平台路由别名，不能直接把 GPT-5.3 Codex 的官方能力套用到它，实际目标以当前路由配置为准。'
  when 'gpt-5.4' then 'GPT-5.4 是 OpenAI 的旗舰专业工作模型，接收文本和图像并输出文本，官方列出 web/file search、computer use、code interpreter 和 MCP 等工具。'
  when 'gpt-5.4-2026-03-05' then 'GPT-5.4 的固定日期版本，用于锁定模型行为和回归结果；需要稳定版本时选择它比使用滚动别名更可控。'
  when 'gpt-5.4-mini' then 'GPT-5.4 Mini 将 GPT-5.4 的推理与编程能力收敛到更快、更省资源的路线，适合高频助手、摘要、分类和代码辅助。'
  when 'gpt-5.4-mini-openai-compact' then 'OpenAI Compact 路由中的 GPT-5.4 Mini 变体，优先服务低延迟文本任务；它是平台路由标识，不是独立公开型号。'
  when 'gpt-5.4-openai-compact' then 'OpenAI Compact 路由中的 GPT-5.4 变体，适合对响应速度敏感的通用问答、内容处理和开发辅助。'
  when 'gpt-5.5' then 'GPT-5.5 面向复杂推理、代码工程、专业写作和多步骤工具协作，适合需要持续推进的知识工作。'
  when 'gpt-5.5-openai-compact' then 'OpenAI Compact 路由中的 GPT-5.5 变体，偏向交互速度和高频调用；底层版本由当前路由配置决定。'
  when 'gpt-5.6' then 'GPT-5.6 是平台当前提供的高级通用路线，覆盖软件开发、知识工作、内容创作和工具协作。'
  when 'gpt-5.6-luna' then 'Xinghai 的 Luna 路由别名，当前请求会交给 GPT-5.6 系列配置的上游目标；它不是 OpenAI 单独发布的型号。'
  when 'gpt-5.6-sol' then 'Xinghai 的 Sol 路由别名，面向文本分析、编程和多步骤工作流；实际模型版本以平台路由配置为准。'
  when 'gpt-5.6-terra' then 'Xinghai 的 Terra 路由别名，面向通用对话、代码辅助和工具调用；它不锁定某个公开模型快照。'
  when 'gpt-image-1' then 'GPT Image 1 是 OpenAI 的弃用图像生成模型，接收文本和图像并生成图像，适合图像创作与编辑。'
  when 'gpt-image-1.5' then 'GPT Image 1.5 是 OpenAI 的弃用图像生成模型，官方强调更好的指令遵循和提示词贴合，支持文本、图像输入以及图像、文本输出。'
  when 'gpt-image-2' then 'GPT Image 2 是 OpenAI 当前的高质量图像生成模型，面向快速生成与编辑，支持灵活尺寸和高保真图像输入。'
  when 'grok' then 'xAI 官方模型目录未找到这个精确 ID；当前记录应视为平台别名，不能据名称推断具体 Grok 版本或能力。'
  when 'grok-4' then 'Grok 4 的官方定位包含 frontier-level 多模态理解、256K 上下文和 advanced reasoning，适合复杂分析与开发任务。'
  when 'grok-4-0709' then 'Grok 4 的固定日期变体，沿用官方发布的多模态理解、256K 上下文和 advanced reasoning 定位；当前是否可调用以 xAI 模型目录为准。'
  when 'grok-4.20-0309-non-reasoning' then 'Grok 4.20 的 0309 非推理版本，官方强调速度和 agentic tool calling，支持文本、图像输入、文本输出与 1M 上下文。'
  when 'grok-4.20-0309-reasoning' then 'Grok 4.20 的 0309 推理版本，官方列出文本、图像输入、文本输出和 1M 上下文，并面向 reasoning 与 agentic tool calling。'
  when 'grok-4.20-multi-agent-0309' then 'Grok 4.20 的 0309 multi-agent 版本，官方目录列出该精确型号；这里仅标注其多代理定位，不延伸未核实的额外能力。'
  when 'grok-4.20-non-reasoning' then '官方别名，指向 Grok 4.20 的 0309 非推理版本；具体能力沿用 xAI 当前别名目标，不锁定独立模型。'
  when 'grok-4.20-reasoning' then '官方别名，指向 Grok 4.20 的 0309 推理版本；具体能力沿用 xAI 当前别名目标，不锁定独立模型。'
  when 'grok-4.3' then 'xAI 官方目录列出的 Grok 4.3 文本模型，提供 1M 上下文；官方页面未进一步承诺特定任务定位。'
  when 'grok-4.5' then 'xAI 将 Grok 4.5 定位为面向 coding、agentic tasks 和 knowledge work 的模型。'
  when 'grok-4.5-latest' then 'Grok 4.5 的滚动别名，沿用 xAI 对 Grok 4.5 的 coding、agentic tasks 和 knowledge work 定位，不锁定具体版本。'
  when 'grok-4.6' then 'Grok 4.6 是 xAI 当前模型目录中的 Grok 系列型号；具体输入输出和任务边界以官方模型页及当前通道配置为准。'
  when 'grok-build' then 'xAI 官方模型目录未列出这个精确 ID；当前记录应视为平台编程路由别名，实际底层模型与能力以 Xinghai 路由配置为准。'
  when 'grok-build-0.1' then 'xAI 官方模型目录未列出这个精确 ID；这是一个固定名称的平台注册路由，不能据名称推断具体模型版本或能力。'
  when 'grok-build-latest' then 'xAI 官方模型目录未列出这个精确 ID；这是一个滚动编程路由别名，实际目标以当前通道配置为准。'
  when 'kimi-k2.5' then 'Moonshot 官方模型列表已将 Kimi K2.5 标为下线；它的历史定位是原生多模态模型，面向视觉理解、编码和 Agent Swarm，仅保留给旧兼容调用。'
  when 'kimi-k2.6' then 'Kimi K2.6 是通用思考模型，支持文本、图片和视频输入，可在思考与非思考模式间切换；官方标注 256K 上下文，适合复杂推理、长程编码和多步工具调用。'
  when 'kimi-k2.7-code' then 'Kimi K2.7 Code 面向 Coding，支持文本、图片和视频输入以及 256K 上下文，始终启用思考，适合长程软件工程和多步编程 Agent。'
  when 'kimi-k3' then 'Kimi K3 是旗舰思考模型，官方标注原生视觉理解和 1M token 上下文，面向长程编程、知识工作、深度推理和视觉协作。'
  when 'kimi-k3-256k' then 'Kimi 官方模型列表目前列出的是 1M 上下文的 kimi-k3，未找到 kimi-k3-256k 这一精确 ID；该记录应视为平台别名，不能据名称推断其上下文或能力。'
  when 'mistral-large-3' then 'Mistral Large 3 是 Mistral 的开放权重通用多模态模型，采用 MoE 架构，官方资料列出 41B 激活参数、675B 总参数和 256K 上下文，并支持函数调用、结构化输出、文档问答与 Agent 工具。'
  when 'muse-spark-1.2-contributor' then 'Muse Spark 1.2 Contributor 面向创作和协作型任务，适合头脑风暴、文本润色、内容起草以及把知识整理成可交付材料。'
  when 'nemotron-3-super' then 'Nemotron 3 Super 是 NVIDIA-Nemotron-3-Super-120B-A12B 的平台简称，官方模型卡对应 120B 总参数、12B 激活参数的 BF16 开放权重 checkpoint，适合本地或自托管文本生成。'
  when 'nemotron-3-ultra' then 'Nemotron 3 Ultra 是 NVIDIA-Nemotron-3-Ultra-550B-A55B 的平台简称，官方模型卡对应 550B 总参数、55B 激活参数的 BF16 开放权重 checkpoint，面向高端自托管文本生成。'
  when 'qwen-vl-ocr' then '阿里云将 Qwen-VL-OCR 定位为基于 Qwen-VL 训练的 OCR 模型，统一处理图像文字识别、解析与加工；输入为文本和图像，输出文本，适合文档 OCR、版面解析和关键信息提取。'
  when 'qwen3.5-omni-flash' then 'Qwen3.5-Omni-Flash 支持文本、图像、视频和音频输入，输出文本与音频；官方强调长音频和音视频理解，适合语音助手、文本创作和多媒体分析。'
  when 'qwen3.5-omni-plus' then 'Qwen3.5-Omni-Plus 面向全模态理解与交互，支持文本、图像、视频、音频输入以及文本和音频输出，并提供结构化 JSON 输出。'
  when 'qwen3.5-plus' then 'Qwen3.5-Plus 是原生视觉语言模型，支持文本、图像、视频输入并输出文本，提供 1M 上下文；官方覆盖长文档、代码与 Agent、图像视频理解和工具调用。'
  when 'qwen3.6-flash' then 'Qwen3.6-Flash 是原生视觉语言 Flash 模型，支持图像、文本、视频输入并输出文本，提供 1M 上下文；官方突出 Agentic Coding、数学与代码推理以及空间目标定位。'
  when 'qwen3.6-max-preview' then '未找到阿里云官方可核实的 qwen3.6-max-preview 精确条目；该名称应标为平台别名或官方资料无法核实，不要从 Qwen3.6 Plus 或 Max 推断能力。'
  when 'qwen3.6-plus' then 'Qwen3.6-Plus 是原生视觉语言模型，支持图像、文本、视频输入并输出文本，提供 1M 上下文；官方覆盖多模态编码、长上下文分析、OCR 和目标定位。'
  when 'qwen3.7-max' then 'Qwen3.7-Max 是 3.7 系列高阶模型，官方当前公开为文本输入输出和 1M 上下文，定位是编程、办公生产力和长期自主执行。'
  when 'qwen3.7-plus' then 'Qwen3.7-Plus 支持图像、文本、视频输入并输出文本，提供 1M 上下文；官方突出交互式混合 Agent、GUI 和移动端自动化。'
  when 'qwen3.8-flash' then 'Qwen3.8-Flash 是原生多模态模型，支持图像、文本、视频输入和文本输出，提供 1M 上下文；官方覆盖编码辅助、Agent、桌面操作、图表和长视频分析。'
  when 'qwen3.8-max' then 'Qwen3.8-Max 是 MoE 旗舰，支持图像、文本、视频输入、文本输出和 1M 上下文；官方覆盖编码、办公、法律、金融、设计以及长程自主规划。'
  when 'step-3.5-flash' then '阶跃星辰旗舰语言推理模型，纯文本输入，256K 上下文，采用稀疏 MoE；官方强调高速推理、工具调用与多步任务执行，适合 Agent、代码、数学、逻辑推理和深度研究。'
  when 'step-3.5-flash-2603' then 'StepFun 将 3.5 Flash 2603 定义为 Agent 优化版，重点提升 Token 效率和推理速度，并针对 Coding 与 Agent 框架做兼容优化；其他模态和参数以具体上游配置为准。'
  when 'step-3.7-flash' then '阶跃星辰旗舰多模态推理模型，原生支持图片和视频理解，提供 256K 上下文、low/medium/high 三档推理强度和工具调用，适合实时 Agent、代码、规划及视觉工作流。'
  when 'step-5-preview' then 'StepFun 面向真实任务的新一代旗舰基模，支持文本、图片、视频输入、文本输出和 1M 上下文，最大输出 64K；适合长材料分析、软件工程、研究型 Agent 和视频理解。'
  when 'step-image-edit-2' then '阶跃星辰图像生成与编辑模型，同时支持文生图和图像编辑，官方定位为 1–2 秒级实时交互修图；官方公告显示该型号计划于 2026-10-10 下线。'
  when 'step-router-v1' then 'Step Router V1 是 Step Plan 的智能路由入口，不是固定权重模型；它会在 deepseek-v4-pro 与 step-3.7-flash 等引擎之间按请求特征调度，模态、上下文和费用随命中引擎变化。'
  when 'stepaudio-2.5-asr' then 'StepAudio 2.5 ASR 是 4B 参数语音识别模型，接收音频并输出转写文本，提供一次性提交、SSE、文件识别和实时 WebSocket，适合实时字幕、会议记录和 Voice Agent。'
  when 'stepaudio-2.5-chat' then 'StepAudio 2.5 Chat 是端到端语音对话模型，支持音频或文本输入，但官方明确只返回文本；它能理解语气、迟疑和轻笑等副语言线索，适合语音问答和文本型 Voice Agent。'
  when 'stepaudio-2.5-realtime' then 'StepAudio 2.5 Realtime 通过 WebSocket 双向处理语音，支持 Server VAD、流式响应、人设和音色复刻，面向带情绪表现的实时语音助手与陪伴场景。'
  when 'stepaudio-2.5-tts' then 'StepAudio 2.5 TTS 接收文本、Global Context 和 Inline Context，生成带停顿、重音与情绪表现的语音，并支持短参考音频的音色复刻，适合有声内容和情感播报。'
  else description
end,
updated_at = now()
where model in (
  'MiniMax-M2', 'MiniMax-M2.1', 'MiniMax-M2.1-highspeed', 'MiniMax-M2.5', 'MiniMax-M2.5-highspeed', 'MiniMax-M2.7', 'MiniMax-M2.7-highspeed', 'MiniMax-M3',
  'codex-auto-review', 'deepseek-v4-flash', 'deepseek-v4-pro', 'deepseek-v4.1-flash', 'doubao-seed-2.0-lite', 'doubao-seed-2.1-turbo', 'doubao-seed-evolving',
  'glm-4-32b', 'glm-4.5', 'glm-4.5-air', 'glm-4.5v', 'glm-4.6', 'glm-4.6v', 'glm-4.7', 'glm-4.7-flash', 'glm-5', 'glm-5-turbo', 'glm-5.1', 'glm-5.2', 'glm-5.2-fast-preview', 'glm-5.3', 'glm-5.3-flash', 'glm-5v-turbo',
  'gpt-4o-audio-preview', 'gpt-4o-realtime-preview', 'gpt-5.2', 'gpt-5.2-2025-12-11', 'gpt-5.2-chat-latest', 'gpt-5.2-pro', 'gpt-5.2-pro-2025-12-11', 'gpt-5.3-codex-spark', 'gpt-5.4', 'gpt-5.4-2026-03-05', 'gpt-5.4-mini', 'gpt-5.4-mini-openai-compact', 'gpt-5.4-openai-compact', 'gpt-5.5', 'gpt-5.5-openai-compact', 'gpt-5.6', 'gpt-5.6-luna', 'gpt-5.6-sol', 'gpt-5.6-terra', 'gpt-image-1', 'gpt-image-1.5', 'gpt-image-2',
  'grok', 'grok-4', 'grok-4-0709', 'grok-4.20-0309-non-reasoning', 'grok-4.20-0309-reasoning', 'grok-4.20-multi-agent-0309', 'grok-4.20-non-reasoning', 'grok-4.20-reasoning', 'grok-4.3', 'grok-4.5', 'grok-4.5-latest', 'grok-4.6', 'grok-build', 'grok-build-0.1', 'grok-build-latest',
  'kimi-k2.5', 'kimi-k2.6', 'kimi-k2.7-code', 'kimi-k3', 'kimi-k3-256k', 'mistral-large-3', 'muse-spark-1.2-contributor', 'nemotron-3-super', 'nemotron-3-ultra',
  'qwen-vl-ocr', 'qwen3.5-omni-flash', 'qwen3.5-omni-plus', 'qwen3.5-plus', 'qwen3.6-flash', 'qwen3.6-max-preview', 'qwen3.6-plus', 'qwen3.7-max', 'qwen3.7-plus', 'qwen3.8-flash', 'qwen3.8-max',
  'step-3.5-flash', 'step-3.5-flash-2603', 'step-3.7-flash', 'step-image-edit-2', 'step-router-v1', 'stepaudio-2.5-asr', 'stepaudio-2.5-chat', 'stepaudio-2.5-realtime', 'stepaudio-2.5-tts'
);

with available as (
  select distinct trim(item.model) as model
  from channels c
  cross join lateral jsonb_array_elements_text(c.models) as item(model)
  where c.enabled and not c.auto_disabled and trim(item.model) <> ''
  union
  select distinct trim(m.public_model) as model
  from model_routes m
  join channels c on c.id = m.channel_id
  where m.enabled and not m.hidden and c.enabled and not c.auto_disabled and trim(m.public_model) <> ''
), curated(model, description) as (
  values
    ('deepseek-v4-flash-vision', 'DeepSeek 的旧视觉兼容别名；官方当前将相关 V4 Flash 旧入口路由到 V4.1-Flash，具体是否支持视觉以当前上游接口为准。'),
    ('deepseek-v4-pro-0813', 'DeepSeek-V4-Pro-0813 的版本标识，面向高强度推理、Agent Coding 与复杂任务；具体 API 能力以当前 DeepSeek 通道返回的模型信息为准。'),
    ('gpt-6', '官方 OpenAI 模型目录未核实这个精确 ID；当前记录应视为平台别名，不能据名称推断版本、模态或上下文。'),
    ('gpt-6-astra', 'Xinghai 的 Astra 路由别名，不是 OpenAI 官方独立型号；实际底层模型由当前路由配置决定。'),
    ('gpt-6-luna', 'Xinghai 的 Luna 路由别名，不是 OpenAI 官方独立型号；实际底层模型由当前路由配置决定。'),
    ('gpt-6-sol', 'Xinghai 的 Sol 路由别名，不是 OpenAI 官方独立型号；实际底层模型由当前路由配置决定。'),
    ('gpt-6.1-sol', 'Xinghai 的版本化路由别名，未在 OpenAI 官方目录核实精确型号；请以当前通道和路由配置为准。'),
    ('gpt-image-2.5', '官方 OpenAI 模型目录未核实这个精确图像模型 ID；当前记录应视为平台别名，不能据名称推断图像能力或版本状态。'),
    ('gpt-image-2.5-flare', 'Xinghai 的图像路由别名，未在 OpenAI 官方目录核实精确型号；实际生成模型和能力以当前路由配置为准。'),
    ('gpt-image-2.5-sunburst', 'Xinghai 的图像路由别名，未在 OpenAI 官方目录核实精确型号；实际生成模型和能力以当前路由配置为准。'),
    ('kimi-k2.8-preview', 'Kimi 官方模型列表未找到这个精确 ID；当前记录应标为平台别名或无法核实，不应从版本号推测模态、上下文或能力。'),
    ('laguna-s-2.1', '未找到与该精确 ID 对应的厂商官方模型资料；当前记录应标为平台别名或无法核实，能力以实际路由配置为准。'),
    ('step-5-preview', 'StepFun 面向真实任务的新一代旗舰基模，支持文本、图片、视频输入、文本输出和 1M 上下文，最大输出 64K；适合长材料分析、软件工程、研究型 Agent 和视频理解。')
)
insert into model_catalog_metadata(model, description)
select curated.model, curated.description
from curated
join available on available.model = curated.model
on conflict (model) do update
set description = excluded.description, updated_at = now();

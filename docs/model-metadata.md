# 模型描述与官方来源

本轮核验日期：2026-10-02。以下逐项记录本次整理的中文描述、官方出处与核验范围。模型描述介绍厂商模型的定位；厂商接口中的内置工具、实时音频等功能，不等同于本平台已经提供相应端点。价格、路由配置和能力字段仍由现有配置决定。

“已核实”表示已取得对应官方模型或版本的说明；“系列已核实”表示官方系列存在，但此调用名可能省略日期或厂商前缀；“待核实”表示未确认精确名称的规格或映射，不代表模型不存在，也不据此断言它是平台自建别名。网络访问失败不构成不存在的证据。

描述应写出足以区分模型的特点，如视觉反馈编程、只返回文本的语音理解、实时音频、固定快照或官方加速服务。不要靠替换模型名重复一套用途列表，不把固定版本写成确定性输出，不将检索和工程备注写入用户文案。

迁移 106 已经应用，保留其原内容；107 负责纠正文案和补齐记录。107 保存本轮从 channel/route 与已有元数据确认的 127 个名称，不按执行瞬间的通道状态或上游模型列表过滤，避免临时下线再恢复后丢失描述。它只补空描述或替换 106 的原文，并保留之后人工修改的描述以及名称、能力、上下文等字段。第二次运行没有数据变化。

| 模型 | 核验范围 | 描述 | 官方资料或核验入口 |
| --- | --- | --- | --- |
| `MiniMax-M2` | 已核实 | MiniMax 面向工具使用的文本模型，可在推理过程中调用函数并持续处理多步任务。200K 上下文用于容纳较长的对话和工具结果，单次输出上限包含思考内容。 | [来源1](https://platform.minimax.io/docs/guides/models-intro) |
| `MiniMax-M2.1` | 已核实 | 重点改进多语言编程与代码重构，能够在理解既有实现后修改代码，而不局限于生成独立片段。采用 230B 总参数、10B 激活参数的稀疏架构。 | [来源1](https://platform.minimax.io/docs/guides/models-intro) |
| `MiniMax-M2.1-highspeed` | 已核实 | M2.1 的官方加速版本，保留相同模型效果与多语言编程、精细重构能力，缩短推理等待时间。需要频繁来回修改代码时，可选用这一版本。 | [来源1](https://platform.minimax.io/docs/guides/models-intro) |
| `MiniMax-M2.5` | 已核实 | 围绕软件开发中的代码生成、理解与重构优化，兼顾复杂任务完成质量和调用成本。主要用途是把开发需求落实到代码修改、修复和工程实现。 | [来源1](https://platform.minimax.io/docs/guides/models-intro) |
| `MiniMax-M2.5-highspeed` | 已核实 | 与 M2.5 保持相同模型效果，官方单独提供更快的推理服务。多语言代码和精准重构能力不变，用于需要较快反馈的开发助手。 | [来源1](https://platform.minimax.io/docs/guides/models-intro) |
| `MiniMax-M2.7` | 已核实 | 将递归自我改进用于真实工程任务，扩展到专业办公成果交付与具有角色特点的交互。它的重点从代码片段进一步延伸到持续推进工程和办公任务。 | [来源1](https://platform.minimax.io/docs/guides/models-intro) |
| `MiniMax-M2.7-highspeed` | 已核实 | M2.7 的官方高速版本，在保持相同效果的前提下加快推理。保留工程与办公任务能力，用于反复调用工具、修改代码和检查结果的工作过程。 | [来源1](https://platform.minimax.io/docs/guides/models-intro) |
| `MiniMax-M3` | 已核实 | 原生多模态编程模型，可结合图片、视频和桌面操作完成开发任务。1M 上下文用于承载大型工程资料，重点覆盖缺陷修复、前后端开发、性能优化及办公自动化。 | [来源1](https://platform.minimax.io/docs/guides/models-intro) / [来源2](https://www.minimax.io/blog/minimax-m3) |
| `codex-auto-review` | 待核实 | 用于自动代码评审的调用入口，检查变更中的缺陷、风险及可改进之处。底层模型的具体版本尚未在公开模型资料中确认。 | 暂无对应的公开模型资料 |
| `deepseek-flash` | 已核实 | DeepSeek-V4.1-Flash 的官方 API 入口，原生理解图片，支持思考、工具调用及长上下文。新架构分别优化输入处理与输出生成，并压缩缓存开销，面向需要频繁读取材料和调用工具的任务。 | [来源1](https://api-docs.deepseek.com/news/news260910/) / [来源2](https://api-docs.deepseek.com/quick_start/pricing/) |
| `deepseek-v4-flash` | 已核实 | V4 系列中较小、较经济的 Flash 模型，侧重推理与工具执行效率。DeepSeek 官方已将这一旧调用名兼容转向 V4.1-Flash，原版和兼容入口的版本需要区分。 | [来源1](https://api-docs.deepseek.com/news/news260910/) / [来源2](https://api-docs.deepseek.com/quick_start/pricing/) |
| `deepseek-v4-flash-vision` | 待核实 | 这一调用名的精确版本尚未确认。DeepSeek 公开的视觉实验版名称为 deepseek-v4-flash-vision-exp，官方已将该实验版入口兼容转向 V4.1-Flash。 | [来源1](https://api-docs.deepseek.com/news/news260910/) / [来源2](https://api-docs.deepseek.com/quick_start/pricing/) |
| `deepseek-v4-pro` | 已核实 | V4 系列面向高强度文本推理的模型，重点覆盖数学、科学问题和编程智能体，并提供百万级上下文。原版不支持图片输入；官方服务的版本映射可能随升级调整。 | [来源1](https://api-docs.deepseek.com/news/news260910/) / [来源2](https://api-docs.deepseek.com/quick_start/pricing/) |
| `deepseek-v4-pro-0813` | 系列已核实 | DeepSeek-V4-Pro 的 0813 版本，重点改进正式版的推理、代码与工具使用能力。该版本名称与官方发布相符，具体服务映射仍取决于接入方。 | [来源1](https://api-docs.deepseek.com/news/news260813/) / [来源2](https://api-docs.deepseek.com/quick_start/pricing/) |
| `deepseek-v4.1-flash` | 系列已核实 | DeepSeek 新架构下的原生视觉模型，能结合图片与文本进行推理。重点改进推理吞吐与长对话缓存效率；官方 API 使用 deepseek-flash 作为调用名。 | [来源1](https://api-docs.deepseek.com/news/news260910/) / [来源2](https://api-docs.deepseek.com/quick_start/pricing/) |
| `doubao-seed-2.0-lite` | 系列已核实 | Seed 2.0 的轻量全模态理解模型，可统一分析文字、图像、视频和音频。可以用来从混合媒体材料中提取信息、理解内容，并以文本组织结果。 | [来源1](https://www.volcengine.com/docs/82379/1553586) |
| `doubao-seed-2.1-turbo` | 系列已核实 | 把深度思考、多模态理解与图形界面操作结合起来，可通过工具调用拆解并执行任务。支持 256K 上下文与结构化输出，面向需要读取屏幕或长材料的智能体应用。 | [来源1](https://www.volcengine.com/docs/82379/1593703) |
| `doubao-seed-evolving` | 系列已核实 | Seed 系列持续演进的编程与智能体模型，重点是长程规划、代码生成及复杂工具编排。调用名不包含固定日期，使用时需要关注服务版本更新。 | [来源1](https://docs.volcengine.com/docs/ark/latest-model?lang=zh) |
| `glm-4-32b` | 待核实 | 官方 GLM-4-32B-0414 是 320 亿参数的开放文本对话模型，强化工程代码、函数调用、检索问答和报告生成，区别于 GLM-Z1 深度推理模型。目录短名对应的具体发布版本与服务规格尚待确认。 | [来源1](https://huggingface.co/zai-org/GLM-4-32B-0414/raw/main/README.md) / [来源2](https://github.com/zai-org/GLM-4) |
| `glm-4.5` | 已核实 | 将推理、编程和智能体工具使用合在同一文本模型中，支持思考与非思考两种模式。其工具调用、网页浏览和前端开发能力针对多步任务优化，提供 128K 上下文。 | [来源1](https://docs.bigmodel.cn/cn/guide/models/text/glm-4.5) |
| `glm-4.5-air` | 已核实 | GLM-4.5 家族的轻量成员，106B 总参数中每次激活 12B，保留推理、编码与工具调用能力。与标准版同为 128K 上下文，侧重计算效率。 | [来源1](https://docs.bigmodel.cn/cn/guide/models/text/glm-4.5) |
| `glm-4.5v` | 已核实 | 在图像和视频中进行视觉推理，能理解文档、图表与界面，并定位画面中的对象。可用于网页界面复刻、视频内容分析和需要读屏的智能体，输出为文本。 | [来源1](https://docs.bigmodel.cn/cn/guide/models/vlm/glm-4.5v) |
| `glm-4.6` | 已核实 | 将文本上下文扩展到 200K，并支持在推理过程中使用工具。相比只生成答案，它更侧重搜索资料、编写代码与处理办公材料之间的连续协作。 | [来源1](https://docs.bigmodel.cn/cn/guide/models/text/glm-4.6) |
| `glm-4.6v` | 已核实 | 把函数调用原生融入视觉理解，可根据图片、文档或视频内容决定下一步操作。重点覆盖图表问答、OCR 与版式重建、视频时间线分析以及图形界面任务。 | [来源1](https://docs.bigmodel.cn/cn/guide/models/vlm/glm-4.6v) |
| `glm-4.7` | 已核实 | 面向需要长程规划的编程智能体，强化工具协同、前端视觉代码与任务交付。提供 200K 上下文和 128K 最大输出，也覆盖深度研究与办公创作。 | [来源1](https://docs.bigmodel.cn/cn/guide/models/text/glm-4.7) |
| `glm-4.7-flash` | 已核实 | 30B 级别的高效编程模型，在较小规模下提供 200K 上下文与 128K 最大输出。可用于前后端开发和工具协作，也覆盖中文写作、翻译与角色对话。 | [来源1](https://docs.bigmodel.cn/cn/guide/models/free/glm-4.7-flash) |
| `glm-5` | 已核实 | 将目标从生成代码推进到完整工程任务，重点处理复杂系统设计与长程执行。可在 200K 文本上下文中结合工具开展编码和办公自动化。 | [来源1](https://docs.bigmodel.cn/cn/guide/models/text/glm-5) |
| `glm-5-turbo` | 已核实 | 针对 OpenClaw 的工具使用和任务执行专门优化，强化复杂指令拆解、定时任务与持续运行。适用于需要长链路调用工具的个人助手和自动化服务。 | [来源1](https://docs.bigmodel.cn/cn/guide/models/text/glm-5-turbo) |
| `glm-5.1` | 已核实 | 围绕自主工作数小时的长程任务训练，强调复杂代码修改后的持续验证与优化。主要用途是工程问题求解、交互页面制作以及办公成果交付。 | [来源1](https://docs.bigmodel.cn/cn/guide/models/text/glm-5.1) |
| `glm-5.2` | 已核实 | 用 1M 上下文承载项目级资料，重点改进跨文件重构、工程规范遵循和长程交付。移动应用、小程序开发等需要兼顾多文件约束的任务是官方强调的方向。 | [来源1](https://docs.bigmodel.cn/cn/guide/models/text/glm-5.2) |
| `glm-5.2-fast-preview` | 待核实 | 官方 GLM-5.2 面向长程编程和项目级工程任务，提供 1M 上下文与最高 128K 输出。此 fast-preview 版本的速度档位、上下文和输出规格尚待确认。 | [来源1](https://docs.z.ai/guides/llm/glm-5.2.md) / [来源2](https://docs.z.ai/release-notes/new-released.md) |
| `glm-5.3` | 已核实 | 基于 GLM-5.2 的同一基础模型，通过后训练加强复杂软件工程、终端操作与长程任务。提供 1M 文本上下文，始终开启思考，可选择 low、high、max 三档推理强度。 | [来源1](https://docs.bigmodel.cn/cn/guide/models/text/glm-5.3) |
| `glm-5.3-flash` | 已核实 | 将视觉反馈融入编程过程，可观察页面渲染与交互结果，再修改代码并继续验证。支持图片、视频和文件理解，1M 上下文也用于办公文档与金融研究等专业任务。 | [来源1](https://docs.bigmodel.cn/cn/guide/models/vlm/glm-5.3-flash) |
| `glm-5.3-flashx` | 已核实 | GLM-5.3-Flash 的高速服务版本，保留原生多模态、视觉编程和 1M 上下文。官方优化了推理速度，用于需要更快往返的代码、浏览器与办公协作。 | [来源1](https://docs.bigmodel.cn/cn/guide/models/vlm/glm-5.3-flash) |
| `glm-5v-turbo` | 已核实 | 面向视觉编程的多模态模型，可根据界面截图生成前端、排查显示问题并探索图形界面。支持图片、视频与文件输入，让智能体根据观察结果规划和执行操作。 | [来源1](https://docs.bigmodel.cn/cn/guide/models/vlm/glm-5v-turbo) |
| `gpt-4o-audio-preview` | 已核实 | 通过 Chat Completions 接收文字或音频，也能返回文字和语音，将听懂音频与生成回答放在同一次调用中。它是 GPT-4o 的音频预览版本，官方现已标记弃用。 | [来源1](https://developers.openai.com/api/docs/models/gpt-4o-audio-preview) |
| `gpt-4o-realtime-preview` | 已核实 | 为连续语音会话设计，官方通过 WebRTC 或 WebSocket 流式处理文字和音频，并支持函数调用。相较提交整段音频，它侧重边说边响应；该预览版本已被官方标记弃用。 | [来源1](https://developers.openai.com/api/docs/models/gpt-4o-realtime-preview) |
| `gpt-5.2` | 已核实 | 面向复杂专业工作的推理模型，能够结合文字与图像分析材料，并通过可调推理强度分配思考时间。主要覆盖编码、信息分析和专业成果制作。 | [来源1](https://developers.openai.com/api/docs/models/gpt-5.2) |
| `gpt-5.2-2025-12-11` | 已核实 | GPT-5.2 的 2025-12-11 快照，保留文本与图像理解、可调推理能力。明确指定这一版本可减少模型升级对应用评测和行为的影响，但不保证每次生成内容相同。 | [来源1](https://developers.openai.com/api/docs/models/gpt-5.2) |
| `gpt-5.2-chat-latest` | 已核实 | 指向 ChatGPT 所用 GPT-5.2 对话快照的调用名，可理解文字与图片。它与通用 GPT-5.2 API 模型的定位不同，官方已将这一对话入口标记弃用。 | [来源1](https://developers.openai.com/api/docs/models/gpt-5.2-chat-latest) |
| `gpt-5.2-pro` | 已核实 | 为困难问题投入更多推理计算，官方提供 medium、high、xhigh 三档思考强度。通过 Responses API 处理文字与图像，较复杂的请求可能需要数分钟。 | [来源1](https://developers.openai.com/api/docs/models/gpt-5.2-pro) |
| `gpt-5.2-pro-2025-12-11` | 已核实 | GPT-5.2 Pro 的 2025-12-11 固定快照，保留高计算量推理与文字、图像输入。用于需要锁定版本的复杂分析，官方接口为 Responses API。 | [来源1](https://developers.openai.com/api/docs/models/gpt-5.2-pro) |
| `gpt-5.3-codex-spark` | 已核实 | GPT-5.3-Codex-Spark 是为实时编程设计的轻量模型，适合快速修改代码、调整逻辑和迭代界面。官方发布时提供 128K 上下文且仅支持文本，重点是低延迟的编程交互。 | [来源1](https://openai.com/index/introducing-gpt-5-3-codex-spark/) |
| `gpt-5.4` | 已核实 | 将编程、推理与计算机操作结合到专业工作中，可读取文字和图像，并在配置工具后跨文件、网页和桌面完成任务。它适用于需要连续操作与结果检查的工程和知识工作。 | [来源1](https://developers.openai.com/api/docs/models/gpt-5.4) |
| `gpt-5.4-2026-03-05` | 已核实 | GPT-5.4 的 2026-03-05 快照，保留专业工作、图像理解和工具使用能力。版本固定便于测试应用升级影响，适合对模型版本有明确要求的部署。 | [来源1](https://developers.openai.com/api/docs/models/gpt-5.4) |
| `gpt-5.4-mini` | 已核实 | 针对编程、计算机操作和子智能体任务设计的小型模型，兼顾图像理解与高吞吐。可承担代码库中的局部修改，或作为更大任务里并行执行的助手。 | [来源1](https://developers.openai.com/api/docs/models/gpt-5.4-mini) |
| `gpt-5.4-mini-openai-compact` | 待核实 | 带有 openai-compact 后缀的 GPT-5.4 Mini 调用入口。公开资料尚未确认这一后缀对应的具体版本或服务参数。 | [来源1](https://developers.openai.com/api/docs/models/gpt-5.4-mini) |
| `gpt-5.4-openai-compact` | 待核实 | 带有 openai-compact 后缀的 GPT-5.4 调用名称。此后缀对应的服务配置和版本映射尚未确认。 | [来源1](https://developers.openai.com/api/docs/models/gpt-5.4) |
| `gpt-5.5` | 已核实 | 面向更复杂的专业工作，重点是编程和需要连续使用工具的任务。可综合文字与图像材料，在接入相应工具后完成搜索、计算机操作及专业成果制作。 | [来源1](https://developers.openai.com/api/docs/models/gpt-5.5) |
| `gpt-5.5-openai-compact` | 待核实 | 带有 openai-compact 后缀的 GPT-5.5 调用入口。公开资料尚未确认其独立规格或与官方快照的对应关系。 | [来源1](https://developers.openai.com/api/docs/models/gpt-5.5) |
| `gpt-5.6` | 已核实 | GPT-5.6 是 OpenAI 官方指向 GPT-5.6 Sol 的模型别名，面向复杂的专业工作。它支持文字与图片输入，提供最高 105 万 token 的上下文窗口，并可调用工具处理任务。 | [来源1](https://developers.openai.com/api/docs/models/gpt-5.6) / [来源2](https://developers.openai.com/api/docs/models/gpt-5.6-sol) |
| `gpt-5.6-luna` | 已核实 | GPT-5.6 Luna 面向成本敏感、调用量大的任务，定位大致对应早期 GPT-5 系列的 nano 档。它支持文字与图片输入，并提供最高 105 万 token 的上下文窗口。 | [来源1](https://developers.openai.com/api/docs/models/gpt-5.6-luna) |
| `gpt-5.6-sol` | 已核实 | GPT-5.6 Sol 是 GPT-5.6 系列的旗舰模型，面向复杂的专业工作，定位大致对应早期 GPT-5 系列不带后缀的主力档。它支持文字与图片输入，以及函数调用和结构化输出，可在较长的上下文中处理任务。 | [来源1](https://developers.openai.com/api/docs/models/gpt-5.6-sol) |
| `gpt-5.6-terra` | 已核实 | GPT-5.6 Terra 在模型能力与成本之间取平衡，定位大致对应早期 GPT-5 系列的 mini 档。它支持文字与图片输入，以及函数调用和结构化输出，适合需要控制调用成本的应用。 | [来源1](https://developers.openai.com/api/docs/models/gpt-5.6-terra) |
| `gpt-6` | 待核实 | gpt-6 这一完整模型标识的公开规格尚未核实。目前尚无法确认其对应的具体型号、上下文长度和输入输出能力。 | [来源1](https://developers.openai.com/api/docs/models/gpt-6) / [来源2](https://developers.openai.com/api/docs/models/gpt-6.md) / [来源3](https://developers.openai.com/api/docs/guides/latest-model?model=gpt-6-astra) |
| `gpt-6-astra` | 已核实 | GPT-6 Astra 面向高难度推理、编程、研究和文档创作，是官方定位中处理最复杂任务的模型。它能够跨代码、浏览器和专业软件执行多步骤任务，适合需要持续使用工具的工作。 | [来源1](https://developers.openai.com/api/docs/models/gpt-6-astra) / [来源2](https://developers.openai.com/api/docs/guides/latest-model?model=gpt-6-astra) |
| `gpt-6-luna` | 已核实 | GPT-6 Luna 面向目标明确、调用量大的任务，着重降低处理成本并提高响应速度。它支持文字与图片输入，也能通过 Responses API 使用内置工具和函数调用。 | [来源1](https://developers.openai.com/api/docs/models/gpt-6-luna) / [来源2](https://developers.openai.com/api/docs/guides/latest-model?model=gpt-6-astra) |
| `gpt-6-sol` | 已核实 | GPT-6 Sol 面向复杂编程和需要连续调用工具的智能体任务。它支持文字与图片输入，并提供最高 105 万 token 的上下文窗口，便于在较长任务中保留相关材料。 | [来源1](https://developers.openai.com/api/docs/models/gpt-6-sol) |
| `gpt-6.1-sol` | 已核实 | GPT-6.1 Sol 面向复杂编程、计算机操作和专业工作，官方定位是在更低成本下提供接近 Astra 的表现。它支持文字与图片输入，并可通过 Responses API 调用工具完成任务。 | [来源1](https://developers.openai.com/api/docs/models/gpt-6.1-sol.md) / [来源2](https://developers.openai.com/api/docs/guides/latest-model?model=gpt-6-astra) |
| `gpt-image-1` | 已核实 | OpenAI 的原生多模态图像模型，可依据文字提示生成画面，或结合参考图进行编辑。官方已将这一代模型标记弃用，描述保留其历史生成与编辑定位。 | [来源1](https://developers.openai.com/api/docs/models/gpt-image-1) |
| `gpt-image-1.5` | 已核实 | 在 GPT Image 系列中重点改进提示词遵循与编辑准确性，使文字要求和参考图片更准确地落实到画面。该版本已被官方标记弃用。 | [来源1](https://developers.openai.com/api/docs/models/gpt-image-1.5) |
| `gpt-image-2` | 已核实 | 同时处理图像生成与编辑，重点提升生成速度、画面质量和参考图保真度。支持更灵活的画面尺寸，用于需要按明确视觉要求制作或修改图像的工作。 | [来源1](https://developers.openai.com/api/docs/models/gpt-image-2) |
| `gpt-image-2.5` | 待核实 | gpt-image-2.5 这一完整模型标识的公开规格尚未核实。目前尚无法确认其对应的具体图像模型及生成、编辑能力。 | [来源1](https://developers.openai.com/api/docs/models/gpt-image-2.5) / [来源2](https://developers.openai.com/api/docs/models/gpt-image-2.5.md) / [来源3](https://developers.openai.com/api/docs/guides/image-generation.md) |
| `gpt-image-2.5-flare` | 已核实 | GPT Image 2.5 Flare 主打快速生成高质量的日常图片，适合需要频繁出图和快速迭代的场景。它接受文字与图片输入、输出图片，并提供从 low 到 max 以及 auto 的画质选项。 | [来源1](https://developers.openai.com/api/docs/models/gpt-image-2.5-flare) |
| `gpt-image-2.5-sunburst` | 已核实 | GPT Image 2.5 Sunburst 可根据文字与图片生成或编辑图像，尤其适合重视修改精度的任务。它输出图片，并提供从 low 到 max 以及 auto 的画质选项。 | [来源1](https://developers.openai.com/api/docs/models/gpt-image-2.5-sunburst) |
| `gpt-reserve` | 待核实 | 这一调用名称尚无可确认的公开模型规格。底层型号、输入形式和上下文上限仍需核实。 | 暂无对应的公开模型资料 |
| `grok` | 待核实 | 未带版本号的 Grok 调用名称，目前尚未核实它指向哪一代模型。具体推理方式和上下文规格需按接入服务确认。 | [来源1](https://docs.x.ai/developers/models) |
| `grok-4` | 已核实 | Grok 4 强调高级推理和多模态理解，官方发布时提供 256K 上下文。它能结合图片与文本处理复杂问题，输出文本答案。 | [来源1](https://x.ai/news/grok-4) |
| `grok-4-0709` | 已核实 | Grok 4 的 0709 版本标识，沿用该系列的多模态理解和高级推理定位。日期名称用于区分版本，不代表每次生成的答案完全一致。 | [来源1](https://x.ai/news/grok-4) |
| `grok-4.20-0309-non-reasoning` | 已核实 | Grok 4.20 的 0309 非推理版本，接收文字与图片、输出文本，提供 1M 上下文。重点是快速响应与智能体工具调用，同时支持结构化输出。 | [来源1](https://docs.x.ai/developers/models/grok-4.20-beta-0309-non-reasoning) |
| `grok-4.20-0309-reasoning` | 已核实 | Grok 4.20 的 0309 推理版本，在作答前分配计算进行思考。支持文字与图片输入、1M 上下文和工具调用，用于需要推理与执行结合的任务。 | [来源1](https://docs.x.ai/developers/models/grok-4.20-reasoning) |
| `grok-4.20-multi-agent-0309` | 已核实 | Grok 4.20 的多智能体版本，以多个智能体协作处理任务为定位。0309 标识将其与该系列的单模型推理及非推理版本区分开来。 | [来源1](https://docs.x.ai/developers/models/grok-4.20-multi-agent-0309) |
| `grok-4.20-non-reasoning` | 已核实 | Grok 4.20 非推理版的官方别名，当前指向 0309 版本。提供文字、图片理解与工具调用，面向更快的响应和较长材料处理。 | [来源1](https://docs.x.ai/developers/models/grok-4.20-beta-0309-non-reasoning) |
| `grok-4.20-reasoning` | 已核实 | Grok 4.20 推理版的官方别名，当前对应 0309 版本。将思考能力与多模态输入、工具使用结合，别名目标可能随服务更新变化。 | [来源1](https://docs.x.ai/developers/models/grok-4.20-reasoning) |
| `grok-4.3` | 已核实 | xAI 提供百万级上下文的 Grok 模型，可在一次请求中纳入较长的对话与文字材料。其调用名称为 grok-4.3。 | [来源1](https://docs.x.ai/developers/models/grok-4.3) |
| `grok-4.5` | 已核实 | 围绕编码、智能体执行和知识工作优化，重点是让模型在使用工具的过程中推进任务。适合需要把分析转为实际操作的开发与资料处理流程。 | [来源1](https://docs.x.ai/developers/models/grok-4.5) |
| `grok-4.5-latest` | 已核实 | Grok 4.5 的 latest 调用名，沿用编程、智能体执行和知识工作的定位。它没有日期锁定，接入时需要关注服务版本更新。 | [来源1](https://docs.x.ai/developers/models/grok-4.5) |
| `grok-4.6` | 已核实 | 面向编程、智能体任务和知识工作的 Grok 模型，提供 500K 上下文，支持文本与图像输入并输出文本。可在 low、medium、high、xhigh 四档推理强度间选择，并支持函数调用与结构化输出。 | [来源1](https://docs.x.ai/developers/models/grok-4.6) / [来源2](https://docs.x.ai/developers/release-notes.md) |
| `grok-build` | 待核实 | 此名称对应的具体模型版本、上下文长度和推理规格尚待确认。 | [来源1](https://docs.x.ai/developers/models/grok-build-0.1) / [来源2](https://docs.x.ai/build/overview.md) / [来源3](https://docs.x.ai/docs/models) |
| `grok-build-0.1` | 已核实 | 面向智能体软件开发、工程任务和自动化工作流的 Grok 编程模型，支持文本与图像输入，提供 256K 上下文。具备推理、函数调用和结构化输出能力，适合在开发工具中反复读取代码、调用工具并完成修改。 | [来源1](https://docs.x.ai/developers/models/grok-build-0.1) / [来源2](https://docs.x.ai/developers/release-notes.md) |
| `grok-build-latest` | 待核实 | 此 latest 名称当前指向的模型版本、更新规则和服务规格尚待确认。 | [来源1](https://docs.x.ai/developers/models/grok-build-0.1) / [来源2](https://docs.x.ai/docs/models) |
| `hy3` | 已核实 | 腾讯混元 Hy3 正式版采用 295B 总参数、21B 激活参数的 MoE 架构，支持 256K 上下文，面向复杂推理、编程和工具协作。相较 Hy3-preview，官方强调更可靠的指令遵循、工具调用与真实智能体任务表现，并采用 Apache 2.0 许可开放。 | [来源1](https://github.com/Tencent-Hunyuan/Hy3) |
| `hy3-preview` | 已核实 | 腾讯混元 Hy3 的预览版本，采用 295B 总参数、每 token 激活 21B 的 MoE 架构，提供 256K 上下文。侧重推理、编程和多步智能体任务，以较小激活规模兼顾能力与推理成本。 | [来源1](https://github.com/Tencent-Hunyuan/Hy3-preview) |
| `hy4-preview` | 已核实 | 腾讯混元 Hy4 的预览模型采用 770B 总参数、49B 激活参数的 MoE 架构，提供 1M 上下文。相较 Hy3 系列扩大了模型容量和可处理材料长度，面向更复杂的推理、编程及长程工具任务。 | [来源1](https://github.com/Tencent-Hunyuan/Hy4-preview) |
| `kimi-k2.5` | 已核实 | Kimi 的原生多模态模型，把视觉理解、编程与智能体协作结合起来，可依据视觉材料完成代码和分析任务。Kimi 官方已下线该型号，接入方保留的同名服务需另行确认版本。 | [来源1](https://www.kimi.com/blog/kimi-k2-5) / [来源2](https://platform.kimi.com/docs/models) |
| `kimi-k2.6` | 已核实 | 支持文字、图片与视频的思考模型，可切换是否开启思考。256K 上下文用于长程编程、复杂材料分析和连续工具使用，同时兼顾常规对话。 | [来源1](https://platform.kimi.com/docs/guide/kimi-k2-6-quickstart) |
| `kimi-k2.7-code` | 已核实 | 专门面向编程的思考模型，始终开启推理，能够结合图片和视频理解开发需求。256K 上下文覆盖较长的工程材料与工具结果，侧重长程代码修改和软件工程。 | [来源1](https://platform.kimi.com/docs/guide/kimi-k2-7-code-quickstart) |
| `kimi-k2.8-preview` | 待核实 | 本平台将此名称转发至 kimi-for-coding。该目标对应的官方发布版本、上下文长度和输入模态尚待确认。 | [来源1](https://platform.kimi.ai/docs/models.md) / [来源2](https://platform.kimi.ai/docs/overview) |
| `kimi-k3` | 已核实 | 具备原生视觉理解的旗舰思考模型，以 1M 上下文承载长程编程与知识工作。持续进行推理，能把图像材料、复杂分析和工具协作结合起来。 | [来源1](https://platform.kimi.com/docs/guide/kimi-k3-quickstart) |
| `kimi-k3-256k` | 待核实 | 本平台将此名称转发至 k3-256k。该目标对应的官方发布版本和实际上下文规格尚待确认，不能直接沿用官方 kimi-k3 的 1M 上下文说明。 | [来源1](https://platform.kimi.ai/docs/models.md) / [来源2](https://platform.kimi.ai/docs/overview) |
| `laguna-s-2.1` | 已核实 | Poolside 的开放权重编程模型，采用 118B 总参数、每 token 激活 8B 的 MoE 架构，提供 1M 上下文，适合持续修改大型代码库和执行多步工具任务。支持按请求开启或关闭思考，接收文本并输出文本。 | [来源1](https://docs.poolside.ai/get-started/supported-models.md) / [来源2](https://docs.poolside.ai/api/overview.md) |
| `longcat-2.0` | 已核实 | 美团 LongCat-2.0 是开放权重 MoE 模型，拥有 1.6T 总参数、约 48B 激活参数和 1M 上下文。其稀疏注意力与长上下文训练面向代码理解、仓库级修改和持续工具执行，适合需要跨多个文件完成的开发任务。 | [来源1](https://longcat.ai/blog/longcat-2.0) / [来源2](https://longcat.ai/) |
| `mimo-v2-omni` | 已核实 | 小米 MiMo-V2-Omni 将文本、图像、视频和音频理解整合到同一模型中，并支持工具调用与界面定位，适合从音视频材料或屏幕内容出发执行任务。官方已将该旧版 API 名称列入 2026 年 6 月 30 日下线清单，其后继版本为 MiMo-V2.5。 | [来源1](https://mimo.xiaomi.com/mimo-v2-omni) / [来源2](https://mimo.mi.com/docs/zh-CN/updates/deprecate) |
| `mimo-v2-pro` | 已核实 | 小米 MiMo-V2-Pro 是面向复杂软件工程与智能体工作流的文本模型，总参数超过 1T、激活参数为 42B，支持 1M 上下文。官方已将该旧版 API 名称列入 2026 年 6 月 30 日下线清单，其后继版本为 MiMo-V2.5-Pro。 | [来源1](https://mimo.xiaomi.com/mimo-v2-pro) / [来源2](https://mimo.mi.com/docs/zh-CN/updates/deprecate) |
| `mimo-v2.5` | 已核实 | MiMo-V2.5 原生理解文本、图像、视频和音频，提供 1M 上下文及最高 128K 文本输出，适合长视频分析和需要跨模态材料的智能体任务。官方计划于北京时间 2026 年 10 月 21 日 10:00 将此名称直接下线，未安排自动替换模型。 | [来源1](https://mimo.mi.com/models/en-US/mimo-v2.5) / [来源2](https://mimo.mi.com/docs/en-US/quick-start/summary/model) / [来源3](https://mimo.mi.com/docs/zh-CN/updates/deprecate) |
| `mimo-v2.5-pro` | 已核实 | MiMo-V2.5-Pro 面向大型代码库开发和长程智能体任务，提供 1M 上下文、最高 128K 输出，并强调连续数百次工具调用中的任务连贯性。官方模型详情页将其列为文本输入与输出，并计划于北京时间 2026 年 10 月 21 日 10:00 直接下线。 | [来源1](https://mimo.mi.com/models/en-US/mimo-v2.5-pro) / [来源2](https://mimo.mi.com/docs/en-US/quick-start/summary/model) / [来源3](https://mimo.mi.com/docs/zh-CN/updates/deprecate) |
| `mimo-v2.6-flash` | 已核实 | MiMo-V2.6-Flash 面向专业办公中的高频调用和规模化任务，在速度、成本和推理能力之间取均衡。支持文本、图像、视频和音频输入，提供 1M 上下文及最高 128K 文本输出，并具备工具调用、联网搜索和结构化输出能力。 | [来源1](https://mimo.mi.com/models/en-US/mimo-v2.6-flash) / [来源2](https://mimo.mi.com/docs/en-US/quick-start/summary/model) |
| `mimo-v2.6-pro` | 已核实 | MiMo-V2.6-Pro 面向复杂项目、长程软件开发、科研和高价值专业任务，强调多智能体协作及持续工具执行。支持文本、图像、视频和音频输入，提供 1M 上下文及最高 128K 文本输出，适合同时处理代码、文档与视觉材料。 | [来源1](https://mimo.mi.com/models/en-US/mimo-v2.6-pro) / [来源2](https://mimo.mi.com/docs/en-US/quick-start/summary/model) |
| `mistral-large-3` | 已核实 | Mistral 的开放权重多模态模型，采用 675B 总参数、41B 激活参数的 MoE 架构，提供 256K 上下文。支持结构化输出和函数调用，可把文档问答与工具使用接入应用。 | [来源1](https://docs.mistral.ai/models/mistral-large-3-25-12) |
| `muse-spark-1.2-contributor` | 待核实 | 官方 Muse Spark 1.2 是为编程与智能体工作流优化的模型。此 contributor 版本的能力范围、上下文与服务规格尚待确认。 | [来源1](https://developer.meta.com/ai/) / [来源2](https://ai.meta.com/blog/introducing-muse-spark-msl) |
| `nemotron-3-super` | 系列已核实 | NVIDIA 面向智能体、推理和对话任务的开放模型，采用 120B 总参数、12B 激活参数设计。其重点是以较少激活参数支撑文本生成和多步任务。 | [来源1](https://huggingface.co/nvidia/NVIDIA-Nemotron-3-Super-120B-A12B-BF16) / [来源2](https://docs.nvidia.com/nemo/megatron-bridge/nightly/models/nemotron/nemotron3-super.html) |
| `nemotron-3-ultra` | 系列已核实 | Nemotron 3 家族中的大规模开放语言模型，拥有 550B 总参数、55B 激活参数。官方提供可自行部署的模型权重，面向需要较大模型容量的文本生成与推理。 | [来源1](https://huggingface.co/nvidia/NVIDIA-Nemotron-3-Ultra-550B-A55B-BF16) |
| `qwen-vl-ocr` | 已核实 | 从图片中识别文字，并进一步解析版面和提取关键信息。基于 Qwen-VL 训练，处理文档、截图与表格后返回文本，重点是 OCR 而非开放式对话。 | [来源1](https://help.aliyun.com/en/model-studio/qwenvl-ocr) |
| `qwen3.5-omni-flash` | 已核实 | 同时理解文字、图片、视频与音频，可返回文本或语音。支持长音频和音视频材料分析，将听、看与说结合到语音助手和多媒体应用中。 | [来源1](https://help.aliyun.com/en/model-studio/qwen3-5-omni-flash) |
| `qwen3.5-omni-plus` | 已核实 | 全模态交互模型，可综合音频、视频、图片和文字作答，并生成语音。除自然语言回答外，也支持 JSON 结构化输出，便于从多媒体内容中整理信息。 | [来源1](https://help.aliyun.com/en/model-studio/qwen3-5-omni-plus) |
| `qwen3.5-plus` | 已核实 | 原生视觉语言模型，能在 1M 上下文中结合文字、图片和视频理解任务。采用线性注意力与稀疏 MoE 混合架构，覆盖长资料分析、视觉理解和编程工具使用。 | [来源1](https://help.aliyun.com/en/model-studio/qwen3-5-plus) |
| `qwen3.6-flash` | 已核实 | 面向高效编程智能体的视觉语言模型，兼顾数学与代码推理。除文字、视频和图片理解外，还能进行目标识别与空间定位，提供 1M 上下文。 | [来源1](https://help.aliyun.com/en/model-studio/qwen3-6-flash) |
| `qwen3.6-max-preview` | 已核实 | 千问 Max 系列的预览模型，提供 256K 上下文，适合深入推理、代码分析和长材料处理。支持思考模式、函数调用与结构化输出，官方表格未将百炼内置工具列为此版本的支持项。 | [来源1](https://help.aliyun.com/zh/model-studio/text-generation-model) |
| `qwen3.6-plus` | 已核实 | 加强根据视觉参考开发前端的能力，可从图片、视频与文字需求生成或修改代码。支持 1M 上下文，并覆盖 OCR、目标识别与视觉信息提取。 | [来源1](https://help.aliyun.com/en/model-studio/qwen3-6-plus) |
| `qwen3.7-flash` | 已核实 | 千问 Flash 系列的轻量模型，提供 1M 上下文，可用于长文档处理、大型代码材料分析和日常智能体任务。支持思考模式、函数调用、百炼内置工具及结构化输出，适合对调用成本较敏感的应用。 | [来源1](https://help.aliyun.com/zh/model-studio/text-generation-model) |
| `qwen3.7-max` | 已核实 | Qwen 3.7 系列面向高复杂度任务的模型，官方以文本接口提供 1M 上下文。重点是编程、办公生产力与长时间自主执行。 | [来源1](https://help.aliyun.com/en/model-studio/qwen3-7-max) |
| `qwen3.7-plus` | 已核实 | 以多模态交互式智能体为特色，能够读屏、理解场景并根据视觉参考编写代码。还覆盖移动应用导航，把图片、视频理解与 GUI 操作结合起来。 | [来源1](https://help.aliyun.com/en/model-studio/qwen3-7-plus) |
| `qwen3.8-27b` | 已核实 | Qwen3.8-27B 是 270 亿参数的稠密视觉语言模型，原生理解图片和视频，面向编程、专业工作及需要持续执行的智能体任务。原生上下文为 262,144 token，可扩展至 1M，并支持按请求关闭思考或调整推理深度。 | [来源1](https://huggingface.co/Qwen/Qwen3.8-27B) / [来源2](https://github.com/QwenLM/Qwen3.8) |
| `qwen3.8-flash` | 已核实 | 以较高吞吐处理多模态任务，能够分析图表、长视频和桌面画面，并参与编码与智能体工作。支持 1M 上下文，可通过 OpenAI 或 Anthropic 兼容协议接入。 | [来源1](https://help.aliyun.com/en/model-studio/qwen3-8-flash) |
| `qwen3.8-max` | 已核实 | 面向编码、办公、法律、金融和设计等专业任务的大规模 MoE 模型。原生理解图片与视频，利用 1M 上下文处理超长材料，并支持长程规划和迭代。 | [来源1](https://help.aliyun.com/en/model-studio/qwen3-8-max) |
| `seed-2.1-pro` | 待核实 | 官方 Seed2.1 Pro 面向复杂办公、专业研究和端到端代码工程，强化项目规划、文件处理与跨工具任务执行，并支持图像和视频理解。目录中 seed-2.1-pro 对应的具体官方发布版本与服务规格尚待确认。 | [来源1](https://seed.bytedance.com/zh/seed2_1) / [来源2](https://seed.bytedance.com/zh/blog/seed2-1-officially-released-advancing-ai-productivity) |
| `seed-2.1-turbo` | 待核实 | 官方 Seed2.1 Turbo 与 Pro 同属面向真实生产力场景的模型系列，采用不同模型尺寸，支持代码工程、办公工具协作以及图像和视频理解。此目录名称对应的具体发布版本、速度档位和上下文规格尚待确认。 | [来源1](https://seed.bytedance.com/zh/seed2_1) / [来源2](https://seed.bytedance.com/zh/blog/seed2-1-officially-released-advancing-ai-productivity) |
| `step-3.5-flash` | 已核实 | 以稀疏 MoE 架构实现高速文本推理，196B 总参数中每次激活 11B。提供 256K 上下文，重点是代码、数学和需要多步工具调用的研究任务。 | [来源1](https://platform.stepfun.com/docs/zh/guides/models/step-3.5-flash) |
| `step-3.5-flash-2603` | 已核实 | 基于 Step 3.5 Flash 针对高频智能体调用优化，改进 Token 使用效率、推理速度和编程框架兼容性。支持切换低推理模式，以缩短不需要深思的执行步骤。 | [来源1](https://platform.stepfun.com/docs/zh/step-plan/integrations/reasoning-api) |
| `step-3.7-flash` | 已核实 | 在高速推理中加入原生图片与视频理解，让智能体依据视觉材料规划并使用工具。提供 256K 上下文和 low、medium、high 三档推理强度，覆盖代码与视觉任务。 | [来源1](https://platform.stepfun.com/docs/zh/guides/models/step-3.7-flash) |
| `step-5-preview` | 已核实 | 面向真实任务交付的多模态预览模型，可同时理解文字、图片和视频。1M 上下文用于长文档、多材料研究及软件工程，单次文本输出上限为 64K。 | [来源1](https://platform.stepfun.com/docs/zh/guides/models/step-5-preview) |
| `step-image-edit-2` | 已核实 | 同时支持文生图和参考图编辑的轻量模型，重点是快速交互式修图。官方曾给出秒级生成定位，并公告于 2026-10-10 下线该型号。 | [来源1](https://platform.stepfun.com/docs/zh/guides/models/step-image-edit-2) |
| `step-router-v1` | 已核实 | Step Plan 的智能路由入口，按请求特点在 DeepSeek-V4-Pro 与 Step-3.7-Flash 等引擎间分配任务。复杂推理和高频执行可走不同引擎，因此它没有单一固定模型规格。 | [来源1](https://platform.stepfun.com/docs/zh/guides/models/step-router) |
| `stepaudio-2.5-asr` | 已核实 | 将语音转为文字的识别模型，支持整段音频、文件转写和实时双向 WebSocket。可以为会议记录、实时字幕和语音智能体提供转写结果。 | [来源1](https://platform.stepfun.com/docs/zh/guides/models/stepaudio-2.5-asr) |
| `stepaudio-2.5-chat` | 已核实 | 直接理解语音中的内容、语气、迟疑与轻笑，接收音频或文字后只返回文本。适合需要听懂表达方式的语音对话，但不直接生成语音回复。 | [来源1](https://platform.stepfun.com/docs/zh/guides/models/stepaudio-2.5-chat) |
| `stepaudio-2.5-realtime` | 已核实 | 通过 WebSocket 边接收语音边生成音频回复，支持语音活动检测、角色设定和音色复刻。回复可带轻笑、叹息等表现，侧重自然的连续语音互动。 | [来源1](https://platform.stepfun.com/docs/zh/guides/models/stepaudio-2.5-realtime) |
| `stepaudio-2.5-tts` | 已核实 | 可用全局和行内指令控制停顿、重音与情绪的语音合成模型。输入文本并生成音频，也能依据短参考录音复刻音色，用于配音、有声内容与情感播报。 | [来源1](https://platform.stepfun.com/docs/zh/guides/models/stepaudio-2.5-tts) |

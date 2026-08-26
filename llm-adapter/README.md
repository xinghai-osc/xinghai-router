# xinghai-router-llm-adapter

完整的 DeepSeek Harness Cordis LLM 插件，把 `@deepseek-ai/dsh-llm` 的 provider-neutral 请求转换为 Xinghai Router 的 OpenAI-compatible Chat Completions SSE 接口。插件入口导出 `name`、`inject` 和 `apply`，可直接交给 DSH/Cordis 加载。

## 直接作为 Cordis 插件

```ts
import * as xinghaiRouter from 'xinghai-router-llm-adapter'

export default {
  ...xinghaiRouter,
  config: {
    baseURL: 'https://platform.ai.hixinghai.top/api/v1',
    apiKeyEnv: 'XINGHAI_API_KEY',
    models: [{ id: 'deepseek-reasoner', contextWindow: 128000 }],
  },
}
```

如果你的 DSH 入口接受插件函数，也可以直接传入：

```ts
import { apply } from 'xinghai-router-llm-adapter'

apply(ctx, {
  baseURL: process.env.XINGHAI_ROUTER_URL,
  apiKeyEnv: 'XINGHAI_API_KEY',
})
```

插件会注册 provider route `xinghai-router`，并在 Cordis fiber 销毁时由注册句柄自动释放。

## 安装进 DSH profile

这个 npm 包已经声明了 `dsh.bundle.patch`，所以应通过 profile 的插件管理命令安装：

```sh
dsh plugin --profile headless add xinghai-router-llm-adapter
```

或者安装到 Web profile：

```sh
dsh plugin --profile web add xinghai-router-llm-adapter
```

安装后，在对应 profile 的 `cordis.patch.yml` 中覆盖 bundle 默认配置。注意 patch 会替换整个 config：

```yaml
- id: llm-xinghai-router
  config:
    baseURL: https://platform.ai.hixinghai.top/api/v1
    apiKeyEnv: XINGHAI_API_KEY
    models:
      - id: deepseek-reasoner
        name: DeepSeek Reasoner
        contextWindow: 128000
        maxTokens: 8192
```

然后设置密钥并启动：

```sh
export XINGHAI_API_KEY=sk-xinghai-your-key
dsh --profile headless "你好"
```

适配器 provider/model 默认选择仍需由 profile 的 `agent-default-model` 行或应用配置指定：

```yaml
- id: agent-default-model
  config:
    provider: xinghai-router
    model: deepseek-reasoner
```

当 `baseURL` 为 `https://platform.ai.hixinghai.top/api/v1` 时，实际请求地址为 `https://platform.ai.hixinghai.top/api/v1/chat/completions`。

适配器特性：

- 独立 provider route：`xinghai-router`；
- `listModels()` 与 `resolveModel()` 提供可选模型目录、上下文窗口和输出上限；
- 使用标准 `StreamChunk`：文本、reasoning、tool-call、usage、finish；
- 转发 `AbortSignal`，识别认证、限流、服务端、上下文超限和传输错误；
- 每个请求携带 DSH attribution headers 与 Bearer API key；
- 只依赖 `@deepseek-ai/dsh-llm` peer dependency，不修改 Xinghai Router 现有网关逻辑。

构建：

```sh
cd llm-adapter
pnpm install
pnpm run build
```

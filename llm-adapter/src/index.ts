import type { Context } from '@deepseek-ai/cordis'
import {
  LlmAdapter,
  LlmError,
  attributionHeaders,
  type GenerateOptions,
  type LlmModelInfo,
  type LlmProviderInfo,
  type LlmResolvedModelInfo,
  type ResolvedRetryPolicy,
  type StreamChunk,
  type CallId,
  ReasoningEffortId,
} from '@deepseek-ai/dsh-llm'

export const name = 'llm-xinghai-router'
export const inject = ['llm']
export const XINGHAI_PROVIDER = 'xinghai-router'
export const DEFAULT_BASE_URL = 'https://platform.ai.hixinghai.top/api/v1'

export interface XinghaiModel {
  id: string
  name?: string
  description?: string
  contextWindow?: number
  maxTokens?: number
  reasoningEfforts?: Record<string, string | null>
  defaultReasoningEffort?: string
}

export interface XinghaiAdapterOptions {
  baseURL: string
  apiKey?: string
  models?: readonly XinghaiModel[]
  defaultContextWindow?: number
  defaultMaxTokens?: number
  retryPolicy?: ResolvedRetryPolicy
  fetch?: typeof globalThis.fetch
  userAgent?: string
}

export interface Config {
  baseURL?: string
  apiKey?: string
  apiKeyEnv?: string
  models?: XinghaiModel[]
  defaultContextWindow?: number
  defaultMaxTokens?: number
  retryPolicy?: ResolvedRetryPolicy
  userAgent?: string
}

export const resolveConfig = (config: Config = {}): XinghaiAdapterOptions => {
  const env = (globalThis as { process?: { env: Record<string, string | undefined> } }).process?.env ?? {}
  const apiKeyEnv = config.apiKeyEnv ?? 'XINGHAI_API_KEY'
  const apiKey = config.apiKey ?? env[apiKeyEnv] ?? ''
  const baseURL = config.baseURL ?? env.XINGHAI_ROUTER_URL ?? DEFAULT_BASE_URL
  if (!baseURL.trim()) throw new Error(`${name}: baseURL must not be empty`)
  if (config.defaultContextWindow !== undefined && (!Number.isSafeInteger(config.defaultContextWindow) || config.defaultContextWindow <= 0)) {
    throw new Error(`${name}: defaultContextWindow must be a positive integer`)
  }
  if (config.defaultMaxTokens !== undefined && (!Number.isSafeInteger(config.defaultMaxTokens) || config.defaultMaxTokens <= 0)) {
    throw new Error(`${name}: defaultMaxTokens must be a positive integer`)
  }
  const seen = new Set<string>()
  for (const model of config.models ?? []) {
    if (!model.id.trim() || seen.has(model.id)) throw new Error(`${name}: model ids must be non-empty and unique`)
    seen.add(model.id)
  }
  return { ...config, baseURL, apiKey, models: config.models ?? [] }
}

interface ModelsResponse {
  data?: Array<{
    id?: string
    name?: string
    description?: string
  }>
}

interface WireChunk {
  id?: string
  choices?: Array<{
    index?: number
    delta?: { content?: string | null; reasoning_content?: string | null; tool_calls?: Array<{ index?: number; id?: string; function?: { name?: string; arguments?: string } }> }
    finish_reason?: string | null
  }>
  usage?: { prompt_tokens?: number; completion_tokens?: number; prompt_tokens_details?: { cached_tokens?: number } }
}

function modelInfo(provider: string, model: XinghaiModel): LlmModelInfo {
  return { provider, id: model.id, name: model.name ?? model.id, description: model.description, inputModalities: ['text'] }
}

function modelReasoning(model: XinghaiModel) {
  const efforts = Object.entries(model.reasoningEfforts ?? {})
  if (efforts.length === 0) return undefined
  return {
    efforts: efforts.map(([id]) => ({ id: ReasoningEffortId(id), name: id === 'off' ? 'Off' : id.charAt(0).toUpperCase() + id.slice(1) })),
    ...(model.defaultReasoningEffort ? { defaultEffort: ReasoningEffortId(model.defaultReasoningEffort) } : {}),
  }
}

function failure(message: string, code: string) {
  return { kind: 'error' as const, failure: { message, code } }
}

function classifyStatus(status: number, message: string): string {
  if (status === 401 || status === 403) return 'AUTH'
  if (status === 429) return 'RATE_LIMIT'
  if (status >= 500) return 'SERVER'
  if (/context|token limit|too long/i.test(message)) return 'CONTEXT_WINDOW_EXCEEDED'
  return status === 400 ? 'INVALID_REQUEST' : `HTTP_${status}`
}

export class XinghaiRouterAdapter extends LlmAdapter {
  private readonly config: XinghaiAdapterOptions
  private readonly configuredModels: readonly XinghaiModel[]
  private discoveredModels: readonly XinghaiModel[] | undefined

  constructor(options: XinghaiAdapterOptions) {
    super()
    this.config = { ...options, baseURL: options.baseURL.replace(/\/$/, '') }
    this.configuredModels = options.models ?? []
    if (!this.config.baseURL) throw new TypeError('baseURL is required')
  }

  providerInfo(provider: string): LlmProviderInfo {
    return { id: provider, name: 'Xinghai Router' }
  }

  providerRetryPolicy(_provider: string): ResolvedRetryPolicy | undefined {
    return this.config.retryPolicy
  }

  async listModels(provider: string): Promise<readonly LlmModelInfo[]> {
    const configured = new Map(this.configuredModels.map((model) => [model.id, model]))
    try {
      const response = await this.fetchModels()
      const discovered = response.data
        ?.filter((model): model is { id: string; name?: string; description?: string } => typeof model.id === 'string' && model.id.trim().length > 0)
        .map((model) => ({
          ...configured.get(model.id),
          id: model.id,
          ...(model.name ? { name: model.name } : {}),
          ...(model.description ? { description: model.description } : {}),
        })) ?? []
      this.discoveredModels = discovered
      return discovered.map((model) => modelInfo(provider, model))
    } catch {
      const fallback = this.discoveredModels ?? this.configuredModels
      return fallback.map((model) => modelInfo(provider, model))
    }
  }

  private async fetchModels(): Promise<ModelsResponse> {
    const apiKey = this.config.apiKey?.trim()
    const response = await (this.config.fetch ?? fetch)(`${this.config.baseURL}/models`, {
      headers: {
        ...attributionHeaders(),
        ...(this.config.userAgent ? { 'user-agent': this.config.userAgent } : {}),
        ...(apiKey ? { authorization: `Bearer ${apiKey}` } : {}),
        accept: 'application/json',
      },
    })
    if (!response.ok) throw new Error(`Xinghai Router models endpoint returned HTTP ${response.status}`)
    return await response.json() as ModelsResponse
  }

  async resolveModel(provider: string, model: string): Promise<LlmResolvedModelInfo> {
    const configured = (this.discoveredModels ?? this.configuredModels).find((item) => item.id === model)
    return {
      ...(configured ? modelInfo(provider, configured) : { provider, id: model, name: model, inputModalities: ['text'] as const }),
      ...(this.config.defaultContextWindow || configured?.contextWindow ? { context: { contextWindow: configured?.contextWindow ?? this.config.defaultContextWindow! } } : {}),
      ...(configured?.maxTokens ?? this.config.defaultMaxTokens ? { defaultMaxTokens: configured?.maxTokens ?? this.config.defaultMaxTokens } : {}),
      ...(configured ? { reasoning: modelReasoning(configured) } : {}),
    }
  }

  async *stream(options: GenerateOptions): AsyncIterable<StreamChunk> {
    const apiKey = this.config.apiKey?.trim()
    if (!apiKey) throw new LlmError('Xinghai Router API key is not configured', 'MISSING_CREDENTIAL')
    const controller = new AbortController()
    const signal = options.signal ? AbortSignal.any([options.signal, controller.signal]) : controller.signal
    const body = {
      model: options.model,
      messages: [
        ...(options.system ? [{ role: 'system', content: options.system }] : []),
        ...options.messages.map((message) => ({ role: message.role, content: message.content.filter((block) => block.type === 'text').map((block) => block.text).join('') })),
      ],
      stream: true,
      stream_options: { include_usage: true },
      ...(options.temperature === undefined ? {} : { temperature: options.temperature }),
      ...(options.reasoningEffort === undefined || options.reasoningEffort === 'off' ? {} : { reasoning_effort: options.reasoningEffort }),
      ...(options.maxTokens === undefined ? {} : { max_tokens: options.maxTokens }),
      ...(options.stop === undefined ? {} : { stop: options.stop }),
      ...(options.tools?.length ? { tools: options.tools.map((tool) => ({ type: 'function', function: { name: tool.name, description: tool.description, parameters: tool.parameters } })) } : {}),
    }
    let response: Response
    try {
      response = await (this.config.fetch ?? fetch)(`${this.config.baseURL}/chat/completions`, {
        method: 'POST',
        signal,
        headers: { ...attributionHeaders(), ...(this.config.userAgent ? { 'user-agent': this.config.userAgent } : {}), authorization: `Bearer ${apiKey}`, 'content-type': 'application/json', accept: 'text/event-stream' },
        body: JSON.stringify(body),
      })
    } catch (error) {
      if (options.signal?.aborted) throw new LlmError('Xinghai Router request aborted', 'ABORTED', { cause: error })
      throw new LlmError('Xinghai Router transport failed', 'TRANSPORT', { cause: error })
    }
    if (!response.ok) {
      const text = await response.text().catch(() => '')
      const message = text || `Xinghai Router returned HTTP ${response.status}`
      throw new LlmError(message, classifyStatus(response.status, message), { status: response.status })
    }
    if (!response.body) throw new LlmError('Xinghai Router returned no response body', 'EMPTY_RESPONSE')

    const reader = response.body.pipeThrough(new TextDecoderStream()).getReader()
    let buffer = ''
    const toolNames = new Map<number, { id: CallId; name: string }>()
    try {
      while (true) {
        const part = await reader.read()
        if (part.done) break
        buffer += part.value
        const events = buffer.split(/\r?\n\r?\n/)
        buffer = events.pop() ?? ''
        for (const event of events) {
          const line = event.split(/\r?\n/).find((item) => item.startsWith('data:'))
          if (!line) continue
          const raw = line.slice(5).trim()
          if (raw === '[DONE]') continue
          let chunk: WireChunk
          try { chunk = JSON.parse(raw) as WireChunk } catch { throw new LlmError('Malformed Xinghai Router SSE payload', 'MALFORMED_RESPONSE') }
          if (chunk.usage) yield { type: 'usage', usage: { inputTokens: Math.max(0, (chunk.usage.prompt_tokens ?? 0) - (chunk.usage.prompt_tokens_details?.cached_tokens ?? 0)), outputTokens: chunk.usage.completion_tokens ?? 0, ...(chunk.usage.prompt_tokens_details?.cached_tokens ? { cacheReadTokens: chunk.usage.prompt_tokens_details.cached_tokens } : {}) } }
          for (const choice of chunk.choices ?? []) {
            const delta = choice.delta ?? {}
            const index = choice.index ?? 0
            if (delta.reasoning_content) yield { type: 'reasoning-delta', index, text: delta.reasoning_content }
            if (delta.content) yield { type: 'text-delta', index, text: delta.content }
            for (const call of delta.tool_calls ?? []) {
              const toolIndex = call.index ?? 0
              const previous = toolNames.get(toolIndex)
              if (call.id || call.function?.name) {
                const id = (call.id ?? previous?.id ?? `call_${toolIndex}`) as CallId
                const name = call.function?.name ?? previous?.name ?? ''
                toolNames.set(toolIndex, { id, name })
                yield { type: 'tool-call-delta', index: toolIndex, id, name: name || undefined, argumentsDelta: call.function?.arguments ?? '' }
              } else if (call.function?.arguments && previous) {
                yield { type: 'tool-call-delta', index: toolIndex, id: previous.id, argumentsDelta: call.function.arguments }
              }
            }
            if (choice.finish_reason) {
              const reason = choice.finish_reason === 'tool_calls' ? { kind: 'tool-calls' as const } : choice.finish_reason === 'length' ? { kind: 'max-tokens' as const } : choice.finish_reason === 'stop' ? { kind: 'stop' as const } : failure(`Provider finish reason: ${choice.finish_reason}`, 'PROVIDER_FINISH')
              yield { type: 'finish', reason }
            }
          }
        }
      }
    } finally {
      controller.abort()
      reader.releaseLock()
    }
  }
}

export function apply(ctx: Context, config: Config = {}): void {
  const resolved = resolveConfig(config)
  const adapter = new XinghaiRouterAdapter(resolved)
  ctx.llm.registerConfigurableProviders([{
    provider: XINGHAI_PROVIDER,
    displayName: 'Xinghai Router',
    settingsNs: name,
    settingsPath: [],
  }])
  ctx.llm.registerAdapter([XINGHAI_PROVIDER], adapter)
}

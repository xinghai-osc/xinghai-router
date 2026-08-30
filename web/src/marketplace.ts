import type { CatalogGroup, CatalogModel, ModelCatalogMetadata, ModelCatalogPerformance } from '~/src/api'

// ----------------------------------------------------------------------------
// Model square (模型广场) pricing and filtering logic.
// Prices are stored per 1M tokens and scaled by model multiplier × group
// multiplier before display.
// ----------------------------------------------------------------------------

export type TokenUnit = 'M' | 'K'
export type ViewMode = 'card' | 'table'
export type SortOption = 'name' | 'price-low' | 'price-high'
export type PriceKind = 'input' | 'output' | 'cache'

export const FILTER_ALL = 'all'
export const PAGE_SIZE = 24

export interface SquareModel extends CatalogModel {
  vendor_name: string
  vendor_slug: string
}

export type Modality = 'text' | 'image' | 'audio' | 'video' | 'file'
export type ContextBucket = '64k' | '128k' | '256k' | '1m' | '1m-plus'

export function normalizeCatalogValue(value: string): string {
  return value.normalize('NFKC').trim().toLowerCase()
}

export function squareModelKey(model: Pick<SquareModel, 'model' | 'vendor_slug'>): string {
  return `${normalizeCatalogValue(model.model)}\u0000${normalizeCatalogValue(model.vendor_slug)}`
}

export function toSquareModel(item: CatalogModel): SquareModel {
  const model: SquareModel = {
    ...item,
    description: item.description ?? null,
    input_modalities: item.input_modalities ?? [],
    output_modalities: item.output_modalities ?? [],
    performance: item.performance ?? null,
    vendor_name: item.provider,
    vendor_slug: item.provider_slug,
  }
  return { ...model, id: model.id || squareModelKey(model) }
}

export function dedupeSquareModels(models: SquareModel[]): SquareModel[] {
  const unique = new Map<string, SquareModel>()
  for (const model of models) {
    const key = squareModelKey(model)
    const existing = unique.get(key)
    if (!existing) {
      unique.set(key, { ...model, groups: [...model.groups] })
      continue
    }

    const groups = new Map(existing.groups.map(group => [group.id, group]))
    for (const group of model.groups) groups.set(group.id, group)
    existing.groups = [...groups.values()]
    existing.input_per_million ??= model.input_per_million
    existing.cached_input_per_million ??= model.cached_input_per_million
    existing.output_per_million ??= model.output_per_million
    existing.multiplier ??= model.multiplier
  }
  return [...unique.values()]
}

/** Unique vendors across the catalog, with model counts, preserving order. */
export function extractVendors(models: SquareModel[]) {
  const map = new Map<string, { name: string; slug: string; count: number }>()
  for (const model of models) {
    const entry = map.get(model.vendor_name)
    if (entry) entry.count += 1
    else map.set(model.vendor_name, { name: model.vendor_name, slug: model.vendor_slug, count: 1 })
  }
  return [...map.values()].sort((a, b) => b.count - a.count)
}

/** Lowest-multiplier group of a model, or the selected group when filtering. */
export function getDisplayGroup(model: CatalogModel, selectedGroup: string): CatalogGroup | undefined {
  const groups = model.groups
  if (!groups.length) return undefined
  if (selectedGroup !== FILTER_ALL) {
    const hit = groups.find(group => group.id === selectedGroup)
    if (hit) return hit
  }
  let best = groups[0]!
  for (const group of groups.slice(1)) {
    if (Number(group.multiplier) < Number(best.multiplier)) best = group
  }
  return best
}

function basePrice(model: CatalogModel, kind: PriceKind): number | null {
  const value = kind === 'input'
    ? model.input_per_million
    : kind === 'output' ? model.output_per_million : model.cached_input_per_million
  if (value == null) return null
  const numeric = Number(value)
  // The backend stores an omitted cached-input price as 0 — treat it as unconfigured.
  if (kind === 'cache' && numeric === 0) return null
  return numeric
}

/** Effective local-currency price per 1M tokens for the display group. */
export function effectivePrice(model: CatalogModel, kind: PriceKind, selectedGroup: string): number | null {
  const base = basePrice(model, kind)
  if (base == null) return null
  const group = getDisplayGroup(model, selectedGroup)
  return base * Number(model.multiplier ?? 1) * Number(group?.multiplier ?? 1)
}

/** Effective price for a concrete group (used by the per-group pricing table). */
export function groupPrice(model: CatalogModel, kind: PriceKind, group: CatalogGroup): number | null {
  const base = basePrice(model, kind)
  if (base == null) return null
  return base * Number(model.multiplier ?? 1) * Number(group.multiplier ?? 1)
}

/** Format a per-1M price for the chosen token unit, trimming trailing zeros. */
export function formatSquarePrice(value: number | null, unit: TokenUnit): string {
  if (value == null) return '—'
  const scaled = unit === 'K' ? value / 1000 : value
  if (scaled === 0) return '¥0'
  const digits = scaled >= 100 ? 2 : scaled >= 1 ? 4 : 6
  return `¥${String(Number.parseFloat(scaled.toFixed(digits)))}`
}

/** Format a group multiplier like x1 / x0.5 / x1.25. */
export function formatRatio(multiplier: number | string): string {
  const value = Number(multiplier)
  if (!Number.isFinite(value)) return 'x1'
  const formatted = Number.isInteger(value)
    ? String(value)
    : value.toFixed(3).replace(/0+$/, '').replace(/\.$/, '')
  return `x${formatted}`
}

/** Format the recent request rate, trimming trailing zeros. */
export function formatRequestRate(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return '0'
  const formatted = value >= 1 ? value.toFixed(2) : value.toPrecision(2)
  return String(Number.parseFloat(formatted))
}

/** Format an average latency in milliseconds, upgrading to seconds above 1s. */
export function formatLatency(ms: number): string {
  if (!Number.isFinite(ms) || ms <= 0) return '—'
  if (ms >= 1000) return `${(ms / 1000).toFixed(2)}s`
  return `${Math.round(ms)}ms`
}

/** Format a success rate (0..1) as a percentage. */
export function formatSuccessRate(rate: number): string {
  if (!Number.isFinite(rate)) return '—'
  return `${(rate * 100).toFixed(1)}%`
}

/** First meaningful character of a vendor name, used as its monogram. */
export function vendorInitial(name: string): string {
  return (name.trim()[0] ?? '?').toUpperCase()
}

export function modelMetadata(model: SquareModel): ModelCatalogMetadata {
  return {
    description: model.description,
    input_modalities: model.input_modalities,
    output_modalities: model.output_modalities,
    context_window: model.context_window,
  }
}

export function modelPerformance(model: SquareModel): ModelCatalogPerformance {
  return model.performance ?? {
    requests: null,
    success_rate: null,
    avg_latency_ms: null,
    avg_first_token_ms: null,
  }
}

export function contextBucket(value: number | null | undefined): ContextBucket | null {
  if (value == null || !Number.isFinite(Number(value)) || Number(value) <= 0) return null
  const context = Number(value)
  if (context <= 64_000) return '64k'
  if (context <= 128_000) return '128k'
  if (context <= 256_000) return '256k'
  if (context <= 1_000_000) return '1m'
  return '1m-plus'
}

export function matchesContextBucket(value: number | null | undefined, buckets: ContextBucket[]): boolean {
  if (!buckets.length) return true
  const bucket = contextBucket(value)
  return bucket != null && buckets.includes(bucket)
}

export function modelSupportsModality(model: SquareModel, field: 'input_modalities' | 'output_modalities', modalities: Modality[]): boolean {
  if (!modalities.length) return true
  const values = modelMetadata(model)[field] ?? []
  return modalities.every(modality => values.includes(modality))
}

export function formatContextWindow(value: number | null | undefined): string {
  if (value == null || !Number.isFinite(Number(value)) || Number(value) <= 0) return '—'
  const context = Number(value)
  if (context >= 1_000_000) return `${Number((context / 1_000_000).toFixed(2))}M`
  if (context >= 1_000) return `${Number((context / 1_000).toFixed(1))}K`
  return String(Math.round(context))
}

export interface SquareFilters {
  search: string
  vendor: string
  group: string
  sortBy: SortOption
  inputModalities?: Modality[]
  outputModalities?: Modality[]
  contextBuckets?: ContextBucket[]
}

function searchText(value: string): string {
  return normalizeCatalogValue(value).replace(/[^\p{L}\p{N}]+/gu, ' ').trim()
}

function searchScore(model: SquareModel, query: string): number | null {
  const modelText = searchText(model.model)
  const vendorText = searchText(model.vendor_name)
  const queryText = searchText(query)
  if (!queryText) return 0

  const tokens = queryText.split(' ').filter(Boolean)
  const modelTokens = modelText.split(' ').filter(Boolean)
  const vendorTokens = vendorText.split(' ').filter(Boolean)
  const matchesToken = (token: string) =>
    [...modelTokens, ...vendorTokens].some(candidate => candidate === token || candidate.startsWith(token))
  if (!tokens.every(matchesToken)) return null

  let score = 0
  if (modelText === queryText) score += 1000
  else if (vendorText === queryText) score += 900
  else if (modelText.startsWith(queryText)) score += 700
  else if (vendorText.startsWith(queryText)) score += 650

  for (const token of tokens) {
    if (modelTokens.some(candidate => candidate === token)) score += 30
    else if (vendorTokens.some(candidate => candidate === token)) score += 20
    else if (modelTokens.some(candidate => candidate.startsWith(token))) score += 10
    else score += 5
  }
  return score
}

export function filterAndSort(models: SquareModel[], filters: SquareFilters): SquareModel[] {
  const query = filters.search.trim()
  let result = dedupeSquareModels(models)
  const scores = new Map<string, number>()

  if (query) {
    result = result.filter((model) => {
      const score = searchScore(model, query)
      if (score == null) return false
      scores.set(squareModelKey(model), score)
      return true
    })
  }
  if (filters.vendor !== FILTER_ALL) result = result.filter(model => model.vendor_name === filters.vendor)
  if (filters.group !== FILTER_ALL) result = result.filter(model => model.groups.some(group => group.id === filters.group))
  if (filters.inputModalities?.length) {
    result = result.filter(model => modelSupportsModality(model, 'input_modalities', filters.inputModalities!))
  }
  if (filters.outputModalities?.length) {
    result = result.filter(model => modelSupportsModality(model, 'output_modalities', filters.outputModalities!))
  }
  if (filters.contextBuckets?.length) {
    result = result.filter(model => matchesContextBucket(modelMetadata(model).context_window, filters.contextBuckets!))
  }

  const sorted = [...result]
  const byName = (a: SquareModel, b: SquareModel) =>
    a.model.localeCompare(b.model) || squareModelKey(a).localeCompare(squareModelKey(b))
  const byPriceLow = (a: SquareModel, b: SquareModel) =>
    (effectivePrice(a, 'input', filters.group) ?? Number.POSITIVE_INFINITY)
    - (effectivePrice(b, 'input', filters.group) ?? Number.POSITIVE_INFINITY) || byName(a, b)
  const byPriceHigh = (a: SquareModel, b: SquareModel) =>
    (effectivePrice(b, 'input', filters.group) ?? -1)
    - (effectivePrice(a, 'input', filters.group) ?? -1) || byName(a, b)
  const sortResult = filters.sortBy === 'price-low'
    ? byPriceLow
    : filters.sortBy === 'price-high' ? byPriceHigh : byName

  sorted.sort((a, b) => {
    if (query) {
      const relevance = (scores.get(squareModelKey(b)) ?? 0) - (scores.get(squareModelKey(a)) ?? 0)
      if (relevance) return relevance
    }
    return sortResult(a, b)
  })
  return sorted
}

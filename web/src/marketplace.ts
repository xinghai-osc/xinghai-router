import type { CatalogGroup, CatalogModel } from '~/src/api'

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

export function toSquareModel(item: CatalogModel): SquareModel {
  return { ...item, vendor_name: item.provider, vendor_slug: item.provider_slug }
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

/** Format a requests-per-second figure, trimming trailing zeros. */
export function formatTPS(value: number): string {
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

/** Deterministic accent colour for a vendor monogram. */
export function vendorColor(name: string): { bg: string; fg: string } {
  let hash = 0
  for (let index = 0; index < name.length; index += 1) hash = (hash * 31 + name.charCodeAt(index)) >>> 0
  const hue = hash % 360
  return { bg: `hsl(${hue} 60% 45% / 0.12)`, fg: `hsl(${hue} 45% 42%)` }
}

/** First meaningful character of a vendor name, used as its monogram. */
export function vendorInitial(name: string): string {
  return (name.trim()[0] ?? '?').toUpperCase()
}

export interface SquareFilters {
  search: string
  vendor: string
  group: string
  sortBy: SortOption
}

export function filterAndSort(models: SquareModel[], filters: SquareFilters): SquareModel[] {
  const query = filters.search.trim().toLowerCase()
  let result = models

  if (query) {
    result = result.filter(model =>
      model.model.toLowerCase().includes(query) || model.vendor_name.toLowerCase().includes(query),
    )
  }
  if (filters.vendor !== FILTER_ALL) result = result.filter(model => model.vendor_name === filters.vendor)
  if (filters.group !== FILTER_ALL) result = result.filter(model => model.groups.some(group => group.id === filters.group))

  const sorted = [...result]
  if (filters.sortBy === 'name') {
    sorted.sort((a, b) => a.model.localeCompare(b.model))
  } else if (filters.sortBy === 'price-low') {
    sorted.sort((a, b) =>
      (effectivePrice(a, 'input', filters.group) ?? Number.POSITIVE_INFINITY)
      - (effectivePrice(b, 'input', filters.group) ?? Number.POSITIVE_INFINITY))
  } else if (filters.sortBy === 'price-high') {
    sorted.sort((a, b) =>
      (effectivePrice(b, 'input', filters.group) ?? -1) - (effectivePrice(a, 'input', filters.group) ?? -1))
  }
  return sorted
}

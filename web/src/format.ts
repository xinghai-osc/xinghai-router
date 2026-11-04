import { ref } from 'vue'

/** Shared display formatters. */

const POINTS_NAME = ref('积分')
export function setPointsName(name: string) {
  POINTS_NAME.value = name.trim() || '积分'
}

const NUMBER = new Intl.NumberFormat('zh-CN')
const DATE = new Intl.DateTimeFormat('zh-CN', {
  year: 'numeric', month: '2-digit', day: '2-digit',
  hour: '2-digit', minute: '2-digit', hour12: false,
})
const DAY = new Intl.DateTimeFormat('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit' })

export function formatNumber(value: number | string | null | undefined): string {
  const numeric = Number(value ?? 0)
  return Number.isFinite(numeric) ? NUMBER.format(numeric) : '—'
}

/** 1_284_302 → "1.28M". Used where column width matters more than precision. */
export function formatCompact(value: number | string | null | undefined): string {
  const numeric = Number(value ?? 0)
  if (!Number.isFinite(numeric)) return '—'
  const abs = Math.abs(numeric)
  if (abs >= 1e9) return `${(numeric / 1e9).toFixed(2)}B`
  if (abs >= 1e6) return `${(numeric / 1e6).toFixed(2)}M`
  if (abs >= 1e4) return `${(numeric / 1e3).toFixed(1)}K`
  return NUMBER.format(numeric)
}

export function formatMoney(value: number | string | null | undefined, digits = 2): string {
  const numeric = Number(value ?? 0)
  if (!Number.isFinite(numeric)) return '—'
  return `${numeric.toFixed(digits)} ${POINTS_NAME.value}`
}

/** Subscription prices are quoted in yuan regardless of the site's point unit. */
export function formatCNY(value: number | string | null | undefined, digits = 2): string {
  const numeric = Number(value ?? 0)
  if (!Number.isFinite(numeric)) return '—'
  return `¥${numeric.toFixed(digits)}`
}

/** Format a money amount by its currency code: CNY as ¥, anything else with the code. */
export function formatCurrency(value: number | string | null | undefined, currency?: string | null, digits = 2): string {
  const code = (currency ?? 'CNY').trim().toUpperCase()
  if (!code || code === 'CNY' || code === 'RMB') return formatCNY(value, digits)
  const numeric = Number(value ?? 0)
  if (!Number.isFinite(numeric)) return '—'
  return `${numeric.toFixed(digits)} ${code}`
}

export function formatPercent(value: number | null | undefined, digits = 1): string {
  const numeric = Number(value ?? 0)
  if (!Number.isFinite(numeric)) return '—'
  return `${numeric.toFixed(digits)}%`
}

export function formatSignedPercent(value: number | null | undefined, digits = 1): string {
  const numeric = Number(value ?? 0)
  if (!Number.isFinite(numeric) || numeric === 0) return '—'
  return `${numeric > 0 ? '+' : ''}${numeric.toFixed(digits)}%`
}

export function formatDateTime(value: string | null | undefined): string {
  if (!value) return '—'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '—' : DATE.format(date)
}

export function formatDate(value: string | null | undefined): string {
  if (!value) return '—'
  const dateOnly = /^\d{4}-\d{2}-\d{2}$/.test(value)
  const date = dateOnly ? new Date(value + 'T00:00:00') : new Date(value)
  return Number.isNaN(date.getTime()) ? '—' : DAY.format(date)
}

/** Shorten an opaque id for table display: 8 leading characters. */
export function shortId(value: string | null | undefined): string {
  return value ? value.slice(0, 8) : '—'
}

<script setup lang="ts">
import { useElementSize } from '@vueuse/core'
import type { ModelPerformance, ModelPerformancePoint } from '~/src/api'
import { formatDateTime, formatNumber } from '~/src/format'
import { formatLatency, formatRequestRate, formatSuccessRate } from '~/src/marketplace'

const props = defineProps<{ performance: ModelPerformance }>()
const { t, locale } = useI18n()
const group = ref('')
const metric = ref('avg_latency_ms')
const activeIndex = ref(0)
const chart = ref<SVGSVGElement | null>(null)
const { width: chartWidth } = useElementSize(chart)
const groupInputId = useId()

type Metric = 'avg_latency_ms' | 'avg_first_token_ms' | 'tps' | 'success_rate'
const metrics = computed(() => [
  { value: 'avg_latency_ms', label: t('site.sqDetailPerfLatency') },
  { value: 'avg_first_token_ms', label: t('site.sqDetailPerfFirstToken') },
  { value: 'tps', label: t('site.sqDetailPerfRate') },
  { value: 'success_rate', label: t('site.sqDetailPerfSuccess') },
])
const metricLabel = computed(() => metrics.value.find(item => item.value === metric.value)?.label ?? '')
const groupOptions = computed(() => props.performance.groups.map(item => ({ value: item.group_id, label: item.group_name })))
const points = computed(() => props.performance.series?.find(item => item.group_id === group.value)?.points ?? [])

watch(groupOptions, (options) => {
  if (!options.some(option => option.value === group.value)) group.value = options[0]?.value ?? ''
}, { immediate: true })
watch(points, (next) => { activeIndex.value = Math.max(0, next.length - 1) }, { immediate: true })

const WIDTH = computed(() => Math.max(240, chartWidth.value || 600))
const HEIGHT = 230
const LEFT = 62
const RIGHT = 12
const TOP = 18
const BOTTOM = 34
const plotWidth = computed(() => WIDTH.value - LEFT - RIGHT)
const plotHeight = HEIGHT - TOP - BOTTOM

function valueOf(point: ModelPerformancePoint): number | null {
  const value = point[metric.value as Metric]
  return value != null && Number.isFinite(value) ? value : null
}

function formatValue(value: number | null): string {
  if (value == null) return t('site.sqUnavailable')
  if (metric.value === 'success_rate') return formatSuccessRate(value)
  if (metric.value === 'tps') return formatRequestRate(value)
  return value === 0 ? t('site.sqPerfZeroLatency') : formatLatency(value)
}

const peak = computed(() => metric.value === 'success_rate' ? 1 : Math.max(...points.value.map(point => valueOf(point) ?? 0), 0) || 1)
const nodes = computed(() => points.value.map((point, index) => {
  const value = valueOf(point)
  return {
    point,
    value,
    x: LEFT + index / Math.max(points.value.length - 1, 1) * plotWidth.value,
    y: value == null ? null : TOP + plotHeight * (1 - value / peak.value),
  }
}))
const path = computed(() => {
  let connected = false
  return nodes.value.map((node) => {
    if (node.y == null) {
      connected = false
      return ''
    }
    const command = connected ? 'L' : 'M'
    connected = true
    return `${command}${node.x},${node.y}`
  }).join(' ')
})
const hasValues = computed(() => nodes.value.some(node => node.value != null))
const active = computed(() => nodes.value[activeIndex.value])
const ticks = computed(() => [1, 0.5, 0].map(ratio => ({ y: TOP + plotHeight * (1 - ratio), label: formatValue(peak.value * ratio) })))
const axisNodes = computed(() => [nodes.value[0], nodes.value[Math.floor((nodes.value.length - 1) / 2)], nodes.value.at(-1)].filter(node => node != null))
const timeFormat = computed(() => new Intl.DateTimeFormat(locale.value, { hour: '2-digit', minute: '2-digit', hourCycle: 'h23' }))
const timeZone = computed(() => timeFormat.value.resolvedOptions().timeZone)
const intervalLabel = computed(() => {
  if (!active.value) return ''
  const start = new Date(active.value.point.timestamp)
  const end = new Date(start.getTime() + props.performance.bucket_minutes * 60_000)
  return t('site.sqPerfInterval', { start: formatDateTime(start.toISOString()), end: timeFormat.value.format(end) })
})
const activeLabel = computed(() => active.value ? t('site.sqPerfPointLabel', {
  time: intervalLabel.value,
  metric: metricLabel.value,
  value: formatValue(active.value.value),
  count: formatNumber(active.value.point.requests),
}) : '')

function inspectPoint(event: PointerEvent) {
  if (!chart.value || !points.value.length) return
  const bounds = chart.value.getBoundingClientRect()
  const position = (event.clientX - bounds.left) / bounds.width * WIDTH.value
  activeIndex.value = Math.min(points.value.length - 1, Math.max(0, Math.round((position - LEFT) / plotWidth.value * (points.value.length - 1))))
}

function stepPoint(event: KeyboardEvent) {
  if (!['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown', 'Home', 'End'].includes(event.key)) return
  event.preventDefault()
  const last = points.value.length - 1
  if (event.key === 'Home') activeIndex.value = 0
  else if (event.key === 'End') activeIndex.value = last
  else activeIndex.value = Math.max(0, Math.min(last, activeIndex.value + (['ArrowRight', 'ArrowUp'].includes(event.key) ? 1 : -1)))
}
</script>

<template>
  <section class="min-w-0 space-y-4 rounded-control border border-line bg-surface p-3.5 sm:p-4" :aria-label="t('site.sqPerfCurveTitle')">
    <div class="flex flex-wrap items-end justify-between gap-3">
      <div>
        <h4 class="text-sm font-medium text-ink">{{ t('site.sqPerfCurveTitle') }}</h4>
        <p class="mt-1 text-2xs text-faint">{{ t('site.sqPerfCurveCaption', { minutes: performance.bucket_minutes, zone: timeZone }) }}</p>
      </div>
      <div class="w-full sm:w-44">
        <label :for="groupInputId" class="mb-1 block text-2xs text-muted">{{ t('site.sqColGroup') }}</label>
        <UiSelect :id="groupInputId" v-model="group" :options="groupOptions" />
      </div>
    </div>

    <MarketplaceAvailabilityHistory v-model:index="activeIndex" :points="points" :bucket-minutes="performance.bucket_minutes" />

    <UiTabs v-model="metric" :items="metrics" />

    <UiEmptyState
      v-if="!points.length"
      :title="t('site.sqPerfCurveEmptyTitle')"
      :description="t('site.sqPerfCurveEmptyBody')"
    />
    <template v-else>
      <div class="flex min-h-14 flex-wrap items-end justify-between gap-2">
        <div>
          <p class="text-2xs text-muted">{{ metricLabel }}</p>
          <p class="numeric mt-1 text-xl font-semibold text-ink">
            {{ formatValue(active?.value ?? null) }}
            <span v-if="metric === 'tps'" class="text-2xs font-normal text-faint">{{ t('site.sqPerfRateUnit') }}</span>
          </p>
        </div>
        <div class="text-right text-2xs text-muted">
          <p class="numeric">{{ intervalLabel }}</p>
          <p class="mt-1">{{ t('site.sqPerfSampleCount', { count: formatNumber(active?.point.requests ?? 0) }) }}</p>
        </div>
      </div>

      <figure class="m-0">
        <div
          role="slider"
          tabindex="0"
          :aria-label="t('site.sqPerfInspect', { metric: metricLabel })"
          :aria-valuemin="0"
          :aria-valuemax="points.length - 1"
          :aria-valuenow="activeIndex"
          :aria-valuetext="activeLabel"
          class="rounded-control focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-clay"
          @keydown="stepPoint"
        >
          <svg
            ref="chart"
            :viewBox="`0 0 ${WIDTH} ${HEIGHT}`"
            class="block h-auto w-full overflow-visible"
            aria-hidden="true"
            @pointermove="inspectPoint"
            @pointerdown="inspectPoint"
          >
            <g v-for="tick in ticks" :key="tick.y">
              <line :x1="LEFT" :x2="WIDTH - RIGHT" :y1="tick.y" :y2="tick.y" stroke="var(--line)" stroke-dasharray="3 5" />
              <text :x="LEFT - 9" :y="tick.y + 4" text-anchor="end" fill="var(--faint)" font-size="11" class="numeric">{{ tick.label }}</text>
            </g>
            <path :d="path" fill="none" stroke="var(--clay)" stroke-width="2" stroke-linejoin="round" stroke-linecap="round" />
            <template v-for="(node, index) in nodes" :key="node.point.timestamp">
              <circle v-if="node.y != null" :cx="node.x" :cy="node.y" :r="index === activeIndex ? 4 : 2.5" fill="var(--clay)" stroke="var(--surface)" stroke-width="1.5" />
            </template>
            <line v-if="active" :x1="active.x" :x2="active.x" :y1="TOP" :y2="HEIGHT - BOTTOM" stroke="var(--muted)" stroke-dasharray="3 4" opacity="0.5" />
            <text v-if="!hasValues" :x="LEFT + plotWidth / 2" :y="TOP + plotHeight / 2" text-anchor="middle" fill="var(--muted)" font-size="12">{{ t('site.sqPerfNoObservations') }}</text>
            <text
              v-for="(node, index) in axisNodes"
              :key="node.point.timestamp"
              :x="node.x"
              :y="HEIGHT - 9"
              :text-anchor="index === 0 ? 'start' : index === axisNodes.length - 1 ? 'end' : 'middle'"
              fill="var(--faint)"
              font-size="11"
              class="numeric"
            >{{ timeFormat.format(new Date(node.point.timestamp)) }}</text>
          </svg>
        </div>
        <figcaption class="mt-1 text-2xs leading-relaxed text-faint">{{ t('site.sqPerfCurveHint') }}</figcaption>
      </figure>
      <p class="text-2xs text-faint">{{ t('site.sqPerfUpdatedAt', { time: formatDateTime(performance.updated_at) }) }}</p>
    </template>
  </section>
</template>

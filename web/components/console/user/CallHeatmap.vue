<script setup lang="ts">
import { formatNumber } from '~/src/format'

export interface HeatmapPoint {
  key: string
  date: string
  label: string
  requests: number
}

const props = defineProps<{ points: HeatmapPoint[] }>()

const { t } = useI18n()

const WEEKS = 53
const DAYS = 7
const CELL = 13
const GAP = 3
const LABEL_WIDTH = 28
const LABEL_HEIGHT = 18
const WIDTH = LABEL_WIDTH + WEEKS * (CELL + GAP) - GAP
const HEIGHT = LABEL_HEIGHT + DAYS * (CELL + GAP) - GAP

const peak = computed(() => Math.max(...props.points.map(point => point.requests), 0))
const total = computed(() => props.points.reduce((sum, point) => sum + point.requests, 0))

const cells = computed(() => props.points.map((point, index) => {
  const column = Math.floor(index / DAYS)
  const day = index % DAYS
  const level = point.requests === 0 || peak.value === 0 ? 0 : Math.min(4, Math.ceil((point.requests / peak.value) * 4))
  return {
    ...point,
    x: LABEL_WIDTH + column * (CELL + GAP),
    y: LABEL_HEIGHT + day * (CELL + GAP),
    level,
    hover: t('console.heatmapTooltip', { date: point.label, requests: formatNumber(point.requests) }),
  }
}))

const monthLabels = computed(() => {
  const labels: { key: string; x: number; label: string }[] = []
  let previous = ''
  for (const cell of cells.value) {
    const month = cell.date.slice(0, 7)
    if (month !== previous) {
      const date = new Date(`${cell.date}T00:00:00`)
      labels.push({
        key: month,
        x: cell.x,
        label: Number.isNaN(date.getTime()) ? month : new Intl.DateTimeFormat(undefined, { month: 'short' }).format(date),
      })
      previous = month
    }
  }
  return labels
})

const weekdayLabels = computed(() => [
  { label: t('console.heatmapSun'), day: 0 },
  { label: t('console.heatmapMon'), day: 1 },
  { label: t('console.heatmapWed'), day: 3 },
  { label: t('console.heatmapFri'), day: 5 },
])
</script>

<template>
  <div class="min-w-0 overflow-x-auto pb-1">
    <div class="min-w-[760px]">
      <svg
        :viewBox="`0 0 ${WIDTH} ${HEIGHT}`"
        class="block h-auto w-full"
        role="img"
        :aria-label="t('console.heatmapAria', { requests: formatNumber(total) })"
      >
        <g fill="var(--faint)" font-family="var(--font-sans)" font-size="10">
          <text v-for="weekday in weekdayLabels" :key="weekday.day" :x="0" :y="LABEL_HEIGHT + weekday.day * (CELL + GAP) + CELL - 2">
            {{ weekday.label }}
          </text>
          <text v-for="month in monthLabels" :key="month.key" :x="month.x" y="10">
            {{ month.label }}
          </text>
        </g>

        <g v-for="cell in cells" :key="cell.key" class="group">
          <rect
            :x="cell.x"
            :y="cell.y"
            :width="CELL"
            :height="CELL"
            rx="3"
            :class="`heatmap-level-${cell.level}`"
            class="transition-opacity duration-150 ease-out group-hover:opacity-70"
          >
            <title>{{ cell.hover }}</title>
          </rect>
        </g>
      </svg>

      <div class="mt-2 flex items-center justify-end gap-2 text-2xs text-faint">
        <span>{{ t('console.heatmapLess') }}</span>
        <span v-for="level in 5" :key="level" class="size-3 rounded-[3px]" :class="`heatmap-level-${level - 1}`" />
        <span>{{ t('console.heatmapMore') }}</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.heatmap-level-0 { fill: var(--sunken); }
.heatmap-level-1 { fill: color-mix(in srgb, var(--clay) 24%, var(--sunken)); }
.heatmap-level-2 { fill: color-mix(in srgb, var(--clay) 45%, var(--sunken)); }
.heatmap-level-3 { fill: color-mix(in srgb, var(--clay) 70%, var(--sunken)); }
.heatmap-level-4 { fill: var(--clay); }
</style>

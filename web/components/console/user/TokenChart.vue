<script setup lang="ts">
import { formatCompact, formatNumber } from '~/src/format'

export interface TokenPoint {
  /** Stable bucket key (local ISO day). */
  key: string
  /** Short axis caption, e.g. "7/26". */
  label: string
  value: number
}

const props = defineProps<{ points: TokenPoint[] }>()

const { t } = useI18n()

const WIDTH = 720
const HEIGHT = 168
const PAD_TOP = 22
const PAD_BOTTOM = 22
const PLOT = HEIGHT - PAD_TOP - PAD_BOTTOM
const BASELINE = PAD_TOP + PLOT
const CORNER = 4

const peak = computed(() => Math.max(...props.points.map(point => point.value), 0))
const slot = computed(() => (props.points.length ? WIDTH / props.points.length : WIDTH))
/** 60% of the slot keeps a surface gap of at least 2px between adjacent bars. */
const barWidth = computed(() => Math.max(3, slot.value * 0.6))

/** Rounds only the data end so the bar stays anchored to the baseline. */
function barPath(x: number, width: number, height: number): string {
  const radius = Math.min(CORNER, width / 2, height)
  const top = BASELINE - height
  return [
    `M${x},${BASELINE}`,
    `L${x},${top + radius}`,
    `Q${x},${top} ${x + radius},${top}`,
    `L${x + width - radius},${top}`,
    `Q${x + width},${top} ${x + width},${top + radius}`,
    `L${x + width},${BASELINE}`,
    'Z',
  ].join(' ')
}

const bars = computed(() => props.points.map((point, index) => {
  const height = peak.value > 0 ? Math.max((point.value / peak.value) * PLOT, point.value > 0 ? 2 : 0) : 0
  const x = index * slot.value + (slot.value - barWidth.value) / 2
  return {
    ...point,
    x,
    center: x + barWidth.value / 2,
    height,
    path: height > 0 ? barPath(x, barWidth.value, height) : '',
    hover: t('console.chartBarLabel', { date: point.label, tokens: formatNumber(point.value) }),
  }
}))

const peakBar = computed(() => bars.value.reduce<(typeof bars.value)[number] | null>(
  (best, bar) => (bar.value > 0 && (!best || bar.value > best.value) ? bar : best),
  null,
))

const peakLabelX = computed(() => {
  if (!peakBar.value) return 0
  return Math.min(Math.max(peakBar.value.center, 24), WIDTH - 24)
})
</script>

<template>
  <figure class="m-0">
    <svg
      :viewBox="`0 0 ${WIDTH} ${HEIGHT}`"
      class="block h-auto w-full"
      role="img"
      :aria-label="t('console.dailyTokensHint')"
    >
      <line
        :x1="0"
        :y1="BASELINE"
        :x2="WIDTH"
        :y2="BASELINE"
        stroke="var(--line)"
        stroke-width="1"
      />

      <g v-for="bar in bars" :key="bar.key" class="group">
        <path
          v-if="bar.path"
          :d="bar.path"
          fill="var(--clay)"
          class="transition-opacity duration-150 ease-out group-hover:opacity-70"
        />
        <rect
          :x="bar.x - 2"
          :y="PAD_TOP"
          :width="barWidth + 4"
          :height="PLOT"
          fill="transparent"
        >
          <title>{{ bar.hover }}</title>
        </rect>
      </g>

      <text
        v-if="peakBar"
        :x="peakLabelX"
        :y="PAD_TOP - 8"
        text-anchor="middle"
        fill="var(--muted)"
        font-family="var(--font-mono)"
        font-size="11"
      >{{ formatCompact(peakBar.value) }}</text>

      <text
        v-if="bars.length"
        :x="0"
        :y="HEIGHT - 6"
        text-anchor="start"
        fill="var(--faint)"
        font-family="var(--font-mono)"
        font-size="11"
      >{{ bars[0].label }}</text>

      <text
        v-if="bars.length > 1"
        :x="WIDTH"
        :y="HEIGHT - 6"
        text-anchor="end"
        fill="var(--faint)"
        font-family="var(--font-mono)"
        font-size="11"
      >{{ bars[bars.length - 1].label }}</text>
    </svg>
  </figure>
</template>

<script setup lang="ts">
import type { ModelPerformancePoint } from '~/src/api'
import { formatDateTime, formatNumber } from '~/src/format'
import { formatSuccessRate } from '~/src/marketplace'

const props = defineProps<{ points: ModelPerformancePoint[]; bucketMinutes: number }>()
const index = defineModel<number>('index', { required: true })
const { t } = useI18n()
const states = computed(() => [
  { key: 'healthy', label: t('site.sqAvailabilityHealthy'), color: 'bg-success', text: 'text-success' },
  { key: 'degraded', label: t('site.sqAvailabilityDegraded'), color: 'bg-warn', text: 'text-warn' },
  { key: 'failed', label: t('site.sqAvailabilityFailed'), color: 'bg-danger', text: 'text-danger' },
  { key: 'unknown', label: t('site.sqAvailabilityUnknown'), color: 'bg-line-strong', text: 'text-faint' },
])
const records = computed(() => props.points.map((point, position) => {
  const stateIndex = point.requests === 0 ? 3 : point.failed_requests === 0 ? 0 : point.failed_requests === point.requests ? 2 : 1
  const state = states.value[stateIndex]!
  const start = formatDateTime(point.timestamp)
  const end = formatDateTime(new Date(new Date(point.timestamp).getTime() + props.bucketMinutes * 60_000).toISOString())
  const rate = point.success_rate == null ? t('site.sqUnavailable') : formatSuccessRate(point.success_rate)
  return {
    ...point,
    position,
    state,
    start,
    end,
    rate,
    label: t('site.sqAvailabilityRecordLabel', {
      start, end, state: state.label, requests: formatNumber(point.requests), failed: formatNumber(point.failed_requests), rate,
    }),
  }
}))
const selected = computed(() => records.value[index.value])
const summary = computed(() => {
  const requests = props.points.reduce((sum, point) => sum + point.requests, 0)
  const failures = props.points.reduce((sum, point) => sum + point.failed_requests, 0)
  return requests ? formatSuccessRate((requests - failures) / requests) : t('site.sqUnavailable')
})
</script>

<template>
  <section v-if="points.length" class="space-y-3 border-y border-line py-4" :aria-label="t('site.sqAvailabilityTitle')">
    <div class="flex flex-wrap items-baseline justify-between gap-2">
      <h5 class="text-[13px] font-medium text-ink">{{ t('site.sqAvailabilityTitle') }}</h5>
      <p class="text-2xs text-muted">{{ t('site.sqAvailabilityOverall') }} <span class="numeric font-medium text-ink">{{ summary }}</span></p>
    </div>
    <p class="text-2xs leading-relaxed text-faint">{{ t('site.sqAvailabilityHint') }}</p>
    <div class="flex gap-1" :aria-label="t('site.sqAvailabilityTitle')" role="group">
      <button
        v-for="record in records"
        :key="record.timestamp"
        type="button"
        :aria-label="record.label"
        :aria-pressed="index === record.position"
        :title="record.label"
        class="h-8 min-w-0 flex-1 rounded-sm border border-transparent transition-opacity duration-150 hover:opacity-70 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-clay"
        :class="[record.state.color, index === record.position && 'outline-2 outline-offset-2 outline-ink']"
        @click="index = record.position"
      />
    </div>
    <div class="numeric flex justify-between gap-2 text-2xs text-faint">
      <span>{{ records[0]?.start }}</span>
      <span>{{ records.at(-1)?.end }}</span>
    </div>
    <div class="flex flex-wrap gap-x-3 gap-y-1.5 text-2xs text-muted">
      <span v-for="state in states" :key="state.key" class="inline-flex items-center gap-1.5">
        <span class="size-2 rounded-sm" :class="state.color" aria-hidden="true" />
        {{ state.label }}
      </span>
    </div>
    <p v-if="selected" class="min-h-10 rounded-control bg-sunken p-2.5 text-2xs leading-relaxed text-muted" aria-live="polite">{{ selected.label }}</p>
    <details>
      <summary class="w-fit cursor-pointer rounded-sm text-2xs font-medium text-clay focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-clay">{{ t('site.sqAvailabilityRecords') }}</summary>
      <UiTable dense class="mt-3">
        <thead>
          <tr>
            <th>{{ t('site.sqAvailabilityTime') }}</th>
            <th>{{ t('site.sqAvailabilityState') }}</th>
            <th class="num">{{ t('site.sqDetailPerfRequests') }}</th>
            <th class="num">{{ t('site.sqAvailabilityFailures') }}</th>
            <th class="num">{{ t('site.sqDetailPerfSuccess') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="record in [...records].reverse()" :key="record.timestamp" :class="index === record.position && 'bg-sunken'">
            <td>
              <button type="button" class="numeric whitespace-nowrap rounded-sm text-2xs text-clay focus-visible:outline-2 focus-visible:outline-clay" :aria-label="record.label" @click="index = record.position">{{ record.start }}</button>
            </td>
            <td class="whitespace-nowrap" :class="record.state.text">{{ record.state.label }}</td>
            <td class="num">{{ formatNumber(record.requests) }}</td>
            <td class="num">{{ formatNumber(record.failed_requests) }}</td>
            <td class="num">{{ record.rate }}</td>
          </tr>
        </tbody>
      </UiTable>
    </details>
  </section>
</template>

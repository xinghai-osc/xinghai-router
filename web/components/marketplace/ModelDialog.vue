<script setup lang="ts">
import { endpoints, type ModelPerformance, type ModelPerformanceGroup } from '~/src/api'
import { formatContextWindow, formatLatency, formatRatio, formatRequestRate, formatSquarePrice, formatSuccessRate, groupPrice, type SquareModel, type TokenUnit } from '~/src/marketplace'

const open = defineModel<boolean>('open', { required: true })

const props = defineProps<{ model: SquareModel | null; unit: TokenUnit }>()

const { t } = useI18n()

const rows = computed(() => {
  const model = props.model
  if (!model) return []
  return [...model.groups]
    .sort((a, b) => Number(a.multiplier) - Number(b.multiplier))
    .map(group => ({
      id: group.id,
      name: group.display_name || group.name,
      ratio: formatRatio(group.multiplier),
      input: formatSquarePrice(groupPrice(model, 'input', group), props.unit),
      output: formatSquarePrice(groupPrice(model, 'output', group), props.unit),
      cache: formatSquarePrice(groupPrice(model, 'cache', group), props.unit),
    }))
})

const unitHint = computed(() =>
  props.unit === 'K' ? t('site.sqUnitHintThousand') : t('site.sqUnitHintMillion'))

const registerTarget = computed(() => {
  const model = props.model?.model
  return model ? `/auth?mode=register&model=${encodeURIComponent(model)}` : '/auth?mode=register'
})

const performance = ref<ModelPerformance | null>(null)
const performancePending = ref(false)
const performanceError = ref('')

const performanceRows = computed<ModelPerformanceGroup[]>(() =>
  performance.value?.groups ?? [])

let performanceRequest = 0

async function loadPerformance() {
  const model = props.model
  if (!model) return
  const request = ++performanceRequest
  performancePending.value = true
  performanceError.value = ''
  try {
    const result = await endpoints.getModelPerformance(model.model)
    if (request === performanceRequest && props.model?.model === model.model) performance.value = result
  } catch (cause) {
    if (request === performanceRequest && props.model?.model === model.model) {
      performanceError.value = cause instanceof Error ? cause.message : t('common.loadFailed')
    }
  } finally {
    if (request === performanceRequest && props.model?.model === model.model) performancePending.value = false
  }
}

watch([open, () => props.model?.model], ([nextOpen]) => {
  performanceRequest += 1
  if (nextOpen) {
    performance.value = null
    loadPerformance()
  } else {
    performance.value = null
    performancePending.value = false
  }
})
</script>

<template>
  <UiSlidePanel v-model:open="open" size="lg" :description="t('site.sqDetailDescription')">
    <template #title>
      <span class="font-mono">{{ model?.model }}</span>
    </template>

    <div v-if="model" class="space-y-5">
      <div class="flex flex-wrap items-center gap-3">
        <SiteVendorMark :name="model.vendor_name" :slug="model.vendor_slug" size="lg" />
        <div class="min-w-0">
          <p class="truncate text-sm font-medium text-ink">{{ model.vendor_name }}</p>
          <p class="numeric text-2xs text-faint">
            {{ t('site.sqDetailMultiplier') }} {{ formatRatio(model.multiplier ?? 1) }}
          </p>
        </div>
        <UiBadge tone="clay" class="numeric ml-auto">{{ t('site.sqGroupCount', { count: model.groups.length }) }}</UiBadge>
      </div>

      <section class="rounded-control border border-line bg-sunken px-4 py-3.5">
        <p class="text-[13px] leading-relaxed text-muted">{{ model.description || t('site.sqDescriptionUnavailable') }}</p>
        <dl class="mt-3 grid grid-cols-2 gap-3 text-[12px]">
          <div>
            <dt class="text-2xs text-faint">{{ t('site.sqCardInputType') }}</dt>
            <dd class="mt-1 text-ink">{{ model.input_modalities?.join(', ') || t('site.sqUnavailable') }}</dd>
          </div>
          <div>
            <dt class="text-2xs text-faint">{{ t('site.sqCardOutputType') }}</dt>
            <dd class="mt-1 text-ink">{{ model.output_modalities?.join(', ') || t('site.sqUnavailable') }}</dd>
          </div>
          <div>
            <dt class="text-2xs text-faint">{{ t('site.sqCardContext') }}</dt>
            <dd class="numeric mt-1 text-ink">{{ formatContextWindow(model.context_window) }}</dd>
          </div>
        </dl>
      </section>

      <section class="space-y-3">
        <div class="flex flex-wrap items-baseline justify-between gap-2">
          <h3 class="text-[13px] font-medium text-ink">{{ t('site.sqDetailGroupsTitle') }}</h3>
          <p class="text-2xs text-faint">{{ unitHint }}</p>
        </div>

        <UiEmptyState
          v-if="!rows.length"
          :title="t('site.sqDetailNoGroups')"
          :description="t('site.sqDetailNote')"
        />

        <UiTable v-else dense>
          <thead>
            <tr>
              <th>{{ t('site.sqColGroup') }}</th>
              <th class="num">{{ t('site.sqDetailMultiplier') }}</th>
              <th class="num">{{ t('site.sqColInput') }}</th>
              <th class="num">{{ t('site.sqColOutput') }}</th>
              <th class="num">{{ t('site.sqColCache') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in rows" :key="row.id">
              <td class="text-[13px]">{{ row.name }}</td>
              <td class="num text-muted">{{ row.ratio }}</td>
              <td class="num">{{ row.input }}</td>
              <td class="num">{{ row.output }}</td>
              <td class="num text-muted">{{ row.cache }}</td>
            </tr>
          </tbody>
        </UiTable>
      </section>

      <section class="space-y-3">
        <div class="flex flex-wrap items-baseline justify-between gap-2">
          <h3 class="text-[13px] font-medium text-ink">{{ t('site.sqDetailPerfTitle') }}</h3>
          <p class="text-2xs text-faint">
            {{ t('site.sqDetailPerfWindow', { hours: performance?.window_hours ?? 24 }) }}
          </p>
        </div>

        <UiAlert
          v-if="performanceError"
          tone="danger"
          :title="t('site.sqDetailPerfErrorTitle')"
        >
          {{ performanceError }}
        </UiAlert>

        <div v-else-if="performancePending" class="rounded-control border border-line bg-sunken px-4 py-5">
          <UiSkeleton :rows="3" />
        </div>

        <UiEmptyState
          v-else-if="!performanceRows.length"
          class="rounded-control border border-line bg-sunken"
          :title="t('site.sqDetailPerfEmptyTitle')"
          :description="t('site.sqDetailPerfEmptyBody')"
        />

        <UiTable v-else dense>
          <thead>
            <tr>
              <th>{{ t('site.sqColGroup') }}</th>
              <th class="num">{{ t('site.sqDetailPerfRequests') }}</th>
              <th class="num">{{ t('site.sqDetailPerfRate') }}</th>
              <th class="num">{{ t('site.sqDetailPerfLatency') }}</th>
              <th class="num">{{ t('site.sqDetailPerfFirstToken') }}</th>
              <th class="num">{{ t('site.sqDetailPerfSuccess') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in performanceRows" :key="row.group_id">
              <td class="text-[13px]">{{ row.group_name }}</td>
              <td class="num text-muted">{{ row.requests }}</td>
              <td class="num">{{ formatRequestRate(row.tps) }}</td>
              <td class="num">{{ formatLatency(row.avg_latency_ms) }}</td>
              <td class="num text-muted">{{ row.avg_first_token_ms == null ? t('common.none') : formatLatency(row.avg_first_token_ms) }}</td>
              <td class="num text-muted">{{ formatSuccessRate(row.success_rate) }}</td>
            </tr>
          </tbody>
        </UiTable>
      </section>

      <p class="rounded-control border border-line bg-sunken px-3.5 py-2.5 text-2xs leading-relaxed text-muted">
        {{ t('site.sqDetailNote') }}
      </p>
    </div>

    <template #footer>
      <UiButton variant="secondary" @click="open = false">{{ t('common.close') }}</UiButton>
      <UiButton :to="registerTarget">{{ t('site.sqDetailCta') }}</UiButton>
    </template>
  </UiSlidePanel>
</template>

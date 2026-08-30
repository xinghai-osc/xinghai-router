<script setup lang="ts">
import { Check, Copy, FileText, Gauge, Plus } from 'lucide-vue-next'
import { useClipboard } from '@vueuse/core'
import {
  effectivePrice, formatContextWindow, formatLatency, formatRatio, formatSquarePrice, formatSuccessRate, getDisplayGroup, modelPerformance,
  type SquareModel, type TokenUnit,
} from '~/src/marketplace'

const props = defineProps<{
  model: SquareModel
  group: string
  unit: TokenUnit
  compareMode?: boolean
  compared?: boolean
}>()

const emit = defineEmits<{
  select: [model: SquareModel]
  toggleCompare: [model: SquareModel]
}>()

const { t } = useI18n()
const { copy, copied } = useClipboard({ copiedDuring: 1400 })
const performance = computed(() => modelPerformance(props.model))
const displayGroup = computed(() => getDisplayGroup(props.model, props.group))
const prices = computed(() => [
  { key: 'site.sqColInput', value: formatSquarePrice(effectivePrice(props.model, 'input', props.group), props.unit) },
  { key: 'site.sqColOutput', value: formatSquarePrice(effectivePrice(props.model, 'output', props.group), props.unit) },
  { key: 'site.sqColCache', value: formatSquarePrice(effectivePrice(props.model, 'cache', props.group), props.unit) },
])

function selectModel() {
  emit('select', props.model)
}

function isInternalControl(target: EventTarget | null) {
  return target instanceof Element && Boolean(target.closest('button, a, input, select, textarea, [role="link"]'))
}

function onCardClick(event: MouseEvent) {
  if (!isInternalControl(event.target)) selectModel()
}

function onCardKeydown(event: KeyboardEvent) {
  if (isInternalControl(event.target)) return
  if (event.key !== 'Enter' && event.key !== ' ') return
  event.preventDefault()
  selectModel()
}

function copyModel() {
  copy(props.model.model)
}
</script>

<template>
  <article
    role="button"
    tabindex="0"
    class="group flex min-h-[350px] flex-col overflow-hidden rounded-card border border-line bg-surface text-left transition-[border-color,transform,background-color] duration-150 ease-out hover:-translate-y-0.5 hover:border-line-strong hover:bg-surface focus-visible:border-clay focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-clay/30"
    :class="compared && 'border-clay ring-2 ring-clay/15'"
    :aria-label="t('site.sqOpenDetail', { model: model.model })"
    @click="onCardClick"
    @keydown="onCardKeydown"
  >
    <div class="p-5 pb-4">
      <div class="flex items-start gap-3">
        <div class="flex min-w-0 flex-1 items-start gap-3">
          <span class="transition-transform duration-150 ease-out group-hover:scale-105">
            <SiteVendorMark :name="model.vendor_name" :slug="model.vendor_slug" size="md" />
          </span>
          <span class="min-w-0">
            <span class="block truncate font-mono text-[15px] font-semibold text-ink">{{ model.model }}</span>
            <span class="mt-0.5 block truncate text-2xs text-faint">{{ model.vendor_name }}</span>
          </span>
        </div>

        <div class="flex shrink-0 items-center gap-1.5">
          <UiBadge v-if="displayGroup" tone="warn" class="numeric">{{ formatRatio(displayGroup.multiplier) }}</UiBadge>
          <button
            type="button"
            class="inline-flex size-7 items-center justify-center rounded-control text-faint transition-colors hover:bg-sunken hover:text-ink"
            :aria-label="copied ? t('common.copied') : t('site.sqCopyModel')"
            :title="copied ? t('common.copied') : t('site.sqCopyModel')"
            @click.stop="copyModel"
          >
            <Check v-if="copied" class="size-3.5 text-success" />
            <Copy v-else class="size-3.5" />
          </button>
          <button
            v-if="compareMode"
            type="button"
            class="inline-flex size-7 items-center justify-center rounded-control border transition-colors"
            :class="compared ? 'border-clay bg-clay-soft text-clay' : 'border-line text-faint hover:border-faint hover:text-ink'"
            :aria-label="t('site.sqCompareModel', { model: model.model })"
            :aria-pressed="compared"
            @click.stop="emit('toggleCompare', model)"
          >
            <Check v-if="compared" class="size-3.5" />
            <Plus v-else class="size-3.5" />
          </button>
        </div>
      </div>

      <p class="mt-4 min-h-10 line-clamp-2 text-[12px] leading-relaxed text-muted">
        {{ model.description || t('site.sqDescriptionUnavailable') }}
      </p>
    </div>

    <div class="mx-5 rounded-control bg-sunken px-3.5 py-3.5">
      <div class="grid grid-cols-2 gap-x-4 gap-y-3">
        <div>
          <p class="text-2xs text-faint">{{ t('site.sqCardInputType') }}</p>
          <div v-if="model.input_modalities?.length" class="mt-1 flex flex-wrap gap-x-2 gap-y-1">
            <span v-for="modality in model.input_modalities" :key="modality" class="text-[11px] text-ink">{{ modality }}</span>
          </div>
          <p v-else class="mt-1 flex items-center gap-1.5 text-[12px] text-ink">
            <FileText class="size-3.5 text-faint" />
            {{ t('site.sqUnavailable') }}
          </p>
        </div>
        <div>
          <p class="text-2xs text-faint">{{ t('site.sqCardOutputType') }}</p>
          <div v-if="model.output_modalities?.length" class="mt-1 flex flex-wrap gap-x-2 gap-y-1">
            <span v-for="modality in model.output_modalities" :key="modality" class="text-[11px] text-ink">{{ modality }}</span>
          </div>
          <p v-else class="mt-1 flex items-center gap-1.5 text-[12px] text-ink">
            <FileText class="size-3.5 text-faint" />
            {{ t('site.sqUnavailable') }}
          </p>
        </div>
        <div>
          <p class="text-2xs text-faint">{{ t('site.sqCardContext') }}</p>
          <p class="numeric mt-1 text-[12px] text-ink">{{ formatContextWindow(model.context_window) }}</p>
        </div>
        <div>
          <p class="text-2xs text-faint">{{ t('site.sqCardAvailability') }}</p>
          <p class="numeric mt-1 text-[12px] text-ink">{{ performance.success_rate == null ? t('site.sqUnavailable') : formatSuccessRate(performance.success_rate) }}</p>
        </div>
      </div>

      <div class="mt-3 border-t border-line pt-3">
        <div class="grid grid-cols-3 gap-2">
          <div v-for="price in prices" :key="price.key" class="min-w-0">
            <p class="truncate text-2xs text-faint">{{ t(price.key) }}</p>
            <p class="numeric mt-1 truncate text-[12px] text-ink">{{ price.value }}</p>
          </div>
        </div>
      </div>
    </div>

    <div class="mt-3 grid grid-cols-3 gap-2 px-5 text-2xs text-faint" :aria-label="t('site.sqDetailPerfTitle')">
      <div class="min-w-0">
        <p class="truncate">{{ t('site.sqCardRequests') }}</p>
        <p class="numeric mt-1 truncate text-muted"><Gauge class="mr-0.5 inline size-3" />{{ performance.requests == null ? t('site.sqUnavailable') : performance.requests }}</p>
      </div>
      <div class="min-w-0">
        <p class="truncate">{{ t('site.sqCardLatency') }}</p>
        <p class="numeric mt-1 truncate text-muted">{{ performance.avg_latency_ms == null ? t('site.sqUnavailable') : formatLatency(performance.avg_latency_ms) }}</p>
      </div>
      <div class="min-w-0">
        <p class="truncate">{{ t('site.sqCardFirstToken') }}</p>
        <p class="numeric mt-1 truncate text-muted">{{ performance.avg_first_token_ms == null ? t('site.sqUnavailable') : formatLatency(performance.avg_first_token_ms) }}</p>
      </div>
    </div>

    <footer class="mt-auto flex items-center justify-between gap-2 px-5 py-4">
      <span class="truncate text-2xs text-faint">
        {{ displayGroup?.name ?? t('common.none') }} · {{ t('site.sqGroupCount', { count: model.groups.length }) }}
      </span>
      <div class="flex shrink-0 items-center gap-1.5">
        <NuxtLink
          :to="{ path: '/auth', query: { mode: 'register', model: model.model } }"
          class="rounded-control border border-line-strong px-2.5 py-1.5 text-[12px] text-muted transition-colors hover:bg-sunken hover:text-ink"
          :aria-label="t('site.sqApiCallModel', { model: model.model })"
          @click.stop
          @keydown.stop
        >{{ t('site.sqApiCall') }}</NuxtLink>
        <NuxtLink
          :to="{ path: '/auth', query: { mode: 'register', model: model.model } }"
          class="rounded-control bg-ink px-2.5 py-1.5 text-[12px] text-paper transition-colors hover:opacity-80"
          :aria-label="t('site.sqChatModel', { model: model.model })"
          @click.stop
          @keydown.stop
        >{{ t('site.sqChat') }}</NuxtLink>
      </div>
    </footer>
  </article>
</template>

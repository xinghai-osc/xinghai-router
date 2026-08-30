<script setup lang="ts">
import { ArrowRight, Sparkles } from 'lucide-vue-next'
import type { SquareModel } from '~/src/marketplace'

const props = defineProps<{
  model?: SquareModel
  copy?: { badge?: string; title?: string; body?: string; cta?: string }
}>()
const emit = defineEmits<{ select: [model: SquareModel] }>()
const { t } = useI18n()

const copy = computed(() => ({
  badge: props.copy?.badge || t('site.sqPromoBadge'),
  title: props.copy?.title || t('site.sqPromoTitle'),
  body: props.copy?.body || t('site.sqPromoFallbackBody'),
  cta: props.copy?.cta || t('site.sqPromoCta'),
}))

function interpolate(value: string) {
  return value.replace(/\{model\}/g, props.model?.model ?? '')
}

function openModel() {
  if (props.model) emit('select', props.model)
}

function isInternalControl(target: EventTarget | null) {
  return target instanceof Element && Boolean(target.closest('button, a, input, select, textarea, [role="link"]'))
}

function onCardClick(event: MouseEvent) {
  if (!isInternalControl(event.target)) openModel()
}

function onCardKeydown(event: KeyboardEvent) {
  if (isInternalControl(event.target)) return
  if (event.key !== 'Enter' && event.key !== ' ') return
  event.preventDefault()
  openModel()
}
</script>

<template>
  <article
    role="button"
    tabindex="0"
    class="relative isolate flex min-h-64 flex-col justify-between overflow-hidden rounded-card border border-clay/30 bg-clay-soft p-5 text-left sm:p-6 2xl:col-span-2 2xl:min-h-[350px] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-clay/30"
    :aria-label="t('site.sqOpenDetail', { model: model?.model ?? t('site.sqPromoTitle') })"
    @click="onCardClick"
    @keydown="onCardKeydown"
  >
    <div class="pointer-events-none absolute -top-20 -right-16 -z-10 size-56 rounded-full border-[22px] border-clay/15" aria-hidden="true" />
    <div class="pointer-events-none absolute -right-8 bottom-8 -z-10 size-28 rounded-full bg-clay/10" aria-hidden="true" />

    <div>
      <UiBadge tone="clay" class="gap-1">
        <Sparkles class="size-3" />
        {{ copy.badge }}
      </UiBadge>
      <h3 class="mt-5 max-w-xs text-xl font-semibold tracking-tight text-ink sm:text-2xl">
        {{ interpolate(copy.title) }}
      </h3>
      <p v-if="model" class="mt-2 truncate font-mono text-[12px] text-clay">
        {{ model.model }}
      </p>
      <p class="mt-2 max-w-sm text-[13px] leading-relaxed text-muted">
        {{ interpolate(copy.body) }}
      </p>
    </div>

    <button
      type="button"
      class="mt-6 inline-flex w-fit items-center gap-1.5 rounded-control bg-clay px-3.5 py-2 text-[13px] font-medium text-clay-ink transition-colors duration-150 hover:bg-clay-hover disabled:pointer-events-none disabled:opacity-45"
      :disabled="!model"
      @click="openModel"
    >
      {{ copy.cta }}
      <ArrowRight class="size-3.5" />
    </button>
  </article>
</template>

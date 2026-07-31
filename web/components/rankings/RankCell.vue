<script setup lang="ts">
import { ArrowDownRight, ArrowUpRight, Minus, Sparkles } from 'lucide-vue-next'

const props = defineProps<{ rank: number; previousRank?: number }>()

const { t } = useI18n()

/** The API omits previous_rank when the entry was not ranked last period. */
const delta = computed(() => props.previousRank ? props.previousRank - props.rank : null)

const movement = computed(() => {
  if (delta.value === null) return { icon: Sparkles, tone: 'text-clay', label: t('site.rkRankNew') }
  if (delta.value > 0) return { icon: ArrowUpRight, tone: 'text-success', label: t('site.rkRankUp', { count: delta.value }) }
  if (delta.value < 0) return { icon: ArrowDownRight, tone: 'text-danger', label: t('site.rkRankDown', { count: -delta.value }) }
  return { icon: Minus, tone: 'text-faint', label: t('site.rkRankFlat') }
})
</script>

<template>
  <span class="flex items-center gap-2">
    <span
      :class="[
        'numeric inline-flex size-7 shrink-0 items-center justify-center rounded-[9px] text-[13px] font-medium',
        rank <= 3 ? 'bg-clay-soft text-clay' : 'bg-sunken text-muted',
      ]"
    >{{ rank }}</span>
    <UiTooltip :content="movement.label">
      <span :class="['flex shrink-0', movement.tone]">
        <component :is="movement.icon" class="size-3.5" />
        <span class="sr-only">{{ movement.label }}</span>
      </span>
    </UiTooltip>
  </span>
</template>

<script setup lang="ts">
import { TrendingDown, TrendingUp } from 'lucide-vue-next'
import type { RankingMover } from '~/src/api'
import { formatSignedPercent } from '~/src/format'

const props = defineProps<{
  rows: RankingMover[]
  direction: 'up' | 'down'
  title: string
  description: string
}>()

const { t } = useI18n()

const items = computed(() => props.rows.map(row => ({
  ...row,
  moveLabel: row.rank_delta >= 0
    ? t('site.rkRankUp', { count: row.rank_delta })
    : t('site.rkRankDown', { count: -row.rank_delta }),
  growthLabel: formatSignedPercent(row.growth_pct),
})))

const tone = computed(() => props.direction === 'up' ? 'text-success' : 'text-danger')
const icon = computed(() => props.direction === 'up' ? TrendingUp : TrendingDown)
</script>

<template>
  <UiCard :title="title" :description="description">
    <template #actions>
      <component :is="icon" :class="['size-4', tone]" />
    </template>

    <UiEmptyState v-if="!items.length" :title="t('site.rkMoversEmpty')" />

    <ul v-else class="space-y-3">
      <li v-for="item in items" :key="item.model_name" class="flex items-center gap-3">
        <SiteVendorMark :name="item.vendor" size="sm" />
        <div class="min-w-0 flex-1">
          <p class="truncate font-mono text-[13px] text-ink">{{ item.model_name }}</p>
          <p class="numeric text-2xs text-faint">{{ t('site.rkCurrentRank', { rank: item.current_rank }) }}</p>
        </div>
        <div class="shrink-0 text-right">
          <p :class="['text-[13px] font-medium', tone]">{{ item.moveLabel }}</p>
          <p class="numeric text-2xs text-faint">{{ item.growthLabel }}</p>
        </div>
      </li>
    </ul>
  </UiCard>
</template>

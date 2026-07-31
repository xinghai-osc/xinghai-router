<script setup lang="ts">
import type { ModelRanking } from '~/src/api'
import { formatCompact, formatPercent, formatSignedPercent } from '~/src/format'

const props = defineProps<{ rows: ModelRanking[] }>()

const { t } = useI18n()

// `share` arrives as a 0–1 fraction from the Go handler.
const items = computed(() => props.rows.map(row => ({
  ...row,
  tokens: formatCompact(row.total_tokens),
  shareLabel: formatPercent(row.share * 100),
  growthLabel: formatSignedPercent(row.growth_pct),
  barWidth: `${Math.min(100, Math.max(2, row.share * 100))}%`,
})))
</script>

<template>
  <UiTable>
    <thead>
      <tr>
        <th class="w-24">{{ t('site.rkColRank') }}</th>
        <th>{{ t('site.rkColModel') }}</th>
        <th>{{ t('site.rkColVendor') }}</th>
        <th class="num">{{ t('site.rkColTokens') }}</th>
        <th class="w-40">{{ t('site.rkColShare') }}</th>
        <th class="num">{{ t('site.rkColGrowth') }}</th>
      </tr>
    </thead>
    <tbody>
      <tr v-for="row in items" :key="row.model_name">
        <td><RankingsRankCell :rank="row.rank" :previous-rank="row.previous_rank" /></td>
        <td class="font-mono text-[13px]">{{ row.model_name }}</td>
        <td>
          <span class="flex items-center gap-2">
            <SiteVendorMark :name="row.vendor" size="sm" />
            <span class="truncate text-[13px] text-muted">{{ row.vendor }}</span>
          </span>
        </td>
        <td class="num">{{ row.tokens }}</td>
        <td>
          <span class="flex items-center gap-2">
            <span class="h-1.5 w-20 shrink-0 overflow-hidden rounded-full bg-sunken">
              <span class="block h-full rounded-full bg-clay" :style="{ width: row.barWidth }" />
            </span>
            <span class="numeric text-2xs text-muted">{{ row.shareLabel }}</span>
          </span>
        </td>
        <td :class="['num', row.growth_pct > 0 ? 'text-success' : row.growth_pct < 0 ? 'text-danger' : 'text-faint']">
          {{ row.growthLabel }}
        </td>
      </tr>
    </tbody>
  </UiTable>
</template>

<script setup lang="ts">
import type { UserRanking } from '~/src/api'
import { formatCompact, formatMoney, formatPercent, formatSignedPercent } from '~/src/format'

const props = defineProps<{ rows: UserRanking[] }>()

const { t } = useI18n()

const items = computed(() => props.rows.map(row => ({
  ...row,
  tokens: formatCompact(row.total_tokens),
  requestsLabel: formatCompact(row.requests),
  costLabel: formatMoney(row.total_cost),
  shareLabel: formatPercent(row.share * 100),
  growthLabel: formatSignedPercent(row.growth_pct),
})))
</script>

<template>
  <UiTable>
    <thead>
      <tr>
        <th class="w-24">{{ t('site.rkColRank') }}</th>
        <th>{{ t('site.rkColUser') }}</th>
        <th>{{ t('site.rkColTopModel') }}</th>
        <th class="num">{{ t('site.rkColRequests') }}</th>
        <th class="num">{{ t('site.rkColTokens') }}</th>
        <th class="num">{{ t('site.rkColCost') }}</th>
        <th class="num">{{ t('site.rkColShare') }}</th>
        <th class="num">{{ t('site.rkColGrowth') }}</th>
      </tr>
    </thead>
    <tbody>
      <tr v-for="row in items" :key="`${row.rank}-${row.name}`">
        <td><RankingsRankCell :rank="row.rank" /></td>
        <td class="text-[13px] font-medium">{{ row.name }}</td>
        <td class="font-mono text-[13px] text-muted">{{ row.top_model || '—' }}</td>
        <td class="num text-muted">{{ row.requestsLabel }}</td>
        <td class="num">{{ row.tokens }}</td>
        <td class="num">{{ row.costLabel }}</td>
        <td class="num text-muted">{{ row.shareLabel }}</td>
        <td :class="['num', row.growth_pct > 0 ? 'text-success' : row.growth_pct < 0 ? 'text-danger' : 'text-faint']">
          {{ row.growthLabel }}
        </td>
      </tr>
    </tbody>
  </UiTable>
</template>

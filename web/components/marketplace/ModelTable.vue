<script setup lang="ts">
import { effectivePrice, formatRatio, formatSquarePrice, getDisplayGroup, squareModelKey, type SquareModel, type TokenUnit } from '~/src/marketplace'

const props = defineProps<{ models: SquareModel[]; group: string; unit: TokenUnit }>()

const emit = defineEmits<{ select: [model: SquareModel] }>()

const { t } = useI18n()

const rows = computed(() => props.models.map((model) => {
  const display = getDisplayGroup(model, props.group)
  return {
    model,
    groupName: display?.name ?? t('common.none'),
    ratio: display ? formatRatio(display.multiplier) : '',
    input: formatSquarePrice(effectivePrice(model, 'input', props.group), props.unit),
    output: formatSquarePrice(effectivePrice(model, 'output', props.group), props.unit),
    cache: formatSquarePrice(effectivePrice(model, 'cache', props.group), props.unit),
  }
}))
</script>

<template>
  <UiTable>
    <thead>
      <tr>
        <th>{{ t('site.sqColModel') }}</th>
        <th>{{ t('site.sqColVendor') }}</th>
        <th>{{ t('site.sqColGroup') }}</th>
        <th class="num">{{ t('site.sqColInput') }}</th>
        <th class="num">{{ t('site.sqColOutput') }}</th>
        <th class="num">{{ t('site.sqColCache') }}</th>
      </tr>
    </thead>
    <tbody>
      <tr v-for="row in rows" :key="squareModelKey(row.model)">
        <td>
          <button
            type="button"
            class="rounded font-mono text-[13px] text-ink transition-colors duration-150 hover:text-clay focus-visible:outline-2 focus-visible:outline-clay"
            :aria-label="t('site.sqOpenDetail', { model: row.model.model })"
            @click="emit('select', row.model)"
          >{{ row.model.model }}</button>
        </td>
        <td>
          <span class="flex items-center gap-2">
            <SiteVendorMark :name="row.model.vendor_name" size="sm" />
            <span class="truncate text-[13px] text-muted">{{ row.model.vendor_name }}</span>
          </span>
        </td>
        <td>
          <span class="flex items-center gap-2">
            <span class="truncate text-[13px] text-muted">{{ row.groupName }}</span>
            <UiBadge v-if="row.ratio" tone="outline" class="numeric">{{ row.ratio }}</UiBadge>
          </span>
        </td>
        <td class="num">{{ row.input }}</td>
        <td class="num">{{ row.output }}</td>
        <td class="num text-muted">{{ row.cache }}</td>
      </tr>
    </tbody>
  </UiTable>
</template>

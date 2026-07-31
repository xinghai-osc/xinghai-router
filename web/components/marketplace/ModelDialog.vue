<script setup lang="ts">
import { formatRatio, formatSquarePrice, groupPrice, type SquareModel, type TokenUnit } from '~/src/marketplace'

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
      name: group.name,
      ratio: formatRatio(group.multiplier),
      input: formatSquarePrice(groupPrice(model, 'input', group), props.unit),
      output: formatSquarePrice(groupPrice(model, 'output', group), props.unit),
      cache: formatSquarePrice(groupPrice(model, 'cache', group), props.unit),
    }))
})

const unitHint = computed(() =>
  props.unit === 'K' ? t('site.sqUnitHintThousand') : t('site.sqUnitHintMillion'))
</script>

<template>
  <UiDialog v-model:open="open" size="lg" :description="t('site.sqDetailDescription')">
    <template #title>
      <span class="font-mono">{{ model?.model }}</span>
    </template>

    <div v-if="model" class="space-y-5">
      <div class="flex flex-wrap items-center gap-3">
        <SiteVendorMark :name="model.vendor_name" size="lg" />
        <div class="min-w-0">
          <p class="truncate text-sm font-medium text-ink">{{ model.vendor_name }}</p>
          <p class="numeric text-2xs text-faint">
            {{ t('site.sqDetailMultiplier') }} {{ formatRatio(model.multiplier ?? 1) }}
          </p>
        </div>
        <UiBadge tone="clay" class="numeric ml-auto">{{ t('site.sqGroupCount', { count: model.groups.length }) }}</UiBadge>
      </div>

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

      <p class="rounded-control border border-line bg-sunken px-3.5 py-2.5 text-2xs leading-relaxed text-muted">
        {{ t('site.sqDetailNote') }}
      </p>
    </div>

    <template #footer>
      <UiButton variant="secondary" @click="open = false">{{ t('common.close') }}</UiButton>
      <UiButton to="/auth?mode=register">{{ t('site.sqDetailCta') }}</UiButton>
    </template>
  </UiDialog>
</template>

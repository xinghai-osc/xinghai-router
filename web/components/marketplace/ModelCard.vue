<script setup lang="ts">
import { effectivePrice, formatRatio, formatSquarePrice, getDisplayGroup, type SquareModel, type TokenUnit } from '~/src/marketplace'

const props = defineProps<{ model: SquareModel; group: string; unit: TokenUnit }>()

const emit = defineEmits<{ select: [model: SquareModel] }>()

const { t } = useI18n()

const displayGroup = computed(() => getDisplayGroup(props.model, props.group))

const rows = computed(() => [
  { key: 'site.sqColInput', value: formatSquarePrice(effectivePrice(props.model, 'input', props.group), props.unit) },
  { key: 'site.sqColOutput', value: formatSquarePrice(effectivePrice(props.model, 'output', props.group), props.unit) },
  { key: 'site.sqColCache', value: formatSquarePrice(effectivePrice(props.model, 'cache', props.group), props.unit) },
])
</script>

<template>
  <button
    type="button"
    class="flex flex-col gap-4 rounded-card border border-line bg-surface p-5 text-left transition-colors duration-150 hover:border-line-strong focus-visible:border-clay focus-visible:outline-none"
    :aria-label="t('site.sqOpenDetail', { model: model.model })"
    @click="emit('select', model)"
  >
    <header class="flex items-start gap-3">
      <SiteVendorMark :name="model.vendor_name" />
      <div class="min-w-0 flex-1">
        <p class="truncate font-mono text-[13px] font-medium text-ink">{{ model.model }}</p>
        <p class="mt-0.5 truncate text-2xs text-faint">{{ model.vendor_name }}</p>
      </div>
      <UiBadge v-if="displayGroup" tone="outline" class="numeric shrink-0">
        {{ formatRatio(displayGroup.multiplier) }}
      </UiBadge>
    </header>

    <dl class="grid grid-cols-3 gap-2 border-t border-line pt-4">
      <div v-for="row in rows" :key="row.key" class="min-w-0 space-y-1">
        <dt class="truncate text-2xs text-faint">{{ t(row.key) }}</dt>
        <dd class="numeric truncate text-[13px] text-ink">{{ row.value }}</dd>
      </div>
    </dl>

    <footer class="flex items-center justify-between gap-2 text-2xs text-faint">
      <span class="truncate">{{ displayGroup?.name ?? t('common.none') }}</span>
      <span class="numeric shrink-0">{{ t('site.sqGroupCount', { count: model.groups.length }) }}</span>
    </footer>
  </button>
</template>

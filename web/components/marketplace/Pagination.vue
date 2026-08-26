<script setup lang="ts">
import { ChevronLeft, ChevronRight } from 'lucide-vue-next'

const page = defineModel<number>({ required: true })

const props = defineProps<{ totalPages: number; total: number }>()

const { t } = useI18n()

const canPrev = computed(() => page.value > 1)
const canNext = computed(() => page.value < props.totalPages)

const pageItems = computed<(number | 'ellipsis')[]>(() => {
  const total = props.totalPages
  const current = page.value
  if (total <= 7) return Array.from({ length: total }, (_, index) => index + 1)

  const items: (number | 'ellipsis')[] = [1]
  const start = Math.max(2, current - 1)
  const end = Math.min(total - 1, current + 1)
  if (start > 2) items.push('ellipsis')
  for (let value = start; value <= end; value += 1) items.push(value)
  if (end < total - 1) items.push('ellipsis')
  items.push(total)
  return items
})
</script>

<template>
  <nav
    v-if="totalPages > 1"
    class="flex flex-wrap items-center justify-between gap-3"
    :aria-label="t('common.page', { page })"
  >
    <p class="numeric text-2xs text-faint">{{ t('common.totalItems', { total }) }}</p>

    <div class="flex items-center gap-1.5">
      <UiButton variant="secondary" size="sm" :disabled="!canPrev" :aria-label="t('common.prev')" @click="page -= 1">
        <ChevronLeft class="size-4" />
        <span class="hidden sm:inline">{{ t('common.prev') }}</span>
      </UiButton>

      <template v-for="(item, index) in pageItems" :key="`${item}-${index}`">
        <span v-if="item === 'ellipsis'" class="px-1 text-sm text-faint" aria-hidden="true">…</span>
        <button
          v-else
          type="button"
          class="numeric size-9 rounded-control border text-[13px] transition-colors duration-150"
          :class="page === item ? 'border-clay bg-clay-soft text-clay' : 'border-line-strong bg-surface text-muted hover:border-faint hover:text-ink'"
          :aria-current="page === item ? 'page' : undefined"
          :aria-label="t('site.sqPageGoto', { page: item })"
          @click="page = item"
        >{{ item }}</button>
      </template>

      <UiButton variant="secondary" size="sm" :disabled="!canNext" :aria-label="t('common.next')" @click="page += 1">
        <span class="hidden sm:inline">{{ t('common.next') }}</span>
        <ChevronRight class="size-4" />
      </UiButton>
    </div>
  </nav>
</template>

<script setup lang="ts">
import { ChevronLeft, ChevronRight } from 'lucide-vue-next'

const page = defineModel<number>({ required: true })

const props = defineProps<{ totalPages: number; total: number }>()

const { t } = useI18n()

const canPrev = computed(() => page.value > 1)
const canNext = computed(() => page.value < props.totalPages)
</script>

<template>
  <nav
    v-if="totalPages > 1"
    class="flex flex-wrap items-center justify-between gap-3"
    :aria-label="t('common.page', { page })"
  >
    <p class="numeric text-2xs text-faint">{{ t('common.totalItems', { total }) }}</p>

    <div class="flex items-center gap-2">
      <UiButton variant="secondary" size="sm" :disabled="!canPrev" @click="page -= 1">
        <ChevronLeft class="size-4" />
        {{ t('common.prev') }}
      </UiButton>
      <span class="numeric px-1 text-[13px] text-muted">{{ t('site.sqPagePosition', { page, total: totalPages }) }}</span>
      <UiButton variant="secondary" size="sm" :disabled="!canNext" @click="page += 1">
        {{ t('common.next') }}
        <ChevronRight class="size-4" />
      </UiButton>
    </div>
  </nav>
</template>

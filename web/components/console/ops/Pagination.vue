<script setup lang="ts">
const props = withDefaults(defineProps<{
  page: number
  pageSize: string
  total: number
  pageSizeOptions?: string[]
}>(), { pageSizeOptions: () => ['20', '50', '100'] })

const emit = defineEmits<{
  'update:page': [value: number]
  'update:pageSize': [value: string]
}>()

const { t } = useI18n()

const pageSizeNumber = computed(() => Math.max(1, parseInt(props.pageSize, 10) || 1))
const totalPages = computed(() => Math.max(1, Math.ceil(props.total / pageSizeNumber.value)))

function goToPage(next: number) {
  if (next < 1 || next > totalPages.value) return
  emit('update:page', next)
}
</script>

<template>
  <div class="flex flex-wrap items-center justify-between gap-3 pt-3">
    <p class="text-[13px] text-muted">{{ t('common.totalItems', { total }) }}</p>
    <div class="flex flex-wrap items-center gap-3">
      <div class="flex items-center gap-2">
        <span class="text-xs text-muted">{{ t('admin.pageSize') }}</span>
        <div class="w-24">
          <UiSelect
            :model-value="pageSize"
            :options="pageSizeOptions.map(value => ({ value, label: value }))"
            size="sm"
            @update:model-value="emit('update:pageSize', String($event))"
          />
        </div>
      </div>
      <div class="flex items-center gap-2">
        <UiButton variant="secondary" size="sm" :disabled="page <= 1" @click="goToPage(page - 1)">
          {{ t('common.prev') }}
        </UiButton>
        <span class="numeric text-[13px] text-muted">{{ t('admin.pageOf', { page, pages: totalPages }) }}</span>
        <UiButton variant="secondary" size="sm" :disabled="page >= totalPages" @click="goToPage(page + 1)">
          {{ t('common.next') }}
        </UiButton>
      </div>
    </div>
  </div>
</template>
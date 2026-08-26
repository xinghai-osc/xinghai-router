<script setup lang="ts">
import type { Component } from 'vue'

withDefaults(defineProps<{
  pending: boolean
  error?: string
  empty?: boolean
  rows?: number
  emptyIcon?: Component
  emptyTitle: string
  emptyDescription?: string
}>(), { rows: 4 })

const { t } = useI18n()
</script>

<template>
  <div v-if="pending" class="rounded-card border border-line bg-surface/45 px-5 py-5"><UiSkeleton :rows="rows" /></div>

  <UiAlert v-else-if="error" tone="danger" :title="t('common.loadFailed')">
    {{ error }}
  </UiAlert>

  <div v-else-if="empty" class="rounded-card border border-line bg-surface/45"><UiEmptyState
    :icon="emptyIcon"
    :title="emptyTitle"
    :description="emptyDescription"
  >
    <slot name="empty-action" />
  </UiEmptyState></div>

  <slot v-else />
</template>

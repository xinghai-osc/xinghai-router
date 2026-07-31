<script setup lang="ts">
import type { Component } from 'vue'

withDefaults(defineProps<{
  pending: boolean
  error?: string
  empty?: boolean
  emptyIcon?: Component
  emptyTitle: string
  emptyDescription?: string
  rows?: number
}>(), { rows: 6 })

const { t } = useI18n()
</script>

<template>
  <div v-if="pending" class="rounded-card border border-line bg-surface px-5 py-6">
    <UiSkeleton :rows="rows" />
  </div>

  <UiAlert v-else-if="error" tone="danger" :title="t('common.loadFailed')">
    {{ error }}
  </UiAlert>

  <div v-else-if="empty" class="rounded-card border border-line bg-surface">
    <UiEmptyState :icon="emptyIcon" :title="emptyTitle" :description="emptyDescription" />
  </div>

  <slot v-else />
</template>

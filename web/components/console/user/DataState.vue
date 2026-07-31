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
  <UiSkeleton v-if="pending" :rows="rows" />

  <UiAlert v-else-if="error" tone="danger" :title="t('common.loadFailed')">
    {{ error }}
  </UiAlert>

  <UiEmptyState
    v-else-if="empty"
    :icon="emptyIcon"
    :title="emptyTitle"
    :description="emptyDescription"
  >
    <slot name="empty-action" />
  </UiEmptyState>

  <slot v-else />
</template>

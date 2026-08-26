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
  <div v-if="pending" class="rounded-card border border-line bg-surface/75 px-5 py-6 shadow-[0_1px_0_rgb(255_255_255/0.03)]">
    <UiSkeleton :rows="rows" />
  </div>

  <UiAlert v-else-if="error" tone="danger" :title="t('common.loadFailed')">
    {{ error }}
  </UiAlert>

  <div v-else-if="empty" class="rounded-card border border-line bg-surface/75 shadow-[0_1px_0_rgb(255_255_255/0.03)]">
    <UiEmptyState :icon="emptyIcon" :title="emptyTitle" :description="emptyDescription" />
  </div>

  <slot v-else />
</template>

<script setup lang="ts">
import type { Component } from 'vue'

defineProps<{
  label: string
  value: string
  hint?: string
  icon?: Component
  loading?: boolean
}>()
</script>

<template>
  <div class="rounded-card border border-line bg-surface px-5 py-4">
    <div class="flex items-start justify-between gap-2">
      <p class="text-[13px] text-muted">{{ label }}</p>
      <component :is="icon" v-if="icon" class="size-4 shrink-0 text-faint" />
    </div>

    <UiSkeleton v-if="loading" class="mt-2.5 h-7 w-24" />
    <p v-else class="numeric mt-1.5 text-2xl text-ink">{{ value }}</p>

    <p v-if="hint" class="mt-1 text-[13px] text-faint">{{ hint }}</p>
  </div>
</template>

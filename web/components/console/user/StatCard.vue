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
  <div class="rounded-card border border-line bg-surface/90 px-5 py-4 shadow-[0_1px_0_rgb(255_255_255/0.03)] transition-[border-color,transform,box-shadow] duration-150 hover:-translate-y-px hover:border-line-strong hover:shadow-pop">
    <div class="flex items-start justify-between gap-2">
      <p class="text-[13px] text-muted">{{ label }}</p>
      <div v-if="icon" class="flex size-8 items-center justify-center rounded-control bg-clay-soft text-clay">
        <component :is="icon" class="size-4 shrink-0" />
      </div>
    </div>

    <UiSkeleton v-if="loading" class="mt-2.5 h-7 w-24" />
    <p v-else class="numeric mt-1.5 text-2xl text-ink">{{ value }}</p>

    <p v-if="hint" class="mt-1 text-[13px] text-faint">{{ hint }}</p>
  </div>
</template>

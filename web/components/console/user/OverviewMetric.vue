<script setup lang="ts">
import type { Component } from 'vue'

defineProps<{
  label: string
  value: string
  hint?: string
  icon?: Component
  loading?: boolean
  featured?: boolean
}>()
</script>

<template>
  <div class="flex min-w-0 flex-col py-2" :aria-busy="loading || undefined">
    <div class="flex items-center gap-2 text-muted">
      <component :is="icon" v-if="icon" class="size-3.5 shrink-0" :stroke-width="1.5" aria-hidden="true" />
      <p class="text-xs leading-5">{{ label }}</p>
    </div>
    <UiSkeleton v-if="loading" class="mt-4 mb-3 h-9 w-24 max-w-full" />
    <p v-else class="numeric mt-4 mb-3 text-2xl leading-tight font-medium tracking-tight [overflow-wrap:anywhere] sm:text-[1.7rem] lg:text-3xl" :class="featured ? 'text-clay' : 'text-ink'" :title="value">{{ value }}</p>
    <p v-if="hint" class="mt-auto text-2xs leading-5 text-faint">{{ hint }}</p>
  </div>
</template>

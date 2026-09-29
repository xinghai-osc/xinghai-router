<script setup lang="ts">
import type { Component } from 'vue'

type Tone = 'accent' | 'success' | 'warn'

withDefaults(defineProps<{
  label: string
  value: string
  hint?: string
  icon?: Component
  loading?: boolean
  tone?: Tone
}>(), { tone: 'accent' })

const TONES: Record<Tone, string> = {
  accent: 'bg-clay-soft text-clay',
  success: 'bg-success-soft text-success',
  warn: 'bg-warn-soft text-warn',
}
</script>

<template>
  <div class="min-w-0 rounded-card border border-line bg-surface p-4 sm:p-5" :aria-busy="loading || undefined">
    <div class="flex items-start justify-between gap-2">
      <p class="min-w-0 text-xs leading-5 font-medium text-muted sm:text-[13px]">{{ label }}</p>
      <div v-if="icon" class="flex size-8 shrink-0 items-center justify-center rounded-control" :class="TONES[tone]">
        <component :is="icon" class="size-4" aria-hidden="true" />
      </div>
    </div>

    <UiSkeleton v-if="loading" class="mt-3 h-7 w-24 max-w-full" />
    <p v-else class="numeric mt-3 text-xl leading-tight font-semibold tracking-tight text-ink [overflow-wrap:anywhere] sm:text-2xl" :title="value">{{ value }}</p>

    <p v-if="hint" class="mt-2 text-xs leading-5 text-muted">{{ hint }}</p>
  </div>
</template>

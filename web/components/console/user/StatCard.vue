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

const TONE_LINES: Record<Tone, string> = {
  accent: 'via-clay/70',
  success: 'via-success/70',
  warn: 'via-warn/70',
}
</script>

<template>
  <div class="group relative flex h-full min-w-0 flex-col overflow-hidden rounded-card border border-line bg-surface p-4 transition-[border-color,transform] duration-150 ease-out hover:border-line-strong motion-safe:hover:-translate-y-0.5 sm:p-5" :aria-busy="loading || undefined">
    <span :class="['pointer-events-none absolute inset-x-6 top-0 h-px bg-gradient-to-r from-transparent to-transparent opacity-60 transition-opacity duration-150 ease-out group-hover:opacity-100', TONE_LINES[tone]]" aria-hidden="true" />
    <div class="flex items-start justify-between gap-2">
      <p class="min-w-0 text-xs leading-5 font-medium text-muted sm:text-[13px]">{{ label }}</p>
      <div v-if="icon" class="flex size-8 shrink-0 items-center justify-center rounded-control transition-transform duration-150 ease-out group-hover:scale-105 motion-reduce:transform-none" :class="TONES[tone]">
        <component :is="icon" class="size-4" aria-hidden="true" />
      </div>
    </div>

    <UiSkeleton v-if="loading" class="mt-4 mb-4 h-8 w-24 max-w-full" />
    <p v-else class="numeric mt-4 mb-4 text-2xl leading-tight font-semibold tracking-tight text-ink [overflow-wrap:anywhere] sm:text-[1.75rem]" :title="value">{{ value }}</p>

    <p v-if="hint" class="mt-auto pt-1 text-xs leading-5 text-muted">{{ hint }}</p>
  </div>
</template>

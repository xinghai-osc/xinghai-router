<script setup lang="ts">
import { cn } from '~/lib/utils'

type Tone = 'neutral' | 'clay' | 'success' | 'warn' | 'danger' | 'outline'

withDefaults(defineProps<{ tone?: Tone; dot?: boolean }>(), { tone: 'neutral' })

const TONES: Record<Tone, string> = {
  neutral: 'bg-sunken text-muted',
  clay: 'bg-clay-soft text-clay',
  success: 'bg-success-soft text-success',
  warn: 'bg-warn-soft text-warn',
  danger: 'bg-danger-soft text-danger',
  outline: 'border border-line text-muted',
}

const DOTS: Record<Tone, string> = {
  neutral: 'bg-faint',
  clay: 'bg-clay',
  success: 'bg-success',
  warn: 'bg-warn',
  danger: 'bg-danger',
  outline: 'bg-faint',
}
</script>

<template>
  <span
    :class="cn(
      'inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-2xs font-medium whitespace-nowrap',
      TONES[tone],
    )"
  >
    <span v-if="dot" :class="cn('size-1.5 rounded-full', DOTS[tone])" />
    <slot />
  </span>
</template>

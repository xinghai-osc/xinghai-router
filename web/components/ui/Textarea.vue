<script setup lang="ts">
import { cn } from '~/lib/utils'

const model = defineModel<string>({ default: '' })

withDefaults(defineProps<{
  placeholder?: string
  rows?: number
  disabled?: boolean
  invalid?: boolean
  mono?: boolean
  id?: string
}>(), { rows: 4 })
</script>

<template>
  <textarea
    :id="id"
    v-model="model"
    :rows="rows"
    :placeholder="placeholder"
    :disabled="disabled"
    :aria-invalid="invalid || undefined"
    :class="cn(
      'w-full resize-y rounded-control border border-line-strong bg-surface px-3 py-2 text-sm text-ink',
      'placeholder:text-faint transition-colors duration-150',
      'hover:border-faint focus:border-clay focus:outline-none focus:ring-2 focus:ring-clay/20',
      'disabled:cursor-not-allowed disabled:bg-sunken disabled:text-muted',
      'aria-invalid:border-danger aria-invalid:focus:ring-danger/20',
      mono && 'font-mono text-[13px] leading-relaxed',
    )"
  />
</template>

<script setup lang="ts">
import { cn } from '~/lib/utils'

const model = defineModel<string | number>({ default: '' })

withDefaults(defineProps<{
  type?: string
  placeholder?: string
  disabled?: boolean
  readonly?: boolean
  invalid?: boolean
  mono?: boolean
  autocomplete?: string
  id?: string
}>(), { type: 'text' })
</script>

<template>
  <div class="relative flex items-center">
    <span v-if="$slots.leading" class="pointer-events-none absolute left-3 flex text-faint">
      <slot name="leading" />
    </span>

    <input
      :id="id"
      v-model="model"
      :type="type"
      :placeholder="placeholder"
      :disabled="disabled"
      :readonly="readonly"
      :autocomplete="autocomplete"
      :aria-invalid="invalid || undefined"
      :class="cn(
        'h-10 w-full rounded-control border border-line-strong bg-surface px-3 text-sm text-ink',
        'placeholder:text-faint transition-colors duration-150',
        'hover:border-faint focus:border-clay focus:outline-none focus:ring-2 focus:ring-clay/20',
        'disabled:cursor-not-allowed disabled:bg-sunken disabled:text-muted',
        'aria-invalid:border-danger aria-invalid:focus:ring-danger/20',
        mono && 'font-mono text-[13px]',
        $slots.leading && 'pl-9',
        $slots.trailing && 'pr-9',
      )"
    >

    <span v-if="$slots.trailing" class="absolute right-3 flex text-faint">
      <slot name="trailing" />
    </span>
  </div>
</template>

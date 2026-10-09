<script setup lang="ts">
import { computed, inject } from 'vue'
import { cn } from '~/lib/utils'
import { uiFieldContextKey } from './field-context'

const model = defineModel<string>({ default: '' })

const props = withDefaults(defineProps<{
  placeholder?: string
  rows?: number
  disabled?: boolean
  invalid?: boolean
  mono?: boolean
  id?: string
}>(), { rows: 4 })

const field = inject(uiFieldContextKey, null)
const textareaId = computed(() => props.id ?? field?.id.value)
const describedBy = computed(() => field?.describedBy.value)
const invalid = computed(() => props.invalid || field?.invalid.value)
</script>

<template>
  <textarea
    :id="textareaId"
    v-model="model"
    :rows="rows"
    :placeholder="placeholder"
    :disabled="disabled"
    :aria-describedby="describedBy"
    :aria-invalid="invalid || undefined"
    :aria-required="field?.required.value || undefined"
    :required="field?.required.value || undefined"
    :class="cn(
      'block min-h-24 w-full resize-y rounded-control border border-line-strong bg-surface px-3 py-2.5 text-sm leading-6 text-ink',
      'placeholder:text-faint transition-colors duration-150 ease-out motion-reduce:transition-none',
      'enabled:hover:border-faint focus:border-clay focus:outline-2 focus:outline-offset-2 focus:outline-clay/60',
      'disabled:cursor-not-allowed disabled:bg-sunken disabled:text-muted',
      'aria-invalid:border-danger aria-invalid:focus:border-danger aria-invalid:focus:outline-danger/60',
      mono && 'font-mono text-[13px] leading-relaxed',
    )"
  />
</template>

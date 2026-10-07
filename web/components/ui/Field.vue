<script setup lang="ts">
import { computed, provide, useId } from 'vue'
import { uiFieldContextKey } from './field-context'

const props = defineProps<{
  label?: string
  hint?: string
  error?: string
  required?: boolean
  for?: string
}>()

const generatedId = useId()
const fieldId = computed(() => props.for || `field-${generatedId}`)
const hintId = computed(() => props.hint && !props.error ? `${fieldId.value}-hint` : undefined)
const errorId = computed(() => props.error ? `${fieldId.value}-error` : undefined)
const describedBy = computed(() => [hintId.value, errorId.value].filter(Boolean).join(' ') || undefined)
const invalid = computed(() => Boolean(props.error))
const required = computed(() => Boolean(props.required))

provide(uiFieldContextKey, { id: fieldId, describedBy, invalid, required })
</script>

<template>
  <div class="space-y-1.5">
    <label v-if="label" :for="fieldId" class="flex items-center gap-1 text-[13px] font-medium text-ink">
      {{ label }}
      <span v-if="required" class="text-danger">*</span>
    </label>

    <slot />

    <p v-if="error" :id="errorId" class="text-[13px] text-danger">{{ error }}</p>
    <p v-else-if="hint" :id="hintId" class="text-[13px] text-muted">{{ hint }}</p>
  </div>
</template>

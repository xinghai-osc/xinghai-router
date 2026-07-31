<script setup lang="ts">
defineProps<{
  label?: string
  hint?: string
  error?: string
  required?: boolean
  for?: string
}>()
</script>

<template>
  <div class="space-y-1.5">
    <label v-if="label" :for="$props.for" class="flex items-center gap-1 text-[13px] font-medium text-ink">
      {{ label }}
      <span v-if="required" class="text-danger">*</span>
    </label>

    <slot />

    <p v-if="error" class="text-[13px] text-danger">{{ error }}</p>
    <p v-else-if="hint" class="text-[13px] text-muted">{{ hint }}</p>
  </div>
</template>

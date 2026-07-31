<script setup lang="ts">
import { Check } from 'lucide-vue-next'
import { CheckboxIndicator, CheckboxRoot } from 'reka-ui'

const model = defineModel<boolean>({ default: false })

defineProps<{ disabled?: boolean; id?: string; label?: string }>()
</script>

<template>
  <label :class="['inline-flex items-center gap-2', disabled ? 'cursor-not-allowed opacity-45' : 'cursor-pointer']">
    <CheckboxRoot
      :id="id"
      v-model="model"
      :disabled="disabled"
      class="flex size-4 shrink-0 items-center justify-center rounded-[5px] border border-line-strong bg-surface transition-colors duration-150 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-clay data-[state=checked]:border-clay data-[state=checked]:bg-clay"
    >
      <CheckboxIndicator class="flex text-clay-ink">
        <Check class="size-3" stroke-width="3" />
      </CheckboxIndicator>
    </CheckboxRoot>
    <span v-if="label || $slots.default" class="text-sm text-ink select-none">
      <slot>{{ label }}</slot>
    </span>
  </label>
</template>

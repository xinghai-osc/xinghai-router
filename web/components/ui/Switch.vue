<script setup lang="ts">
import { SwitchRoot, SwitchThumb } from 'reka-ui'

const model = defineModel<boolean>({ default: false })

defineProps<{ disabled?: boolean; id?: string; label?: string }>()
</script>

<template>
  <SwitchRoot
    :id="id"
    v-model="model"
    :disabled="disabled"
    :aria-label="label"
    class="inline-flex h-[22px] w-[38px] shrink-0 cursor-pointer items-center rounded-full border border-transparent bg-line-strong p-0.5 transition-colors duration-150 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-clay disabled:cursor-not-allowed disabled:opacity-45 data-[state=checked]:bg-clay"
  >
    <!-- The thumb sits on clay or line-strong in every theme, so plain white is
         the one colour that stays legible; it is not a missing token. -->
    <SwitchThumb
      class="pointer-events-none block size-[16px] rounded-full bg-white shadow-sm transition-transform duration-150 ease-out will-change-transform data-[state=checked]:translate-x-4"
    />
  </SwitchRoot>
</template>

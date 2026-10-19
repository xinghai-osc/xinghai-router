<script setup lang="ts">
import { vendorInitial } from '~/src/marketplace'
import { vendorIcon } from '~/src/vendor-icons'

const props = withDefaults(defineProps<{ name: string; slug?: string; size?: 'sm' | 'md' | 'lg' }>(), { size: 'md' })

const SIZES = { sm: 'size-6', md: 'size-8', lg: 'size-10' }
const ICON_SIZES = { sm: 'size-4', md: 'size-5', lg: 'size-6' }
const MONO_SIZES = { sm: 'text-[10px]', md: 'text-[13px]', lg: 'text-sm' }
const initial = computed(() => vendorInitial(props.name))
const icon = computed(() => vendorIcon(props.name, props.slug))
const iconMask = computed(() => icon.value ? `url("${icon.value}")` : undefined)
</script>

<template>
  <span
    :class="['inline-flex shrink-0 items-center justify-center rounded-[9px] bg-clay-soft font-semibold text-clay', SIZES[size], MONO_SIZES[size]]"
    :title="name"
    aria-hidden="true"
  >
    <span
      v-if="icon"
      :class="['vendor-icon bg-current', ICON_SIZES[size]]"
      :style="{ maskImage: iconMask, WebkitMaskImage: iconMask }"
    />
    <template v-else>{{ initial }}</template>
  </span>
</template>

<style scoped>
.vendor-icon {
  mask-position: center;
  mask-repeat: no-repeat;
  mask-size: contain;
  -webkit-mask-position: center;
  -webkit-mask-repeat: no-repeat;
  -webkit-mask-size: contain;
}
</style>

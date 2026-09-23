<script setup lang="ts">
import { vendorInitial } from '~/src/marketplace'

const props = withDefaults(defineProps<{ name: string; slug?: string; size?: 'sm' | 'md' | 'lg' }>(), { size: 'md' })

const SIZES = { sm: 'size-6', md: 'size-8', lg: 'size-10' }
const MONO_SIZES = { sm: 'text-[10px]', md: 'text-[13px]', lg: 'text-sm' }
const initial = computed(() => vendorInitial(props.name))
</script>

<template>
  <span
    :class="['inline-flex shrink-0 items-center justify-center rounded-[9px] bg-clay-soft font-semibold text-clay', SIZES[size], MONO_SIZES[size]]"
    :title="name"
    aria-hidden="true"
  >{{ initial }}</span>
</template>

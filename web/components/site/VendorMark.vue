<script setup lang="ts">
import { getLobeIconCDN } from '@lobehub/icons/es/features/getLobeIconCDN'
import { ModelProvider } from '@lobehub/icons/es/features/providerEnum'
import { vendorInitial } from '~/src/marketplace'

const props = withDefaults(defineProps<{ name: string; slug?: string; size?: 'sm' | 'md' | 'lg' }>(), { size: 'md' })

const SIZES = { sm: 'size-6', md: 'size-8', lg: 'size-10' }
const LOGO_SIZES = { sm: 'size-3.5', md: 'size-5', lg: 'size-6' }
const MONO_SIZES = { sm: 'text-[10px]', md: 'text-[13px]', lg: 'text-sm' }

const validIds = new Set(Object.values(ModelProvider))

const initial = computed(() => vendorInitial(props.name))
const logoUrl = computed(() => {
  const id = (props.slug ?? props.name).trim().toLowerCase()
  if (!validIds.has(id)) return null
  return getLobeIconCDN(id, { format: 'svg', type: 'mono', cdn: 'unpkg' })
})
const logoStyle = computed(() => logoUrl.value
  ? {
      maskImage: `url("${logoUrl.value}")`,
      WebkitMaskImage: `url("${logoUrl.value}")`,
      maskRepeat: 'no-repeat',
      WebkitMaskRepeat: 'no-repeat',
      maskPosition: 'center',
      WebkitMaskPosition: 'center',
      maskSize: 'contain',
      WebkitMaskSize: 'contain',
    }
  : undefined)
</script>

<template>
  <span
    :class="['inline-flex shrink-0 items-center justify-center rounded-[9px] bg-clay-soft text-clay', SIZES[size]]"
    :title="name"
  >
    <span
      v-if="logoUrl"
      :style="logoStyle"
      :class="['bg-clay', LOGO_SIZES[size]]"
      aria-hidden="true"
    />
    <span
      v-else
      :class="['font-semibold text-clay', MONO_SIZES[size]]"
      aria-hidden="true"
    >{{ initial }}</span>
  </span>
</template>

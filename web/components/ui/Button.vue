<script setup lang="ts">
import { Loader2 } from 'lucide-vue-next'
import { cn } from '~/lib/utils'

type Variant = 'primary' | 'secondary' | 'ghost' | 'danger' | 'link'
type Size = 'sm' | 'md' | 'lg' | 'icon'

const props = withDefaults(defineProps<{
  variant?: Variant
  size?: Size
  loading?: boolean
  disabled?: boolean
  type?: 'button' | 'submit' | 'reset'
  to?: string
  href?: string
  block?: boolean
}>(), { variant: 'primary', size: 'md', type: 'button' })

const VARIANTS: Record<Variant, string> = {
  primary: 'bg-clay text-clay-ink hover:bg-clay-hover active:translate-y-px',
  secondary: 'border border-line-strong bg-surface text-ink hover:bg-sunken active:translate-y-px',
  ghost: 'text-muted hover:bg-sunken hover:text-ink',
  danger: 'bg-danger text-white hover:opacity-90 active:translate-y-px',
  link: 'text-clay underline-offset-4 hover:underline',
}

const SIZES: Record<Size, string> = {
  sm: 'h-8 gap-1.5 px-3 text-[13px]',
  md: 'h-10 gap-2 px-4 text-sm',
  lg: 'h-12 gap-2 px-6 text-[15px]',
  icon: 'size-9 justify-center',
}

const tag = computed(() => (props.to ? resolveComponent('NuxtLink') : props.href ? 'a' : 'button'))
const isDisabled = computed(() => props.disabled || props.loading)
</script>

<template>
  <component
    :is="tag"
    :to="to"
    :href="href"
    :type="to || href ? undefined : type"
    :disabled="tag === 'button' ? isDisabled : undefined"
    :aria-busy="loading || undefined"
    :aria-disabled="isDisabled || undefined"
    :class="cn(
      'inline-flex shrink-0 items-center rounded-control font-medium whitespace-nowrap transition-[background-color,color,opacity,transform] duration-150 ease-out',
      'disabled:pointer-events-none disabled:opacity-45 aria-disabled:pointer-events-none aria-disabled:opacity-45',
      VARIANTS[variant],
      SIZES[size],
      block && 'w-full justify-center',
    )"
  >
    <Loader2 v-if="loading" class="size-4 animate-spin" />
    <slot />
  </component>
</template>

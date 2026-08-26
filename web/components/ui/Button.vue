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
  secondary: 'border border-line-strong bg-surface/60 text-ink backdrop-blur-md hover:bg-sunken active:translate-y-px',
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
      'liquid-button relative isolate inline-flex shrink-0 items-center overflow-hidden rounded-button font-medium whitespace-nowrap transition-[background-color,color,opacity,transform] duration-150 ease-out',
      'disabled:pointer-events-none disabled:opacity-45 aria-disabled:pointer-events-none aria-disabled:opacity-45',
      VARIANTS[variant],
      SIZES[size],
      block && 'w-full justify-center',
    )"
  >
    <span class="relative z-10 inline-flex items-center gap-2">
      <Loader2 v-if="loading" class="size-4 animate-spin" />
      <slot />
    </span>
  </component>
</template>

<style scoped>
.liquid-button::after {
  content: '';
  position: absolute;
  z-index: 0;
  top: 50%;
  left: 50%;
  width: 130%;
  aspect-ratio: 1;
  border-radius: 999px;
  background: currentColor;
  opacity: 0;
  pointer-events: none;
  transform: translate(-50%, -50%) scale(0);
  transition: opacity 180ms ease-out, transform 360ms ease-out;
}

.liquid-button:hover::after {
  opacity: 0.08;
  transform: translate(-50%, -50%) scale(1);
}

.liquid-button:disabled::after,
.liquid-button[aria-disabled='true']::after {
  display: none;
}
</style>

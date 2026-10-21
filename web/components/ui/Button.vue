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
  primary: 'glass-control border border-clay/70 text-clay-ink hover:bg-clay-hover active:translate-y-px',
  secondary: 'glass-control border border-line-strong text-ink hover:bg-sunken active:translate-y-px',
  ghost: 'glass-control border border-line/50 text-muted hover:bg-sunken hover:text-ink',
  danger: 'glass-control border border-danger/70 text-white hover:opacity-90 active:translate-y-px',
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
      `ui-button ui-button-${variant}`,
       'relative isolate inline-flex shrink-0 items-center overflow-hidden rounded-full font-medium whitespace-nowrap transition-[background-color,color,opacity,transform] duration-150 ease-out',
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
.ui-button-primary,
.ui-button-danger {
  --glass-highlight: color-mix(in srgb, var(--shimmer-highlight) 20%, transparent);
}

.ui-button-primary {
  --glass-fill: var(--clay);
  --glass-solid: var(--clay);
  --glass-opacity: 94%;
}

.ui-button-danger {
  --glass-fill: var(--danger);
  --glass-solid: var(--danger);
  --glass-opacity: 94%;
}

.ui-button::after {
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
  transition: opacity 150ms ease-out, transform 150ms ease-out;
}

.ui-button:hover::after {
  opacity: 0.08;
  transform: translate(-50%, -50%) scale(1);
}

.ui-button:disabled::after,
.ui-button[aria-disabled='true']::after {
  display: none;
}
</style>

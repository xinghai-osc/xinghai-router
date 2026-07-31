<script setup lang="ts">
import { cn } from '~/lib/utils'

withDefaults(defineProps<{
  title?: string
  description?: string
  flush?: boolean
  interactive?: boolean
}>(), {})
</script>

<template>
  <section
    :class="cn(
      'rounded-card border border-line bg-surface',
      interactive && 'transition-colors duration-150 hover:border-line-strong',
    )"
  >
    <header
      v-if="title || $slots.title || $slots.actions"
      class="flex flex-wrap items-start justify-between gap-3 border-b border-line px-5 py-4"
    >
      <div class="min-w-0">
        <h2 class="text-[15px] font-semibold text-ink">
          <slot name="title">{{ title }}</slot>
        </h2>
        <p v-if="description || $slots.description" class="mt-0.5 text-[13px] text-muted">
          <slot name="description">{{ description }}</slot>
        </p>
      </div>
      <div v-if="$slots.actions" class="flex shrink-0 items-center gap-2">
        <slot name="actions" />
      </div>
    </header>

    <div :class="cn(!flush && 'px-5 py-4')">
      <slot />
    </div>

    <footer v-if="$slots.footer" class="border-t border-line px-5 py-3">
      <slot name="footer" />
    </footer>
  </section>
</template>

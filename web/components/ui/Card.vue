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
      'min-w-0 rounded-card border border-line bg-surface text-ink',
      interactive && 'transition-colors duration-150 ease-out hover:border-line-strong hover:bg-sunken/30 focus-within:border-clay motion-reduce:transition-none',
    )"
  >
    <header
      v-if="title || $slots.title || description || $slots.description || $slots.actions"
      class="flex flex-wrap items-start justify-between gap-x-6 gap-y-3 rounded-t-[inherit] border-b border-line bg-sunken/30 px-4 py-4 sm:px-6 sm:py-5"
    >
      <div v-if="title || $slots.title || description || $slots.description" class="min-w-0 flex-1 basis-48">
        <h2 v-if="title || $slots.title" class="text-[15px] leading-6 font-semibold tracking-tight text-ink">
          <slot name="title">{{ title }}</slot>
        </h2>
        <p v-if="description || $slots.description" :class="cn('text-[13px] leading-6 text-muted', (title || $slots.title) && 'mt-1')">
          <slot name="description">{{ description }}</slot>
        </p>
      </div>
      <div v-if="$slots.actions" class="flex max-w-full flex-wrap items-center gap-2">
        <slot name="actions" />
      </div>
    </header>

    <div :class="cn('min-w-0', !flush && 'p-4 sm:p-6')">
      <slot />
    </div>

    <footer v-if="$slots.footer" class="rounded-b-[inherit] border-t border-line bg-sunken/30 px-4 py-4 sm:px-6">
      <slot name="footer" />
    </footer>
  </section>
</template>

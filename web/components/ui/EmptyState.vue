<script setup lang="ts">
import { Inbox } from 'lucide-vue-next'
import type { Component } from 'vue'

defineProps<{
  icon?: Component
  title: string
  description?: string
}>()
</script>

<template>
  <div class="flex min-w-0 flex-col items-center justify-center gap-4 px-4 py-12 text-center sm:px-6 sm:py-16">
    <div class="flex size-12 shrink-0 items-center justify-center rounded-card border border-line bg-sunken text-muted" aria-hidden="true">
      <component :is="icon ?? Inbox" class="size-5" />
    </div>
    <div class="w-full max-w-sm space-y-1.5 break-words">
      <p class="text-sm leading-6 font-semibold text-ink">{{ title }}</p>
      <p v-if="description" class="text-[13px] leading-6 text-muted">{{ description }}</p>
    </div>
    <div v-if="$slots.default" class="flex max-w-full flex-wrap items-center justify-center gap-2 pt-1">
      <slot />
    </div>
  </div>
</template>

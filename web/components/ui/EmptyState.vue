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
  <div class="flex flex-col items-center justify-center gap-3 px-6 py-14 text-center">
    <div class="flex size-12 items-center justify-center rounded-full bg-clay-soft text-clay">
      <component :is="icon ?? Inbox" class="size-5" />
    </div>
    <div class="space-y-1">
      <p class="text-sm font-medium text-ink">{{ title }}</p>
      <p v-if="description" class="max-w-sm text-[13px] text-muted">{{ description }}</p>
    </div>
    <div v-if="$slots.default" class="pt-1">
      <slot />
    </div>
  </div>
</template>

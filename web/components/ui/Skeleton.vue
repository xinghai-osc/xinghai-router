<script setup lang="ts">
import { cn } from '~/lib/utils'

withDefaults(defineProps<{ rows?: number; class?: string }>(), { rows: 1 })
</script>

<template>
  <div class="space-y-2">
    <div
      v-for="row in rows"
      :key="row"
      :class="cn('h-4 overflow-hidden rounded-[6px] bg-sunken', $props.class)"
      :style="rows > 1 && row === rows ? { width: '60%' } : undefined"
    >
      <div class="skeleton-shimmer h-full w-full" />
    </div>
  </div>
</template>

<style scoped>
.skeleton-shimmer {
  background: linear-gradient(
    90deg,
    transparent 0%,
    var(--shimmer-highlight, rgb(255 255 255 / 0.45)) 50%,
    transparent 100%
  );
  animation: skeleton-shimmer 1.6s ease-in-out infinite;
}

@keyframes skeleton-shimmer {
  0% {
    transform: translateX(-100%);
  }
  100% {
    transform: translateX(100%);
  }
}
</style>

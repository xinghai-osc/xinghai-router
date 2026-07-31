<script setup lang="ts">
import { cn } from '~/lib/utils'

withDefaults(defineProps<{ rows?: number; class?: string }>(), { rows: 1 })
</script>

<template>
  <div class="space-y-2">
    <div
      v-for="row in rows"
      :key="row"
      :class="cn('h-4 animate-pulse rounded-[6px] bg-sunken', $props.class)"
      :style="rows > 1 && row === rows ? { width: '60%' } : undefined"
    />
  </div>
</template>

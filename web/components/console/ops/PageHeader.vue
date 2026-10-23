<script setup lang="ts">
defineProps<{ lead?: string }>()
</script>

<template>
  <div class="flex min-w-0 flex-col items-start justify-between gap-4 border-b border-line pb-5 sm:flex-row sm:items-start">
    <p v-if="lead" class="min-w-0 max-w-3xl text-sm leading-7 text-muted [overflow-wrap:anywhere]">{{ lead }}</p>
    <div v-if="$slots.actions" class="flex w-full flex-wrap items-center gap-2 sm:ml-auto sm:w-auto sm:max-w-[55%] sm:justify-end">
      <slot name="actions" />
    </div>
  </div>
</template>

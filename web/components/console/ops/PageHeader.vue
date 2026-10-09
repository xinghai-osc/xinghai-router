<script setup lang="ts">
defineProps<{ lead?: string }>()
</script>

<template>
  <div class="flex flex-col items-start justify-between gap-4 sm:flex-row sm:items-center rounded-card border border-line bg-surface px-4 py-4 sm:px-5">
    <p v-if="lead" class="max-w-2xl text-sm leading-6 text-muted">{{ lead }}</p>
    <div v-if="$slots.actions" class="flex w-full flex-wrap items-center gap-2 sm:ml-auto sm:w-auto sm:shrink-0">
      <slot name="actions" />
    </div>
  </div>
</template>

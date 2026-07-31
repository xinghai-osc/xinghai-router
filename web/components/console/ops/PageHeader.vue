<script setup lang="ts">
defineProps<{ lead?: string }>()
</script>

<template>
  <div class="flex flex-wrap items-center justify-between gap-3">
    <p v-if="lead" class="max-w-2xl text-[13px] text-muted">{{ lead }}</p>
    <div v-if="$slots.actions" class="ml-auto flex flex-wrap items-center gap-2">
      <slot name="actions" />
    </div>
  </div>
</template>

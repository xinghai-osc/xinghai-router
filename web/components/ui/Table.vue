<script setup lang="ts">
defineProps<{ dense?: boolean }>()
</script>

<template>
  <div class="ui-table overflow-x-auto rounded-card border border-line bg-surface/90 shadow-[0_1px_0_rgb(255_255_255/0.03)]" :class="dense && 'is-dense'">
    <table class="w-full border-collapse text-left text-sm">
      <slot />
    </table>
  </div>
</template>

<style scoped>
.ui-table :deep(thead) {
  position: sticky;
  top: 0;
  z-index: 1;
  background-color: color-mix(in srgb, var(--sunken) 88%, var(--surface));
  backdrop-filter: blur(12px);
}

.ui-table :deep(th) {
  padding: 0.625rem 1rem;
  font-size: 0.75rem;
  font-weight: 500;
  color: var(--muted);
  white-space: nowrap;
}

.ui-table :deep(td) {
  padding: 0.75rem 1rem;
  color: var(--ink);
  vertical-align: middle;
}

.ui-table.is-dense :deep(td) {
  padding: 0.5rem 1rem;
}

.ui-table :deep(tbody tr) {
  border-top: 1px solid var(--line);
  transition: background-color 150ms ease-out;
}

.ui-table :deep(tbody tr:hover) {
  background-color: color-mix(in srgb, var(--sunken) 55%, transparent);
}

.ui-table :deep(.num) {
  font-family: var(--font-mono);
  font-variant-numeric: tabular-nums;
  text-align: right;
  white-space: nowrap;
}
</style>

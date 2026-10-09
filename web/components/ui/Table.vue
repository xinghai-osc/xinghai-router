<script setup lang="ts">
defineProps<{ dense?: boolean }>()
</script>

<template>
  <div
    class="ui-table min-w-0 overflow-x-auto overscroll-x-contain rounded-card border border-line bg-surface focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-clay"
    :class="dense && 'is-dense'"
    tabindex="0"
  >
    <table class="w-full border-collapse text-left text-sm leading-6">
      <slot />
    </table>
  </div>
</template>

<style scoped>
.ui-table :deep(thead) {
  position: sticky;
  top: 0;
  z-index: 1;
  background-color: color-mix(in srgb, var(--sunken) 70%, var(--surface));
}

.ui-table :deep(th) {
  padding: 0.875rem 1rem;
  font-size: 0.75rem;
  font-weight: 600;
  line-height: 1.5;
  letter-spacing: 0.025em;
  color: var(--muted);
  white-space: nowrap;
}

.ui-table :deep(td) {
  padding: 1rem;
  color: var(--ink);
  vertical-align: middle;
}

.ui-table.is-dense :deep(th) {
  padding: 0.5rem 1rem;
}

.ui-table.is-dense :deep(td) {
  padding: 0.5rem 1rem;
}

.ui-table :deep(tbody tr) {
  border-top: 1px solid var(--line);
  transition: background-color 150ms ease-out;
}

.ui-table :deep(tbody tr:hover),
.ui-table :deep(tbody tr:focus-within) {
  background-color: color-mix(in srgb, var(--sunken) 55%, transparent);
}

.ui-table :deep(.num) {
  font-family: var(--font-mono);
  font-variant-numeric: tabular-nums;
  text-align: right;
  white-space: nowrap;
}
.ui-table :deep(tfoot) {
  border-top: 1px solid var(--line);
  background-color: var(--sunken);
  font-weight: 500;
}

@media (min-width: 640px) {
  .ui-table :deep(th),
  .ui-table :deep(td) {
    padding-right: 1.5rem;
    padding-left: 1.5rem;
  }
}

@media (prefers-reduced-motion: reduce) {
  .ui-table :deep(tbody tr) {
    transition: none;
  }
}
</style>

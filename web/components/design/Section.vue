<script setup lang="ts">
defineProps<{ title: string; description?: string }>()
</script>

<template>
  <section class="space-y-4 border-t border-line pt-10">
    <div class="space-y-1">
      <h2 class="text-lg font-semibold text-ink">{{ title }}</h2>
      <p v-if="description" class="text-[13px] text-muted">{{ description }}</p>
    </div>
    <slot />
  </section>
</template>

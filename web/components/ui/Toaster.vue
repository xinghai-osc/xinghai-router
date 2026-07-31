<script setup lang="ts">
const { toasts, dismiss } = useToast()
</script>

<template>
  <Teleport to="body">
    <div class="pointer-events-none fixed inset-x-0 bottom-0 z-[60] flex flex-col items-center gap-2 p-4 sm:items-end">
      <UiAlert
        v-for="item in toasts"
        :key="item.id"
        :tone="item.tone"
        :title="item.title"
        dismissible
        class="pointer-events-auto w-full max-w-sm bg-surface! shadow-pop"
        @dismiss="dismiss(item.id)"
      >
        {{ item.body }}
      </UiAlert>
    </div>
  </Teleport>
</template>

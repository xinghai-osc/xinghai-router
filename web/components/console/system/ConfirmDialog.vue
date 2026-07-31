<script setup lang="ts">
const open = defineModel<boolean>('open', { default: false })

withDefaults(defineProps<{
  title?: string
  body?: string
  confirmLabel?: string
  busy?: boolean
}>(), {})

defineEmits<{ confirm: [] }>()

const { t } = useI18n()
</script>

<template>
  <UiDialog v-model:open="open" size="sm" :title="title ?? t('system.deleteConfirmTitle')">
    <p class="text-sm text-muted">{{ body ?? t('system.deleteConfirmBody') }}</p>

    <template #footer>
      <UiButton variant="secondary" @click="open = false">{{ t('common.cancel') }}</UiButton>
      <UiButton variant="danger" :loading="busy" @click="$emit('confirm')">
        {{ confirmLabel ?? t('common.delete') }}
      </UiButton>
    </template>
  </UiDialog>
</template>

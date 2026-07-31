<script setup lang="ts">
const model = defineModel<string>({ default: '' })

const props = defineProps<{
  id: string
  label: string
  configured: boolean
  hint?: string
  placeholder?: string
}>()

const { t } = useI18n()

const hintText = computed(() => props.hint ?? t('system.secretWriteOnlyHint'))
</script>

<template>
  <div class="space-y-1.5">
    <div class="flex items-center justify-between gap-2">
      <label :for="id" class="text-[13px] font-medium text-ink">{{ label }}</label>
      <UiBadge :tone="configured ? 'success' : 'neutral'" dot>
        {{ configured ? t('system.configured') : t('system.notConfigured') }}
      </UiBadge>
    </div>

    <UiInput
      :id="id"
      v-model="model"
      type="password"
      autocomplete="new-password"
      :placeholder="placeholder"
    />

    <p class="text-[13px] text-muted">{{ hintText }}</p>
  </div>
</template>

<script setup lang="ts">
import type { PaymentMethod, PaymentMethodForm } from '~/src/api'

const open = defineModel<boolean>('open', { default: false })

const props = defineProps<{
  method: PaymentMethod | null
  busy?: boolean
}>()

const emit = defineEmits<{ submit: [form: PaymentMethodForm] }>()

const { t } = useI18n()

const code = ref('')
const name = ref('')
const enabled = ref(true)
const invalid = ref('')

watch(() => [open.value, props.method] as const, ([isOpen]) => {
  if (!isOpen) return
  invalid.value = ''
  code.value = props.method?.code ?? ''
  name.value = props.method?.name ?? ''
  enabled.value = props.method?.enabled ?? true
}, { immediate: true })

function submit() {
  if (!code.value.trim()) {
    invalid.value = t('system.methodCodeRequired')
    return
  }
  invalid.value = ''
  emit('submit', { code: code.value.trim(), name: name.value.trim(), enabled: enabled.value })
}
</script>

<template>
  <UiDialog
    v-model:open="open"
    size="sm"
    :title="method ? t('system.editMethod') : t('system.newMethod')"
  >
    <form class="space-y-4" @submit.prevent="submit">
      <UiField
        :label="t('system.methodCode')"
        :hint="t('system.methodCodeHint')"
        :error="invalid"
        required
        for="method-code"
      >
        <UiInput id="method-code" v-model="code" mono />
      </UiField>

      <UiField :label="t('system.methodName')" :hint="t('system.methodNameHint')" for="method-name">
        <UiInput id="method-name" v-model="name" />
      </UiField>

      <div class="flex items-center gap-3">
        <UiSwitch v-model="enabled" :label="t('common.enable')" />
        <span class="text-[13px] text-muted">{{ t('common.enable') }}</span>
      </div>
    </form>

    <template #footer>
      <UiButton variant="secondary" @click="open = false">{{ t('common.cancel') }}</UiButton>
      <UiButton :loading="busy" @click="submit">{{ t('common.save') }}</UiButton>
    </template>
  </UiDialog>
</template>

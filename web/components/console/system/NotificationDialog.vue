<script setup lang="ts">
import type { Notification, NotificationForm } from '~/src/api'

const open = defineModel<boolean>('open', { default: false })

const props = defineProps<{
  notification: Notification | null
  busy?: boolean
}>()

const emit = defineEmits<{ submit: [form: NotificationForm] }>()

const { t } = useI18n()

const title = ref('')
const content = ref('')
const enabled = ref(true)
const sortOrder = ref(0)
const invalid = ref('')

watch(() => [open.value, props.notification] as const, ([isOpen]) => {
  if (!isOpen) return
  invalid.value = ''
  title.value = props.notification?.title ?? ''
  content.value = props.notification?.content ?? ''
  enabled.value = props.notification?.enabled ?? true
  sortOrder.value = props.notification?.sort_order ?? 0
}, { immediate: true })

function submit() {
  if (!title.value.trim()) {
    invalid.value = t('system.notificationTitleRequired')
    return
  }
  invalid.value = ''
  emit('submit', {
    title: title.value.trim(),
    content: content.value.trim(),
    enabled: enabled.value,
    sort_order: sortOrder.value,
  })
}
</script>

<template>
  <UiDialog
    v-model:open="open"
    size="sm"
    :title="notification ? t('system.editNotification') : t('system.newNotification')"
  >
    <form class="space-y-4" @submit.prevent="submit">
      <UiField
        :label="t('system.notificationTitle')"
        :error="invalid"
        required
        for="notification-title"
      >
        <UiInput
          id="notification-title"
          v-model="title"
          :maxlength="200"
          :placeholder="t('system.notificationTitlePlaceholder')"
        />
      </UiField>

      <UiField
        :label="t('system.notificationContent')"
        :hint="t('system.notificationContentHint')"
        for="notification-content"
      >
        <UiTextarea id="notification-content" v-model="content" :maxlength="5000" :rows="4" />
      </UiField>

      <UiField
        :label="t('system.notificationSortOrder')"
        :hint="t('system.notificationSortOrderHint')"
        for="notification-sort"
      >
        <UiInput id="notification-sort" v-model.number="sortOrder" type="number" min="0" step="1" />
      </UiField>

      <div class="flex items-center gap-3">
        <UiSwitch v-model="enabled" :label="t('system.notificationEnabled')" />
        <span class="text-[13px] text-muted">{{ t('system.notificationEnabled') }}</span>
      </div>
    </form>

    <template #footer>
      <UiButton variant="secondary" @click="open = false">{{ t('common.cancel') }}</UiButton>
      <UiButton :loading="busy" @click="submit">{{ t('common.save') }}</UiButton>
    </template>
  </UiDialog>
</template>

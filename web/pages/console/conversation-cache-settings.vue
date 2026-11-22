<script setup lang="ts">
import { endpoints, type ConversationCacheSettings } from '~/src/api'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { settings: site } = useSiteSettings()
const { toast } = useToast()
const { busy, run } = useAction()

useHead({ title: () => `${t('system.conversationCacheTitle')} · ${site.value.name}` })

const DEFAULTS: ConversationCacheSettings = {
  conversation_cache_enabled: false,
  conversation_cache_retain: false,
  conversation_cache_retention_hours: 24,
}

const { data, pending, error, refresh } = useResource(
  () => endpoints.getConversationCacheSettings(),
  { ...DEFAULTS },
)

const form = reactive<ConversationCacheSettings>({ ...DEFAULTS })
const retentionHours = ref<string | number>(DEFAULTS.conversation_cache_retention_hours)
const retentionHoursValid = computed(() => {
  const hours = Number(retentionHours.value)
  return Number.isInteger(hours) && hours >= 1 && hours <= 87600
})
const retentionHoursError = computed(() => !form.conversation_cache_retain && !retentionHoursValid.value
  ? t('system.conversationCacheRetentionHoursInvalid')
  : '')

watch(data, (next) => {
  Object.assign(form, DEFAULTS, next)
  retentionHours.value = next.conversation_cache_retention_hours ?? DEFAULTS.conversation_cache_retention_hours
}, { immediate: true })

async function save() {
  if (retentionHoursError.value) {
    toast.error(retentionHoursError.value)
    return
  }
  const payload: ConversationCacheSettings = {
    ...form,
    conversation_cache_retention_hours: retentionHoursValid.value
      ? Number(retentionHours.value)
      : form.conversation_cache_retention_hours,
  }
  const ok = await run(() => endpoints.updateConversationCacheSettings(payload))
  if (!ok) {
    toast.error(t('common.actionFailed'))
    return
  }
  toast.success(t('system.conversationCacheSaved'))
  await refresh()
}
</script>

<template>
  <ConsoleSystemGate permission="system.manage">
    <div class="space-y-4">
      <div class="min-w-0 space-y-1">
        <h2 class="text-lg font-semibold text-ink">{{ t('system.conversationCacheTitle') }}</h2>
        <p class="text-[13px] text-muted">{{ t('system.conversationCacheLead') }}</p>
      </div>

      <UiAlert v-if="error" tone="danger" :title="t('common.loadFailed')">{{ error }}</UiAlert>

      <div v-else-if="pending" class="space-y-4">
        <UiCard>
          <UiSkeleton :rows="3" class="h-10" />
        </UiCard>
      </div>

      <form v-else class="space-y-4" novalidate @submit.prevent="save">
        <UiCard>
          <div class="space-y-4">
            <UiField
              :label="t('system.conversationCacheEnabled')"
              :hint="t('system.conversationCacheEnabledHint')"
            >
              <UiSwitch
                v-model="form.conversation_cache_enabled"
                :label="t('system.conversationCacheEnabled')"
              />
            </UiField>

            <UiField
              :label="t('system.conversationCacheRetain')"
              :hint="t('system.conversationCacheRetainHint')"
            >
              <UiSwitch
                v-model="form.conversation_cache_retain"
                :label="t('system.conversationCacheRetain')"
              />
            </UiField>

            <UiField
              :label="t('system.conversationCacheRetentionHours')"
              :hint="t('system.conversationCacheRetentionHoursHint')"
              :error="retentionHoursError"
              :required="!form.conversation_cache_retain"
            >
              <UiInput
                v-model="retentionHours"
                type="number"
                min="1"
                max="87600"
                step="1"
                :disabled="form.conversation_cache_retain"
              />
            </UiField>
          </div>
        </UiCard>

        <div class="flex justify-end">
          <UiButton type="submit" :loading="busy">{{ t('common.save') }}</UiButton>
        </div>
      </form>
    </div>
  </ConsoleSystemGate>
</template>

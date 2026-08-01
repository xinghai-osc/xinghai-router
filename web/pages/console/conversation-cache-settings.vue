<script setup lang="ts">
import { endpoints, type ConversationCacheSettings } from '~/src/api'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { settings: site } = useSiteSettings()
const { toast } = useToast()
const { busy, run } = useAction()

useHead({ title: () => `${t('system.conversationCacheTitle')} · ${site.value.name}` })

const DEFAULTS: ConversationCacheSettings = { conversation_cache_enabled: false }

const { data, pending, error, refresh } = useResource(
  () => endpoints.getConversationCacheSettings(),
  { ...DEFAULTS },
)

const form = reactive<ConversationCacheSettings>({ ...DEFAULTS })

watch(data, (next) => {
  Object.assign(form, next)
}, { immediate: true })

async function save() {
  const payload: ConversationCacheSettings = { ...form }
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

      <form v-else class="space-y-4" @submit.prevent="save">
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
              :label="t('system.conversationCacheRetention')"
              :hint="t('system.conversationCacheRetentionHint')"
            >
              <p class="text-[13px] text-muted">{{ t('system.conversationCacheRetentionHint') }}</p>
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

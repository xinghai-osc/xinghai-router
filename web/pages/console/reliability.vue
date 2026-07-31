<script setup lang="ts">
import { endpoints, type ReliabilitySettings } from '~/src/api'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { settings: site } = useSiteSettings()
const { toast } = useToast()
const { busy, run } = useAction()

useHead({ title: () => `${t('system.reliabilityTitle')} · ${site.value.name}` })

const DEFAULTS: ReliabilitySettings = {
  retry_count: 0,
  retry_status_codes: '',
  health_check_mode: 'off',
  health_check_interval_minutes: 0,
  health_check_auto_recover: false,
  health_check_channel_ids: '',
  auto_disable_on_test_failure: false,
  auto_disable_slow_seconds: 0,
  auto_disable_status_codes: '',
  auto_disable_keywords: '',
}

const { data, pending, error, refresh } = useResource(
  () => endpoints.getAdminReliabilitySettings(),
  { ...DEFAULTS },
)

const form = reactive<ReliabilitySettings>({ ...DEFAULTS })
const mode = ref<string>(DEFAULTS.health_check_mode)
const retryCount = ref('0')
const interval = ref('0')
const slowSeconds = ref('0')

watch(data, (next) => {
  Object.assign(form, next)
  mode.value = next.health_check_mode || 'off'
  retryCount.value = String(next.retry_count ?? 0)
  interval.value = String(next.health_check_interval_minutes ?? 0)
  slowSeconds.value = String(next.auto_disable_slow_seconds ?? 0)
}, { immediate: true })

const modeOptions = computed(() => [
  { value: 'off', label: t('system.modeOff') },
  { value: 'scheduled_all', label: t('system.modeScheduledAll') },
  { value: 'passive_recovery', label: t('system.modePassiveRecovery') },
])

function toInt(value: string): number {
  const parsed = Number.parseInt(value.trim(), 10)
  return Number.isFinite(parsed) && parsed >= 0 ? parsed : 0
}

async function save() {
  const payload: ReliabilitySettings = {
    ...form,
    health_check_mode: mode.value as ReliabilitySettings['health_check_mode'],
    retry_count: toInt(retryCount.value),
    health_check_interval_minutes: toInt(interval.value),
    auto_disable_slow_seconds: toInt(slowSeconds.value),
  }
  const ok = await run(() => endpoints.updateReliabilitySettings(payload))
  if (!ok) {
    toast.error(t('common.actionFailed'))
    return
  }
  toast.success(t('system.reliabilitySaved'))
  await refresh()
}
</script>

<template>
  <ConsoleSystemGate permission="system.manage">
    <div class="space-y-4">
      <div class="min-w-0 space-y-1">
        <h2 class="text-lg font-semibold text-ink">{{ t('system.reliabilityTitle') }}</h2>
        <p class="text-[13px] text-muted">{{ t('system.reliabilityLead') }}</p>
      </div>

      <UiAlert v-if="error" tone="danger" :title="t('common.loadFailed')">{{ error }}</UiAlert>

      <div v-else-if="pending" class="space-y-4">
        <UiCard>
          <UiSkeleton :rows="4" class="h-10" />
        </UiCard>
        <UiCard>
          <UiSkeleton :rows="5" class="h-10" />
        </UiCard>
      </div>

      <form v-else class="space-y-4" @submit.prevent="save">
        <UiCard :title="t('system.retrySection')">
          <div class="grid gap-4 sm:grid-cols-2">
            <UiField :label="t('system.retryCount')" :hint="t('system.retryCountHint')" for="retry-count">
              <UiInput id="retry-count" v-model="retryCount" />
            </UiField>

            <UiField
              :label="t('system.retryStatusCodes')"
              :hint="t('system.retryStatusCodesHint')"
              for="retry-codes"
            >
              <UiInput id="retry-codes" v-model="form.retry_status_codes" mono />
            </UiField>
          </div>
        </UiCard>

        <UiCard :title="t('system.healthCheckSection')">
          <div class="space-y-4">
            <div class="grid gap-4 sm:grid-cols-2">
              <UiField
                :label="t('system.healthCheckMode')"
                :hint="t('system.healthCheckModeHint')"
                for="health-mode"
              >
                <UiSelect
                  id="health-mode"
                  v-model="mode"
                  :options="modeOptions"
                  :placeholder="t('common.selectPlaceholder')"
                />
              </UiField>

              <UiField
                :label="t('system.healthCheckInterval')"
                :hint="t('system.healthCheckIntervalHint')"
                for="health-interval"
              >
                <UiInput id="health-interval" v-model="interval" />
              </UiField>
            </div>

            <UiField
              :label="t('system.healthCheckChannelIds')"
              :hint="t('system.healthCheckChannelIdsHint')"
              for="health-channels"
            >
              <UiInput id="health-channels" v-model="form.health_check_channel_ids" mono />
            </UiField>

            <UiField :label="t('system.healthCheckAutoRecover')" :hint="t('system.healthCheckAutoRecoverHint')">
              <UiSwitch
                v-model="form.health_check_auto_recover"
                :label="t('system.healthCheckAutoRecover')"
              />
            </UiField>
          </div>
        </UiCard>

        <UiCard :title="t('system.autoDisableSection')">
          <div class="space-y-4">
            <UiField
              :label="t('system.autoDisableOnTestFailure')"
              :hint="t('system.autoDisableOnTestFailureHint')"
            >
              <UiSwitch
                v-model="form.auto_disable_on_test_failure"
                :label="t('system.autoDisableOnTestFailure')"
              />
            </UiField>

            <div class="grid gap-4 sm:grid-cols-2">
              <UiField
                :label="t('system.autoDisableSlowSeconds')"
                :hint="t('system.autoDisableSlowSecondsHint')"
                for="disable-slow"
              >
                <UiInput id="disable-slow" v-model="slowSeconds" />
              </UiField>

              <UiField
                :label="t('system.autoDisableStatusCodes')"
                :hint="t('system.autoDisableStatusCodesHint')"
                for="disable-codes"
              >
                <UiInput id="disable-codes" v-model="form.auto_disable_status_codes" mono />
              </UiField>
            </div>

            <UiField
              :label="t('system.autoDisableKeywords')"
              :hint="t('system.autoDisableKeywordsHint')"
              for="disable-keywords"
            >
              <UiTextarea id="disable-keywords" v-model="form.auto_disable_keywords" mono :rows="3" />
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

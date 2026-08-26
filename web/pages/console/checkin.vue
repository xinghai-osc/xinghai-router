<script setup lang="ts">
import { CalendarCheck } from 'lucide-vue-next'
import { endpoints, type CheckinEntry } from '~/src/api'
import { formatDate, formatMoney } from '~/src/format'
import { GEETEST_CANCELLED } from '~/composables/useGeetest'
import { CORPTCHA_CANCELLED, CORPTCHA_UNAVAILABLE } from '~/composables/useCorptcha'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { settings } = useSiteSettings()
const { toast } = useToast()
const { busy, run } = useAction()
const { challenge: geetestChallenge } = useGeetest()
const { challenge: corptchaChallenge } = useCorptcha()

const captchaProvider = computed(() => settings.value.captcha_provider ?? '')
const geetestCaptchaId = computed(() => settings.value.geetest_captcha_id ?? '')
const corptchaSiteKey = computed(() => settings.value.corptcha_site_id ?? '')
const captchaEnabled = computed(() =>
  captchaProvider.value === 'geetest' ? geetestCaptchaId.value !== '' : captchaProvider.value === 'corptcha' ? corptchaSiteKey.value !== '' : false,
)
const formError = ref('')

async function runCaptcha(): Promise<Record<string, string> | null> {
  try {
    if (captchaProvider.value === 'corptcha') return await corptchaChallenge(corptchaSiteKey.value, 'checkin')
    return await geetestChallenge(geetestCaptchaId.value)
  } catch (cause) {
    const reason = cause instanceof Error ? cause.message : ''
    formError.value = reason === GEETEST_CANCELLED || reason === CORPTCHA_CANCELLED || reason === CORPTCHA_UNAVAILABLE
      ? t('auth.captchaRequired')
      : t('auth.captchaUnavailable')
    return null
  }
}

useHead({ title: () => `${t('nav.checkin')} · ${settings.value.name}` })

const { data: status, pending, error, refresh } = useResource(
  () => endpoints.getCheckinStatus(),
  { checked_in: false, data: [] as CheckinEntry[] },
)

async function submit() {
  formError.value = ''
  const ok = await run(async () => {
    const captcha = captchaEnabled.value ? await runCaptcha() : null
    if (captchaEnabled.value && !captcha) return
    const result = await endpoints.checkin(captcha ?? undefined)
    if (result.already_checked_in) toast.success(t('console.checkinAlready'))
    else toast.success(t('console.checkinSuccess', { reward: formatMoney(result.reward ?? 0, 4) }))
    await refresh()
  })
  if (!ok) toast.error(t('common.actionFailed'))
}
</script>

<template>
  <div class="space-y-4">
    <UiCard :title="t('console.checkinTitle')" :description="t('console.checkinDescription')">
      <div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div class="flex items-center gap-3">
          <div class="grid size-11 place-items-center rounded-full bg-clay-soft text-clay"><CalendarCheck class="size-5" /></div>
          <div>
            <p class="font-medium text-ink">{{ status.checked_in ? t('console.checkinDone') : t('console.checkinAvailable') }}</p>
            <p class="text-sm text-muted">{{ t('console.checkinRewardHint') }}</p>
          </div>
        </div>
        <UiButton :loading="busy" :disabled="status.checked_in" @click="submit">
          {{ status.checked_in ? t('console.checkinDone') : t('console.checkinButton') }}
        </UiButton>
      </div>
      <UiAlert v-if="formError" class="mt-4" tone="danger" :title="formError" />
    </UiCard>

    <UiCard :title="t('console.checkinHistoryTitle')" flush>
      <div class="px-5 py-4">
        <ConsoleUserDataState
          :pending="pending"
          :error="error"
          :empty="!status.data.length"
          :rows="5"
          :empty-icon="CalendarCheck"
          :empty-title="t('console.checkinEmptyTitle')"
          :empty-description="t('console.checkinEmptyBody')"
        >
          <UiTable>
            <thead><tr><th>{{ t('console.checkinDate') }}</th><th class="num">{{ t('console.checkinStreak') }}</th><th class="num">{{ t('console.checkinReward') }}</th></tr></thead>
            <tbody><tr v-for="item in status.data" :key="item.checkin_date"><td>{{ formatDate(item.checkin_date) }}</td><td class="num">{{ item.streak }}</td><td class="num text-success">+{{ formatMoney(item.reward, 4) }}</td></tr></tbody>
          </UiTable>
        </ConsoleUserDataState>
      </div>
    </UiCard>
  </div>
</template>

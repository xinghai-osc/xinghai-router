<script setup lang="ts">
import { LogIn, Ticket } from 'lucide-vue-next'
import { ApiError, endpoints, getToken, type RedemptionResult } from '~/src/api'
import { formatMoney } from '~/src/format'

const { t } = useI18n()
const { settings } = useSiteSettings()
const { toast } = useToast()

useHead({
  title: () => `${t('site.redeemMetaTitle')} · ${settings.value.name}`,
  meta: [{ name: 'description', content: () => t('site.redeemMetaDescription') }],
})

const route = useRoute()
const code = ref(typeof route.query.code === 'string' ? route.query.code : '')
const formError = ref('')
const result = ref<RedemptionResult | null>(null)
const signedIn = ref(!!getToken())
const busy = ref(false)

const REDEEM_ERROR_KEYS: Record<string, string> = {
  not_found: 'site.redeemCodeNotFound',
  code_disabled: 'site.redeemCodeDisabled',
  code_expired: 'site.redeemCodeExpired',
  code_used: 'site.redeemCodeUsed',
  already_redeemed: 'site.redeemAlreadyRedeemed',
}

/** Sign-in link that returns to this redeem page, keeping the code in the URL. */
const signInTarget = computed(() => {
  const query = code.value ? `?code=${encodeURIComponent(code.value)}` : ''
  return `/auth?redirect=${encodeURIComponent(`/redeem${query}`)}`
})

const resultText = computed(() => {
  if (!result.value) return ''
  if (result.value.reward_type === 'balance' && result.value.amount) {
    return t('site.redeemSuccessBalance', { amount: formatMoney(result.value.amount) })
  }
  return t('site.redeemSuccessSubscription')
})

async function submit() {
  const value = code.value.trim()
  if (!value) {
    formError.value = t('site.redeemCodeRequired')
    return
  }
  formError.value = ''
  result.value = null
  busy.value = true
  try {
    result.value = await endpoints.redeemCode(value)
    toast.success(resultText.value)
  } catch (cause) {
    if (cause instanceof ApiError) {
      const key = REDEEM_ERROR_KEYS[cause.code]
      formError.value = key ? t(key) : cause.message
    } else {
      formError.value = t('common.actionFailed')
    }
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div>
    <section class="shell pt-16 pb-10 md:pt-20">
      <div class="max-w-2xl space-y-3">
        <p class="text-2xs font-medium tracking-wide text-clay uppercase">{{ t('site.redeemEyebrow') }}</p>
        <h1 class="display text-4xl text-ink md:text-5xl">{{ t('site.redeemTitle') }}</h1>
        <p class="text-muted">{{ t('site.redeemLead') }}</p>
      </div>
    </section>

    <section class="shell pb-24">
      <div class="mx-auto w-full max-w-xl space-y-5">
        <div class="aspect-[4/3] overflow-hidden rounded-card border border-line bg-surface">
          <img
            src="/card.png"
            width="1448"
            height="1086"
            :alt="t('site.redeemCardAlt')"
            class="h-full w-full object-contain"
          >
        </div>

        <UiCard flush>
          <div v-if="!signedIn" class="flex flex-col items-center gap-4 px-5 py-8 text-center">
            <Ticket class="size-8 text-muted" />
            <div class="space-y-1">
              <h2 class="text-base font-semibold text-ink">{{ t('site.redeemSignInTitle') }}</h2>
              <p class="text-sm text-muted">{{ t('site.redeemSignInBody') }}</p>
            </div>
            <p v-if="code" class="rounded-control bg-sunken px-3 py-1.5 font-mono text-[13px] text-ink">{{ code }}</p>
            <UiButton :to="signInTarget" size="lg">
              <LogIn class="size-4" />
              {{ t('site.redeemSignInCta') }}
            </UiButton>
          </div>

          <form v-else class="space-y-4 px-5 py-5" novalidate @submit.prevent="submit">
            <UiField :label="t('site.redeemCodeLabel')" :error="formError" required>
              <UiInput v-model="code" :placeholder="t('site.redeemCodePlaceholder')" mono>
                <template #leading>
                  <Ticket class="size-4 text-muted" />
                </template>
              </UiInput>
            </UiField>

            <UiAlert v-if="result" tone="success" :title="resultText" />

            <UiButton type="submit" size="lg" block :loading="busy">
              {{ t('site.redeemSubmit') }}
            </UiButton>

            <p class="text-center text-sm">
              <NuxtLink to="/console/redeem" class="text-muted transition-colors duration-150 hover:text-clay">
                {{ t('site.redeemViewHistory') }}
              </NuxtLink>
            </p>
          </form>
        </UiCard>
      </div>
    </section>
  </div>
</template>

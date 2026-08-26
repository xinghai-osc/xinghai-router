<script setup lang="ts">
import { endpoints, getToken } from '~/src/api'
import { GEETEST_CANCELLED } from '~/composables/useGeetest'
import { CORPTCHA_CANCELLED, CORPTCHA_UNAVAILABLE } from '~/composables/useCorptcha'

const RESEND_SECONDS = 60

const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const { settings } = useSiteSettings()
const { signIn } = useAccount()
const { toast } = useToast()
const { challenge: geetestChallenge } = useGeetest()
const { challenge: corptchaChallenge } = useCorptcha()

const mode = ref(route.query.mode === 'register' ? 'register' : route.query.mode === 'reset' ? 'reset' : 'signin')
const form = reactive({ name: '', email: '', password: '', code: '', invitationCode: typeof route.query.invite === 'string' ? route.query.invite : '' })
const formError = ref('')
const busy = ref(false)
const sending = ref(false)
const cooldown = ref(0)
const resetSent = ref(false)
let cooldownTimer: ReturnType<typeof setInterval> | null = null

const isRegister = computed(() => mode.value === 'register')
const isReset = computed(() => mode.value === 'reset')
const plan = computed(() => (typeof route.query.plan === 'string' ? route.query.plan : ''))
/** Effective captcha provider as resolved by the Go service. */
const captchaProvider = computed(() => settings.value.captcha_provider ?? '')
const geetestCaptchaId = computed(() => settings.value.geetest_captcha_id ?? '')
const corptchaSiteKey = computed(() => settings.value.corptcha_site_id ?? '')
const captchaEnabled = computed(() =>
  captchaProvider.value === 'geetest' ? geetestCaptchaId.value !== '' : captchaProvider.value === 'corptcha' ? corptchaSiteKey.value !== '' : false,
)
const emailCodeEnabled = computed(() => Boolean(settings.value.email_verification_enabled))

// Registration always verifies the captcha at submit (like sign-in and reset);
// when email verification is also on, the code is verified in addition.
const captchaOnSubmit = computed(() => captchaEnabled.value)

const tabs = computed(() => [
  { value: 'signin', label: t('common.signIn') },
  { value: 'register', label: t('common.signUp') },
])

const title = computed(() => (isRegister.value ? t('auth.signUpTitle') : isReset.value ? t('auth.resetTitle') : t('auth.signInTitle')))
const lead = computed(() => (isRegister.value ? t('auth.signUpLead') : isReset.value ? t('auth.resetLead') : t('auth.signInLead')))

useHead({ title: () => `${title.value} · ${settings.value.name}` })

watch(mode, (next) => {
  formError.value = ''
  resetSent.value = false
  router.replace({ query: { ...route.query, mode: next === 'register' ? 'register' : next === 'reset' ? 'reset' : undefined } })
})

function stopCooldown() {
  if (!cooldownTimer) return
  clearInterval(cooldownTimer)
  cooldownTimer = null
}

function startCooldown() {
  stopCooldown()
  cooldown.value = RESEND_SECONDS
  cooldownTimer = setInterval(() => {
    cooldown.value -= 1
    if (cooldown.value <= 0) stopCooldown()
  }, 1000)
}

onBeforeUnmount(stopCooldown)

// A live session reaching this page (SameSite=strict keeps the cookie off the
// SSR request, so the console middleware can bounce a signed-in visitor here).
onMounted(() => {
  if (getToken()) navigateTo(consoleTarget())
})

function consoleTarget() {
  return { path: '/console', query: plan.value ? { plan: plan.value } : {} }
}

function validate(): string {
  if (isRegister.value && !form.name.trim()) return t('auth.nameRequired')
  if (form.password.length < 8) return t('auth.passwordTooShort')
  if (form.password.length > 72) return t('auth.passwordTooLong')
  if (isRegister.value && emailCodeEnabled.value && !form.code.trim()) return t('auth.codeRequired')
  return ''
}

/**
 * Runs the active provider's challenge and resolves with the payload the Go
 * handlers expect: captcha_token/captcha_purpose for Corptcha, the Geetest
 * fields otherwise. Resolves to null when the challenge was dismissed or
 * could not be shown.
 */
async function runCaptcha(purpose: string): Promise<Record<string, string> | null> {
  try {
    if (captchaProvider.value === 'corptcha') {
      return await corptchaChallenge(corptchaSiteKey.value, purpose)
    }
    return await geetestChallenge(geetestCaptchaId.value)
  } catch (cause) {
    const reason = cause instanceof Error ? cause.message : ''
    const cancelled = reason === GEETEST_CANCELLED || reason === CORPTCHA_CANCELLED
    formError.value = cancelled || reason === CORPTCHA_UNAVAILABLE ? t('auth.captchaRequired') : t('auth.captchaUnavailable')
    return null
  }
}

function describe(cause: unknown): string {
  if (cause instanceof Error && cause.message.includes('email_not_allowed')) return t('auth.emailNotAllowed')
  return cause instanceof Error && cause.message ? cause.message : t('common.requestFailed')
}

async function sendCode() {
  if (sending.value || cooldown.value > 0) return
  formError.value = ''
  const email = form.email.trim()
  sending.value = true
  try {
    let captcha: Record<string, string> | null = null
    if (captchaEnabled.value) {
      captcha = await runCaptcha('email_code')
      if (!captcha) return
    }
    await endpoints.sendEmailCode(email, captcha ?? undefined)
    toast.success(t('auth.codeSent'))
    startCooldown()
  } catch (cause) {
    formError.value = describe(cause)
  } finally {
    sending.value = false
  }
}

async function submitReset() {
  if (busy.value || resetSent.value) return
  formError.value = ''
  const email = form.email.trim()
  if (!email) {
    formError.value = t('auth.emailInvalid')
    return
  }
  busy.value = true
  try {
    let captcha: Record<string, string> | null = null
    if (captchaEnabled.value) {
      captcha = await runCaptcha('reset')
      if (!captcha) return
    }
    await endpoints.requestPasswordReset(email, captcha ?? undefined)
    resetSent.value = true
  } catch (cause) {
    formError.value = describe(cause)
  } finally {
    busy.value = false
  }
}

async function submit() {
  if (busy.value) return
  formError.value = ''
  const invalid = validate()
  if (invalid) {
    formError.value = invalid
    return
  }
  busy.value = true
  try {
    let captcha: Record<string, string> | null = null
    if (captchaOnSubmit.value) {
      captcha = await runCaptcha(isRegister.value ? 'register' : 'login')
      if (!captcha) return
    }
    const email = form.email.trim()
    const { token } = isRegister.value
      ? await endpoints.register({ name: form.name.trim(), email, password: form.password, code: form.code.trim(), invitation_code: form.invitationCode.trim(), ...captcha })
      : await endpoints.login({ email, password: form.password, ...captcha })
    await signIn(token)
    toast.success(isRegister.value ? t('auth.signUpSuccess') : t('auth.signInSuccess'))
    await navigateTo(consoleTarget())
  } catch (cause) {
    formError.value = describe(cause)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="shell flex min-h-[calc(100dvh-9rem)] items-center justify-center py-16 md:py-20">
    <div class="w-full max-w-[26rem]">
      <div class="flex flex-col items-center gap-5 text-center">
        <SiteLogo :name="settings.name" :icon-url="settings.icon_url" />
        <div class="space-y-2">
          <h1 class="display text-3xl text-ink md:text-4xl">{{ title }}</h1>
          <p class="text-sm text-muted">{{ lead }}</p>
        </div>
      </div>

      <UiCard flush class="mt-8">
        <UiTabs v-model="mode" :items="tabs">
          <form v-if="!isReset" class="space-y-4 px-5 py-5" novalidate @submit.prevent="submit">
            <UiField v-if="isRegister" :label="t('auth.displayName')" for="auth-name" required>
              <UiInput
                id="auth-name"
                v-model="form.name"
                autocomplete="nickname"
                :placeholder="t('auth.displayNamePlaceholder')"
              />
            </UiField>

            <UiField v-if="isRegister && settings.invitations_enabled" :label="t('auth.invitationCode')" for="auth-invitation" :hint="t('auth.invitationCodeHint')">
              <UiInput
                id="auth-invitation"
                v-model="form.invitationCode"
                autocomplete="off"
                mono
                :placeholder="t('auth.invitationCodePlaceholder')"
              />
            </UiField>

            <UiField :label="isRegister ? t('auth.email') : t('auth.emailOrUsername')" for="auth-email" required>
              <UiInput
                id="auth-email"
                v-model="form.email"
                type="text"
                autocomplete="email"
                :placeholder="isRegister ? t('auth.emailPlaceholder') : t('auth.emailOrUsernamePlaceholder')"
              />
            </UiField>

            <UiField
              :label="t('auth.password')"
              for="auth-password"
              :hint="isRegister ? t('auth.passwordHint') : undefined"
              required
            >
              <UiInput
                id="auth-password"
                v-model="form.password"
                type="password"
                :autocomplete="isRegister ? 'new-password' : 'current-password'"
              />
            </UiField>

            <div v-if="!isRegister && emailCodeEnabled" class="-mt-2 flex justify-end">
              <button
                type="button"
                class="text-xs text-muted transition-colors duration-150 hover:text-clay"
                @click="mode = 'reset'"
              >
                {{ t('auth.forgotPassword') }}
              </button>
            </div>

            <UiField v-if="isRegister && emailCodeEnabled" :label="t('auth.emailCode')" for="auth-code" required>
              <div class="flex items-center gap-2">
                <UiInput
                  id="auth-code"
                  v-model="form.code"
                  class="flex-1"
                  autocomplete="one-time-code"
                  :placeholder="t('auth.emailCodePlaceholder')"
                />
                <UiButton
                  variant="secondary"
                  :loading="sending"
                  :disabled="cooldown > 0"
                  @click="sendCode"
                >
                  {{ cooldown > 0 ? t('auth.resendIn', { seconds: cooldown }) : t('auth.sendCode') }}
                </UiButton>
              </div>
            </UiField>

            <UiAlert v-if="formError" tone="danger" dismissible @dismiss="formError = ''">
              {{ formError }}
            </UiAlert>

            <UiButton type="submit" size="lg" block :loading="busy">
              {{ isRegister ? t('common.signUp') : t('common.signIn') }}
            </UiButton>

            <div v-if="!isRegister && settings.oauth_providers?.length" class="relative my-2">
              <div class="absolute inset-0 flex items-center">
                <span class="w-full border-t border-line" />
              </div>
              <div class="relative flex justify-center text-xs">
                <span class="bg-surface px-2 text-muted">{{ t('auth.orContinueWith') }}</span>
              </div>
            </div>

            <div v-if="!isRegister" class="flex flex-col gap-2">
              <a
                v-for="p in settings.oauth_providers || []"
                :key="p"
                :href="`/api/auth/oauth/${p}`"
                class="inline-flex items-center justify-center gap-2 rounded-control border border-line bg-surface px-4 py-2.5 text-sm font-medium text-ink transition-colors duration-150 hover:bg-sunken"
              >
                <img v-if="p === 'github'" src="data:image/svg+xml,%3Csvg%20xmlns%3D%22http%3A%2F%2Fwww.w3.org%2F2000%2Fsvg%22%20width%3D%2216%22%20height%3D%2216%22%20viewBox%3D%220%200%2024%2024%22%3E%3Cpath%20fill%3D%22%23181717%22%20d%3D%22M12%200C5.37%200%200%205.37%200%2012c0%205.31%203.435%209.795%208.205%2011.385.6.105.825-.255.825-.57%200-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015%201.62.87%201.845%201.23%201.08%201.815%202.805%201.305%203.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925%200-1.305.465-2.385%201.23-3.225-.12-.3-.54-1.53.12-3.18%200%200%201.005-.315%203.3%201.23.96-.27%201.98-.405%203-.405s2.04.135%203%20.405c2.295-1.56%203.3-1.23%203.3-1.23.66%201.65.24%202.88.12%203.18.765.84%201.23%201.905%201.23%203.225%200%204.605-2.805%205.625-5.475%205.925.435.375.81%201.095.81%202.22%200%201.605-.015%202.895-.015%203.3%200%20.315.225.69.825.57A12.02%2012.02%200%200%200%2024%2012c0-6.63-5.37-12-12-12z%22%3E%3C%2Fpath%3E%3C%2Fsvg%3E" alt="GitHub" class="size-4">
                <span>{{ p === 'github' ? 'GitHub' : p }}</span>
              </a>
            </div>
          </form>

          <form v-else class="space-y-4 px-5 py-5" novalidate @submit.prevent="submitReset">
            <UiField :label="t('auth.email')" for="auth-reset-email" required>
              <UiInput
                id="auth-reset-email"
                v-model="form.email"
                type="text"
                autocomplete="email"
                :placeholder="t('auth.emailPlaceholder')"
              />
            </UiField>

            <UiAlert v-if="formError" tone="danger" dismissible @dismiss="formError = ''">
              {{ formError }}
            </UiAlert>

            <UiAlert v-if="resetSent" tone="success" dismissible @dismiss="resetSent = false">
              {{ t('auth.resetSent') }}
            </UiAlert>

            <UiButton type="submit" size="lg" block :loading="busy" :disabled="resetSent">
              {{ t('auth.sendResetLink') }}
            </UiButton>
          </form>
        </UiTabs>
      </UiCard>

      <p class="mt-6 text-center text-[13px] text-muted">
        <button
          type="button"
          class="text-clay underline-offset-4 transition-opacity duration-150 hover:underline"
          @click="mode = isReset ? 'signin' : isRegister ? 'signin' : 'register'"
        >
          {{ isReset ? t('auth.resetBackToSignIn') : isRegister ? t('auth.toSignIn') : t('auth.toSignUp') }}
        </button>
      </p>
    </div>
  </div>
</template>

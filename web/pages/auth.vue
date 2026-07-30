<script setup lang="ts">
import { endpoints, getToken } from '~/src/api'
import { GEETEST_CANCELLED, type GeetestResult } from '~/composables/useGeetest'

const RESEND_SECONDS = 60

const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const { settings } = useSiteSettings()
const { signIn } = useAccount()
const { toast } = useToast()
const { challenge } = useGeetest()

const mode = ref(route.query.mode === 'register' ? 'register' : 'signin')
const form = reactive({ name: '', email: '', password: '', code: '' })
const formError = ref('')
const busy = ref(false)
const sending = ref(false)
const cooldown = ref(0)
let cooldownTimer: ReturnType<typeof setInterval> | null = null

const isRegister = computed(() => mode.value === 'register')
const plan = computed(() => (typeof route.query.plan === 'string' ? route.query.plan : ''))
const captchaId = computed(() => settings.value.geetest_captcha_id ?? '')
const captchaEnabled = computed(() => Boolean(settings.value.geetest_enabled) && captchaId.value !== '')
const emailCodeEnabled = computed(() => Boolean(settings.value.email_verification_enabled))

// The register handler verifies the email code *instead of* the captcha, so a
// second challenge at submit time would be dead weight there.
const captchaOnSubmit = computed(() => captchaEnabled.value && !(isRegister.value && emailCodeEnabled.value))

const tabs = computed(() => [
  { value: 'signin', label: t('common.signIn') },
  { value: 'register', label: t('common.signUp') },
])

const title = computed(() => (isRegister.value ? t('auth.signUpTitle') : t('auth.signInTitle')))
const lead = computed(() => (isRegister.value ? t('auth.signUpLead') : t('auth.signInLead')))

useHead({ title: () => `${title.value} · ${settings.value.name}` })

watch(mode, (next) => {
  formError.value = ''
  router.replace({ query: { ...route.query, mode: next === 'register' ? 'register' : undefined } })
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

/** Resolves to null when the challenge was dismissed or could not be shown. */
async function runCaptcha(): Promise<GeetestResult | null> {
  try {
    return await challenge(captchaId.value)
  } catch (cause) {
    const reason = cause instanceof Error ? cause.message : ''
    formError.value = reason === GEETEST_CANCELLED ? t('auth.captchaRequired') : t('auth.captchaUnavailable')
    return null
  }
}

function describe(cause: unknown): string {
  return cause instanceof Error && cause.message ? cause.message : t('common.requestFailed')
}

async function sendCode() {
  if (sending.value || cooldown.value > 0) return
  formError.value = ''
  const email = form.email.trim()
  sending.value = true
  try {
    let captcha: GeetestResult | null = null
    if (captchaEnabled.value) {
      captcha = await runCaptcha()
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
    let captcha: GeetestResult | null = null
    if (captchaOnSubmit.value) {
      captcha = await runCaptcha()
      if (!captcha) return
    }
    const email = form.email.trim()
    const { token } = isRegister.value
      ? await endpoints.register({ name: form.name.trim(), email, password: form.password, code: form.code.trim(), ...captcha })
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
          <form class="space-y-4 px-5 py-5" novalidate @submit.prevent="submit">
            <UiField v-if="isRegister" :label="t('auth.displayName')" for="auth-name" required>
              <UiInput
                id="auth-name"
                v-model="form.name"
                autocomplete="nickname"
                :placeholder="t('auth.displayNamePlaceholder')"
              />
            </UiField>

            <UiField :label="t('auth.email')" for="auth-email" required>
              <UiInput
                id="auth-email"
                v-model="form.email"
                type="text"
                autocomplete="email"
                :placeholder="t('auth.emailPlaceholder')"
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
          </form>
        </UiTabs>
      </UiCard>

      <p class="mt-6 text-center text-[13px] text-muted">
        <button
          type="button"
          class="text-clay underline-offset-4 transition-opacity duration-150 hover:underline"
          @click="mode = isRegister ? 'signin' : 'register'"
        >
          {{ isRegister ? t('auth.toSignIn') : t('auth.toSignUp') }}
        </button>
      </p>
    </div>
  </div>
</template>

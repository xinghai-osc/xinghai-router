<script setup lang="ts">
import { endpoints } from '~/src/api'

const route = useRoute()
const { t } = useI18n()
const { settings } = useSiteSettings()
const { toast } = useToast()

const token = computed(() => (typeof route.query.token === 'string' ? route.query.token.trim() : ''))
const form = reactive({ password: '', confirm: '' })
const formError = ref('')
const busy = ref(false)
const done = ref(false)

const title = computed(() => t('auth.resetTitle'))
const lead = computed(() => t('auth.resetLead'))

useHead({ title: () => `${title.value} · ${settings.value.name}` })

function validate(): string {
  if (form.password.length < 8) return t('auth.passwordTooShort')
  if (form.password.length > 72) return t('auth.passwordTooLong')
  if (form.password !== form.confirm) return t('auth.passwordsMismatch')
  return ''
}

async function submit() {
  if (busy.value || done.value) return
  formError.value = ''
  const invalid = validate()
  if (invalid) {
    formError.value = invalid
    return
  }
  busy.value = true
  try {
    await endpoints.confirmPasswordReset(token.value, form.password)
    done.value = true
    toast.success(t('auth.resetSuccess'))
    await navigateTo('/auth')
  } catch (cause) {
    formError.value = cause instanceof Error && cause.message ? cause.message : t('common.requestFailed')
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
        <form v-if="token" class="space-y-4 px-5 py-5" novalidate @submit.prevent="submit">
          <UiField :label="t('auth.newPassword')" for="auth-reset-password" :hint="t('auth.passwordHint')" required>
            <UiInput
              id="auth-reset-password"
              v-model="form.password"
              type="password"
              autocomplete="new-password"
            />
          </UiField>

          <UiField :label="t('auth.confirmPassword')" for="auth-reset-confirm" required>
            <UiInput
              id="auth-reset-confirm"
              v-model="form.confirm"
              type="password"
              autocomplete="new-password"
            />
          </UiField>

          <UiAlert v-if="formError" tone="danger" dismissible @dismiss="formError = ''">
            {{ formError }}
          </UiAlert>

          <UiButton type="submit" size="lg" block :loading="busy">
            {{ t('auth.resetTitle') }}
          </UiButton>
        </form>

        <div v-else class="space-y-4 px-5 py-5">
          <UiAlert tone="danger">
            {{ t('auth.resetInvalidToken') }}
          </UiAlert>
          <UiButton to="/auth" size="lg" block variant="secondary">
            {{ t('auth.resetBackToSignIn') }}
          </UiButton>
        </div>
      </UiCard>

      <p class="mt-6 text-center text-[13px] text-muted">
        <NuxtLink to="/auth" class="text-clay underline-offset-4 transition-opacity duration-150 hover:underline">
          {{ t('auth.resetBackToSignIn') }}
        </NuxtLink>
      </p>
    </div>
  </div>
</template>

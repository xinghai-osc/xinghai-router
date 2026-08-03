<script setup lang="ts">
import { UserRound, Link } from 'lucide-vue-next'
import { endpoints, type OAuthConnection } from '~/src/api'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const AVATAR_MAX_BYTES = 2 * 1024 * 1024
const AVATAR_TYPES = ['image/png', 'image/jpeg', 'image/gif', 'image/webp']
const MIN_PASSWORD_LENGTH = 8

const { t } = useI18n()
const { toast } = useToast()
const { account, loadAccount } = useAccount()
const { settings } = useSiteSettings()

useHead({ title: () => `${t('nav.account')} · ${settings.value.name}` })

const profileAction = useAction()
const passwordAction = useAction()
const preferencesAction = useAction()
const unlinkAction = useAction()

const { data: connections, refresh: refreshConnections } = useResource(
  () => endpoints.getOAuthConnections(),
  { data: [] as OAuthConnection[] },
)

async function unlink(provider: string) {
  const ok = await unlinkAction.run(() => endpoints.unlinkOAuthConnection(provider))
  if (!ok) { toast.error(t('common.actionFailed')); return }
  toast.success(t('console.oauthUnlinked'))
  await refreshConnections()
}

const avatarUrl = ref('')
const avatarError = ref('')
const fileInput = ref<HTMLInputElement | null>(null)

const password = reactive({ current: '', next: '', confirm: '' })
const passwordError = ref('')

const optIn = ref(false)
const maskName = ref(false)
const dataUsage = ref(true)

// While must_change_password is set the backend only answers /account/me,
// /account/password and /auth/logout — the other two cards would 403.
const locked = computed(() => Boolean(account.value?.must_change_password))

watch(account, (value) => {
  if (!value) return
  avatarUrl.value = value.avatar_url
  optIn.value = value.leaderboard_opt_in
  maskName.value = value.leaderboard_mask_name
  dataUsage.value = value.data_usage_enabled
}, { immediate: true })

function pickAvatar() {
  fileInput.value?.click()
}

function onAvatarSelected(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return

  if (!AVATAR_TYPES.includes(file.type)) { avatarError.value = t('console.avatarInvalidType'); return }
  if (file.size > AVATAR_MAX_BYTES) { avatarError.value = t('console.avatarTooLarge'); return }

  const reader = new FileReader()
  reader.onload = () => {
    avatarError.value = ''
    avatarUrl.value = typeof reader.result === 'string' ? reader.result : ''
  }
  reader.onerror = () => { avatarError.value = t('console.avatarReadFailed') }
  reader.readAsDataURL(file)
}

async function saveProfile() {
  const ok = await profileAction.run(() => endpoints.updateAccountProfile(avatarUrl.value.trim()))
  if (!ok) {
    avatarError.value = profileAction.error.value
    toast.error(t('common.actionFailed'))
    return
  }
  avatarError.value = ''
  toast.success(t('console.profileSaved'))
  await loadAccount(true)
}

async function savePassword() {
  if (password.next.length < MIN_PASSWORD_LENGTH) { passwordError.value = t('console.passwordTooShort'); return }
  if (password.next !== password.confirm) { passwordError.value = t('console.passwordMismatch'); return }
  if (password.next === password.current) { passwordError.value = t('console.passwordSameAsCurrent'); return }

  const ok = await passwordAction.run(() => endpoints.changeAccountPassword(password.current, password.next))
  if (!ok) {
    passwordError.value = passwordAction.error.value
    toast.error(t('common.actionFailed'))
    return
  }
  passwordError.value = ''
  password.current = ''
  password.next = ''
  password.confirm = ''
  toast.success(t('console.passwordChanged'))
  // Clears must_change_password, which unlocks the rest of the console.
  await loadAccount(true)
}

async function savePreferences() {
  const ok = await preferencesAction.run(() => endpoints.updateAccountPreferences(optIn.value, maskName.value, dataUsage.value))
  if (!ok) { toast.error(t('common.actionFailed')); return }
  toast.success(t('console.preferencesSaved'))
  await loadAccount(true)
}
</script>

<template>
  <div class="grid gap-4 lg:grid-cols-2">
    <UiCard
      :title="t('console.profileTitle')"
      :description="t('console.profileDescription')"
      :class="locked && 'order-2'"
    >
      <div class="space-y-4">
        <dl class="space-y-1 text-[13px]">
          <div class="flex justify-between gap-3">
            <dt class="text-muted">{{ t('console.accountId') }}</dt>
            <dd class="truncate font-mono text-ink">{{ account?.id }}</dd>
          </div>
          <div class="flex justify-between gap-3">
            <dt class="text-muted">{{ t('console.accountEmail') }}</dt>
            <dd class="truncate text-ink">{{ account?.email }}</dd>
          </div>
          <div class="flex justify-between gap-3">
            <dt class="text-muted">{{ t('console.accountRole') }}</dt>
            <dd class="text-ink">{{ account?.role }}</dd>
          </div>
        </dl>

        <div class="flex items-center gap-4">
          <img
            v-if="avatarUrl"
            :src="avatarUrl"
            :alt="t('console.avatarPreview')"
            class="size-14 shrink-0 rounded-full border border-line object-cover"
          >
          <span
            v-else
            class="flex size-14 shrink-0 items-center justify-center rounded-full bg-sunken text-faint"
          >
            <UserRound class="size-5" />
          </span>

          <div class="flex flex-wrap gap-2">
            <UiButton variant="secondary" size="sm" @click="pickAvatar">{{ t('console.avatarPick') }}</UiButton>
            <UiButton v-if="avatarUrl" variant="ghost" size="sm" @click="avatarUrl = ''">
              {{ t('console.avatarClear') }}
            </UiButton>
          </div>

          <input
            ref="fileInput"
            type="file"
            accept="image/png,image/jpeg,image/gif,image/webp"
            class="hidden"
            :aria-label="t('console.avatarPick')"
            @change="onAvatarSelected"
          >
        </div>

        <UiField
          :label="t('console.avatarUrl')"
          :hint="t('console.avatarUrlHint')"
          :error="avatarError"
        >
          <UiTextarea
            v-model="avatarUrl"
            mono
            :rows="3"
            :placeholder="t('console.avatarUrlPlaceholder')"
          />
        </UiField>
      </div>

      <template #footer>
        <UiButton size="sm" :loading="profileAction.busy.value" :disabled="locked" @click="saveProfile">
          {{ t('common.save') }}
        </UiButton>
        <p v-if="locked" class="mt-2 text-[13px] text-muted">{{ t('console.lockedUntilPasswordChanged') }}</p>
      </template>
    </UiCard>

    <UiCard
      :title="t('console.passwordTitle')"
      :description="t('console.passwordDescription')"
      :class="locked && 'order-1'"
    >
      <form class="space-y-3" @submit.prevent="savePassword">
        <UiField :label="t('console.currentPassword')" required>
          <UiInput v-model="password.current" type="password" autocomplete="current-password" />
        </UiField>

        <UiField :label="t('console.newPassword')" required>
          <UiInput v-model="password.next" type="password" autocomplete="new-password" />
        </UiField>

        <UiField :label="t('console.confirmPassword')" required :error="passwordError">
          <UiInput v-model="password.confirm" type="password" autocomplete="new-password" />
        </UiField>
      </form>

      <template #footer>
        <UiButton size="sm" :loading="passwordAction.busy.value" @click="savePassword">
          {{ t('console.changePassword') }}
        </UiButton>
      </template>
    </UiCard>

    <UiCard
      :title="t('console.preferencesTitle')"
      :description="t('console.preferencesDescription')"
      :class="locked && 'order-3'"
    >
      <div class="space-y-4">
        <div class="flex items-start justify-between gap-4">
          <div class="min-w-0">
            <p class="text-sm text-ink">{{ t('console.dataUsageEnabled') }}</p>
            <p class="text-[13px] text-muted">{{ t('console.dataUsageEnabledHint') }}</p>
          </div>
          <UiSwitch v-model="dataUsage" :label="t('console.dataUsageEnabled')" />
        </div>

        <div class="flex items-start justify-between gap-4">
          <div class="min-w-0">
            <p class="text-sm text-ink">{{ t('console.leaderboardOptIn') }}</p>
            <p class="text-[13px] text-muted">{{ t('console.leaderboardOptInHint') }}</p>
          </div>
          <UiSwitch v-model="optIn" :label="t('console.leaderboardOptIn')" />
        </div>

        <div class="flex items-start justify-between gap-4">
          <div class="min-w-0">
            <p class="text-sm text-ink">{{ t('console.leaderboardMaskName') }}</p>
            <p class="text-[13px] text-muted">{{ t('console.leaderboardMaskNameHint') }}</p>
          </div>
          <UiSwitch v-model="maskName" :disabled="!optIn" :label="t('console.leaderboardMaskName')" />
        </div>
      </div>

      <template #footer>
        <UiButton size="sm" :loading="preferencesAction.busy.value" :disabled="locked" @click="savePreferences">
          {{ t('common.save') }}
        </UiButton>
        <p v-if="locked" class="mt-2 text-[13px] text-muted">{{ t('console.lockedUntilPasswordChanged') }}</p>
      </template>
    </UiCard>

    <UiCard
      :title="t('console.oauthConnections')"
      :description="t('console.oauthConnectionsHint')"
      :class="locked && 'order-4'"
    >
      <div v-if="!connections.data.value.length" class="py-4 text-center text-sm text-muted">
        {{ t('console.oauthNoConnections') }}
      </div>
      <div v-else class="space-y-2">
        <div
          v-for="conn in connections.data.value"
          :key="conn.provider"
          class="flex items-center justify-between rounded-control border border-line px-3 py-2"
        >
          <div class="flex items-center gap-2">
            <Link class="size-4 text-muted" />
            <span class="text-sm font-medium text-ink">{{ conn.provider === 'github' ? 'GitHub' : conn.provider }}</span>
            <span class="text-xs text-muted">({{ conn.provider_username }})</span>
          </div>
          <UiButton variant="ghost" size="sm" :loading="unlinkAction.busy.value" @click="unlink(conn.provider)">
            {{ t('console.oauthUnlink') }}
          </UiButton>
        </div>
      </div>
    </UiCard>
  </div>
</template>

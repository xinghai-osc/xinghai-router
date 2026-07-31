<script setup lang="ts">
import { ImageOff, Trash2 } from 'lucide-vue-next'
import { endpoints, type AdminSiteSettings, type OAuthProvider } from '~/src/api'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { settings: site } = useSiteSettings()
const { toast } = useToast()
const { busy, run } = useAction()

useHead({ title: () => `${t('system.siteSettingsTitle')} · ${site.value.name}` })

const EMPTY: AdminSiteSettings = {
  name: '',
  icon_url: '',
  auto_disable_failed_channels: false,
  geetest_captcha_id: '',
  has_geetest_captcha_key: false,
  smtp_host: '',
  smtp_port: '',
  smtp_username: '',
  has_smtp_password: false,
  smtp_from: '',
}

const { data, pending, error, refresh } = useResource(
  () => endpoints.getAdminSiteSettings(),
  { ...EMPTY },
)

const { data: oauthProviders, refresh: refreshOAuth } = useResource(
  () => endpoints.getOAuthProviders(),
  { data: [] as OAuthProvider[] },
)

const oauthFormOpen = ref(false)
const oauthFormError = ref('')
const editingOAuth = ref('')
const oauthForm = reactive({
  client_id: '',
  client_secret: '',
  enabled: true,
})

function openOAuthForm(provider: OAuthProvider | null) {
  editingOAuth.value = provider?.id ?? ''
  oauthFormError.value = ''
  oauthForm.client_id = provider?.client_id ?? ''
  oauthForm.client_secret = ''
  oauthForm.enabled = provider?.enabled ?? true
  oauthFormOpen.value = true
}

async function saveOAuth() {
  oauthFormError.value = ''
  const provider = editingOAuth.value
  if (!provider) { oauthFormError.value = 'Provider not set'; return }
  if (!oauthForm.client_id.trim()) { oauthFormError.value = t('system.oauthClientIdRequired'); return }
  const ok = await run(() => endpoints.saveOAuthProvider(provider, { ...oauthForm }))
  if (!ok) { oauthFormError.value = t('common.actionFailed'); return }
  oauthFormOpen.value = false
  toast.success(t('system.oauthProviderSaved'))
  await refreshOAuth()
}

async function deleteOAuth(provider: string) {
  if (!confirm(t('system.oauthDeleteConfirm'))) return
  const ok = await run(() => endpoints.deleteOAuthProvider(provider))
  if (!ok) { toast.error(t('common.actionFailed')); return }
  toast.success(t('system.oauthProviderDeleted'))
  await refreshOAuth()
}

const form = reactive({
  name: '',
  icon_url: '',
  auto_disable_failed_channels: false,
  geetest_captcha_id: '',
  smtp_host: '',
  smtp_port: '',
  smtp_username: '',
  smtp_from: '',
})

/** Write-only: never seeded from the API, cleared again after every save. */
const geetestKey = ref('')
const smtpPassword = ref('')

const iconBroken = ref(false)

watch(data, (next) => {
  form.name = next.name
  form.icon_url = next.icon_url
  form.auto_disable_failed_channels = next.auto_disable_failed_channels
  form.geetest_captcha_id = next.geetest_captcha_id
  form.smtp_host = next.smtp_host
  form.smtp_port = next.smtp_port
  form.smtp_username = next.smtp_username
  form.smtp_from = next.smtp_from
  geetestKey.value = ''
  smtpPassword.value = ''
}, { immediate: true })

watch(() => form.icon_url, () => { iconBroken.value = false })

const iconPreviewUrl = computed(() => form.icon_url.trim())

type SiteSettingsPayload = AdminSiteSettings & { geetest_captcha_key: string; smtp_password: string }

async function save() {
  // The handler rejects unknown fields, so the read-only `has_*` flags that the
  // GET response carries must be left out of the body.
  const payload = {
    name: form.name.trim(),
    icon_url: form.icon_url.trim(),
    auto_disable_failed_channels: form.auto_disable_failed_channels,
    geetest_captcha_id: form.geetest_captcha_id.trim(),
    geetest_captcha_key: geetestKey.value,
    smtp_host: form.smtp_host.trim(),
    smtp_port: form.smtp_port.trim(),
    smtp_username: form.smtp_username.trim(),
    smtp_password: smtpPassword.value,
    smtp_from: form.smtp_from.trim(),
  } as SiteSettingsPayload

  const ok = await run(() => endpoints.updateAdminSiteSettings(payload))
  if (!ok) {
    toast.error(t('common.actionFailed'))
    return
  }
  geetestKey.value = ''
  smtpPassword.value = ''
  toast.success(t('system.siteSettingsSaved'))
  await refresh()
}
</script>

<template>
  <ConsoleSystemGate permission="system.manage">
    <div class="space-y-4">
      <div class="min-w-0 space-y-1">
        <h2 class="text-lg font-semibold text-ink">{{ t('system.siteSettingsTitle') }}</h2>
        <p class="text-[13px] text-muted">{{ t('system.siteSettingsLead') }}</p>
      </div>

      <UiAlert v-if="error" tone="danger" :title="t('common.loadFailed')">{{ error }}</UiAlert>

      <div v-else-if="pending" class="space-y-4">
        <UiCard>
          <UiSkeleton :rows="3" class="h-10" />
        </UiCard>
        <UiCard>
          <UiSkeleton :rows="4" class="h-10" />
        </UiCard>
      </div>

      <form v-else class="space-y-4" @submit.prevent="save">
        <UiCard :title="t('system.basicSection')">
          <div class="space-y-4">
            <div class="grid gap-4 sm:grid-cols-2">
              <UiField :label="t('system.siteName')" :hint="t('system.siteNameHint')" required for="site-name">
                <UiInput id="site-name" v-model="form.name" />
              </UiField>

              <UiField :label="t('system.siteIcon')" :hint="t('system.siteIconHint')" for="site-icon">
                <UiInput id="site-icon" v-model="form.icon_url" type="url" />
              </UiField>
            </div>

            <div class="flex items-center gap-3 rounded-control border border-line bg-sunken px-3 py-2.5">
              <div class="flex size-10 shrink-0 items-center justify-center overflow-hidden rounded-control border border-line bg-surface text-faint">
                <img
                  v-if="iconPreviewUrl && !iconBroken"
                  :src="iconPreviewUrl"
                  :alt="t('system.iconPreview')"
                  class="size-full object-contain"
                  @error="iconBroken = true"
                >
                <ImageOff v-else class="size-4" />
              </div>
              <div class="min-w-0">
                <p class="text-[13px] font-medium text-ink">{{ t('system.iconPreview') }}</p>
                <p class="mt-0.5 truncate text-[13px] text-muted">
                  {{ iconPreviewUrl || t('system.iconPreviewEmpty') }}
                </p>
              </div>
            </div>

            <UiField
              :label="t('system.autoDisableFailedChannels')"
              :hint="t('system.autoDisableFailedChannelsHint')"
            >
              <UiSwitch
                v-model="form.auto_disable_failed_channels"
                :label="t('system.autoDisableFailedChannels')"
              />
            </UiField>
          </div>
        </UiCard>

        <UiCard :title="t('system.captcha')">
          <div class="grid gap-4 sm:grid-cols-2">
            <UiField :label="t('system.geetestId')" :hint="t('system.geetestIdHint')" for="geetest-id">
              <UiInput id="geetest-id" v-model="form.geetest_captcha_id" mono />
            </UiField>

            <ConsoleSystemSecretField
              id="geetest-key"
              v-model="geetestKey"
              :label="t('system.geetestKey')"
              :configured="data.has_geetest_captcha_key"
            />
          </div>
        </UiCard>

        <UiCard :title="t('system.smtp')">
          <div class="grid gap-4 sm:grid-cols-2">
            <UiField :label="t('system.smtpHost')" :hint="t('system.smtpHostHint')" for="smtp-host">
              <UiInput id="smtp-host" v-model="form.smtp_host" />
            </UiField>

            <UiField :label="t('system.smtpPort')" :hint="t('system.smtpPortHint')" for="smtp-port">
              <UiInput id="smtp-port" v-model="form.smtp_port" />
            </UiField>

            <UiField :label="t('system.smtpUsername')" :hint="t('system.smtpUsernameHint')" for="smtp-username">
              <UiInput id="smtp-username" v-model="form.smtp_username" autocomplete="off" />
            </UiField>

            <ConsoleSystemSecretField
              id="smtp-password"
              v-model="smtpPassword"
              :label="t('system.smtpPassword')"
              :configured="data.has_smtp_password"
            />

            <UiField :label="t('system.smtpFrom')" :hint="t('system.smtpFromHint')" for="smtp-from">
              <UiInput id="smtp-from" v-model="form.smtp_from" type="email" />
            </UiField>
          </div>
        </UiCard>

        <UiCard :title="t('system.oauthSection')" :description="t('system.oauthSectionHint')">
          <div class="space-y-3">
            <div v-if="!oauthProviders.data.value.data.length" class="text-sm text-muted py-2">
              {{ t('system.oauthNoProviders') }}
            </div>
            <div
              v-for="provider in oauthProviders.data.value.data"
              :key="provider.id"
              class="flex items-center justify-between rounded-control border border-line px-3 py-2"
            >
              <div class="flex items-center gap-2">
                <span class="text-sm font-medium text-ink">{{ provider.id }}</span>
                <UiBadge :tone="provider.enabled ? 'success' : 'neutral'" dot>
                  {{ provider.enabled ? t('common.enabled') : t('common.disabled') }}
                </UiBadge>
              </div>
              <div class="flex items-center gap-1">
                <UiButton variant="ghost" size="sm" @click="openOAuthForm(provider)">{{ t('common.edit') }}</UiButton>
                <UiButton variant="danger" size="sm" @click="deleteOAuth(provider.id)">
                  <Trash2 class="size-4" />
                </UiButton>
              </div>
            </div>
            <UiButton variant="secondary" size="sm" @click="openOAuthForm(null)">{{ t('system.oauthAddProvider') }}</UiButton>
          </div>
        </UiCard>

        <div class="flex justify-end">
          <UiButton type="submit" :loading="busy">{{ t('common.save') }}</UiButton>
        </div>
      </form>
    </div>

    <UiSlidePanel
      v-model:open="oauthFormOpen"
      size="sm"
      :title="editingOAuth ? t('system.oauthEditProvider', { provider: editingOAuth }) : t('system.oauthAddProvider')"
    >
      <div class="space-y-4">
        <UiAlert v-if="oauthFormError" tone="danger">{{ oauthFormError }}</UiAlert>

        <UiField :label="t('system.oauthProviderId')" required>
          <UiInput v-model="editingOAuth" :disabled="!!editingOAuth" mono :placeholder="t('system.oauthProviderIdPlaceholder')" />
        </UiField>

        <UiField :label="t('system.oauthClientId')" required>
          <UiInput v-model="oauthForm.client_id" mono />
        </UiField>

        <UiField :label="t('system.oauthClientSecret')" :hint="t('system.oauthClientSecretHint')">
          <UiInput v-model="oauthForm.client_secret" type="password" mono autocomplete="off" />
        </UiField>

        <UiCheckbox v-model="oauthForm.enabled">{{ t('system.oauthEnabled') }}</UiCheckbox>
      </div>

      <template #footer>
        <UiButton variant="secondary" @click="oauthFormOpen = false">{{ t('common.cancel') }}</UiButton>
        <UiButton :loading="busy" @click="saveOAuth">{{ t('common.save') }}</UiButton>
      </template>
    </UiSlidePanel>
  </ConsoleSystemGate>
</template>

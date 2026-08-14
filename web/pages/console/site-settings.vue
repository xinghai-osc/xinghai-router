<script setup lang="ts">
import { ImageOff } from 'lucide-vue-next'
import { endpoints, type AdminSiteSettings } from '~/src/api'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { settings: site } = useSiteSettings()
const { toast } = useToast()
const { busy, run } = useAction()

useHead({ title: () => `${t('system.siteSettingsTitle')} · ${site.value.name}` })

const EMPTY: AdminSiteSettings = {
  name: '',
  icon_url: '',
  announcement: '',
  auto_disable_failed_channels: false,
  captcha_provider: '',
  geetest_captcha_id: '',
  has_geetest_captcha_key: false,
  corptcha_site_id: '',
  has_corptcha_secret: false,
  smtp_host: '',
  smtp_port: '',
  smtp_username: '',
  has_smtp_password: false,
  smtp_from: '',
  public_base_url: '',
  invitations_enabled: false,
  inviter_reward: '0',
  invitee_reward: '0',
}

const { data, pending, error, refresh } = useResource(
  () => endpoints.getAdminSiteSettings(),
  { ...EMPTY },
)

const form = reactive({
  name: '',
  icon_url: '',
  announcement: '',
  auto_disable_failed_channels: false,
  captcha_provider: '',
  geetest_captcha_id: '',
  corptcha_site_id: '',
  smtp_host: '',
  smtp_port: '',
  smtp_username: '',
  smtp_from: '',
  public_base_url: '',
  invitations_enabled: false,
  inviter_reward: '0',
  invitee_reward: '0',
})

/** Write-only: never seeded from the API, cleared again after every save. */
const geetestKey = ref('')
const corptchaSecret = ref('')
const smtpPassword = ref('')

const iconBroken = ref(false)

const captchaProviderOptions = [
  { value: '', label: t('system.captchaProviderAuto') },
  { value: 'geetest', label: t('system.captchaProviderGeetest') },
  { value: 'corptcha', label: t('system.captchaProviderCorptcha') },
]

watch(data, (next) => {
  form.name = next.name
  form.icon_url = next.icon_url
  form.announcement = next.announcement
  form.auto_disable_failed_channels = next.auto_disable_failed_channels
  form.captcha_provider = next.captcha_provider
  form.geetest_captcha_id = next.geetest_captcha_id
  form.corptcha_site_id = next.corptcha_site_id
  form.smtp_host = next.smtp_host
  form.smtp_port = next.smtp_port
  form.smtp_username = next.smtp_username
  form.smtp_from = next.smtp_from
  form.public_base_url = next.public_base_url
  form.invitations_enabled = next.invitations_enabled
  form.inviter_reward = next.inviter_reward
  form.invitee_reward = next.invitee_reward
  geetestKey.value = ''
  corptchaSecret.value = ''
  smtpPassword.value = ''
}, { immediate: true })

watch(() => form.icon_url, () => { iconBroken.value = false })

const iconPreviewUrl = computed(() => form.icon_url.trim())

type SiteSettingsPayload = Omit<AdminSiteSettings, 'inviter_reward' | 'invitee_reward'> & { inviter_reward: number; invitee_reward: number; geetest_captcha_key: string; corptcha_secret: string; smtp_password: string }

async function save() {
  // The handler rejects unknown fields, so the read-only `has_*` flags that the
  // GET response carries must be left out of the body.
  const payload = {
    name: form.name.trim(),
    icon_url: form.icon_url.trim(),
    announcement: form.announcement.trim(),
    auto_disable_failed_channels: form.auto_disable_failed_channels,
    captcha_provider: form.captcha_provider,
    geetest_captcha_id: form.geetest_captcha_id.trim(),
    geetest_captcha_key: geetestKey.value,
    corptcha_site_id: form.corptcha_site_id.trim(),
    corptcha_secret: corptchaSecret.value,
    smtp_host: form.smtp_host.trim(),
    smtp_port: form.smtp_port.trim(),
    smtp_username: form.smtp_username.trim(),
    smtp_password: smtpPassword.value,
    smtp_from: form.smtp_from.trim(),
    public_base_url: form.public_base_url.trim(),
    invitations_enabled: form.invitations_enabled,
    inviter_reward: Number(form.inviter_reward),
    invitee_reward: Number(form.invitee_reward),
  } as SiteSettingsPayload

  const ok = await run(() => endpoints.updateAdminSiteSettings(payload))
  if (!ok) {
    toast.error(t('common.actionFailed'))
    return
  }
  geetestKey.value = ''
  corptchaSecret.value = ''
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

              <UiField :label="t('system.announcement')" :hint="t('system.announcementHint')" for="site-announcement" class="sm:col-span-2">
                <UiTextarea id="site-announcement" v-model="form.announcement" :maxlength="2000" :rows="3" />
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

            <UiField
              :label="t('system.publicBaseUrl')"
              :hint="t('system.publicBaseUrlHint')"
              for="public-base-url"
            >
              <UiInput id="public-base-url" v-model="form.public_base_url" type="url" />
            </UiField>
          </div>
        </UiCard>

        <UiCard :title="t('system.captcha')">
          <div class="space-y-4">
            <UiField :label="t('system.captchaProvider')" :hint="t('system.captchaProviderHint')" for="captcha-provider">
              <UiSelect id="captcha-provider" v-model="form.captcha_provider" :options="captchaProviderOptions" />
            </UiField>

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

              <UiField :label="t('system.corptchaSiteId')" :hint="t('system.corptchaSiteIdHint')" for="corptcha-site-id">
                <UiInput id="corptcha-site-id" v-model="form.corptcha_site_id" mono />
              </UiField>

              <ConsoleSystemSecretField
                id="corptcha-secret"
                v-model="corptchaSecret"
                :label="t('system.corptchaSecret')"
                :configured="data.has_corptcha_secret"
              />
            </div>
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

        <UiCard :title="t('system.invitations')">
          <div class="space-y-4">
            <UiField :label="t('system.invitationsEnabled')" :hint="t('system.invitationsEnabledHint')">
              <UiSwitch v-model="form.invitations_enabled" :label="t('system.invitationsEnabled')" />
            </UiField>
            <div class="grid gap-4 sm:grid-cols-2">
              <UiField :label="t('system.inviterReward')" :hint="t('system.inviterRewardHint')" for="inviter-reward">
                <UiInput id="inviter-reward" v-model="form.inviter_reward" type="number" min="0" step="0.00000001" />
              </UiField>
              <UiField :label="t('system.inviteeReward')" :hint="t('system.inviteeRewardHint')" for="invitee-reward">
                <UiInput id="invitee-reward" v-model="form.invitee_reward" type="number" min="0" step="0.00000001" />
              </UiField>
            </div>
          </div>
        </UiCard>

        <div class="flex justify-end">
          <UiButton type="submit" :loading="busy">{{ t('common.save') }}</UiButton>
        </div>
      </form>
    </div>
  </ConsoleSystemGate>
</template>

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
  auto_disable_failed_channels: false,
  geetest_captcha_id: '',
  has_geetest_captcha_key: false,
  smtp_host: '',
  smtp_port: '',
  smtp_username: '',
  has_smtp_password: false,
  smtp_from: '',
  public_base_url: '',
}

const { data, pending, error, refresh } = useResource(
  () => endpoints.getAdminSiteSettings(),
  { ...EMPTY },
)

const form = reactive({
  name: '',
  icon_url: '',
  auto_disable_failed_channels: false,
  geetest_captcha_id: '',
  smtp_host: '',
  smtp_port: '',
  smtp_username: '',
  smtp_from: '',
  public_base_url: '',
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
  form.public_base_url = next.public_base_url
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
    public_base_url: form.public_base_url.trim(),
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

        <div class="flex justify-end">
          <UiButton type="submit" :loading="busy">{{ t('common.save') }}</UiButton>
        </div>
      </form>
    </div>
  </ConsoleSystemGate>
</template>

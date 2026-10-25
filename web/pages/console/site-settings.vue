<script setup lang="ts">
import { clearPublicCache, endpoints, type AdminSiteSettings, type FeaturedCopy } from '~/src/api'

definePageMeta({ layout: 'console', middleware: 'console-auth' })
const { t } = useI18n()
function emptyFeaturedCopy(): FeaturedCopy { return { zh: { badge: '', title: '', body: '', cta: '' }, 'zh-Hant': { badge: '', title: '', body: '', cta: '' }, en: { badge: '', title: '', body: '', cta: '' } } }
const { settings: site } = useSiteSettings(); const { toast } = useToast(); const { busy, run } = useAction()
useHead({ title: () => `${t('system.siteSettingsTitle')} · ${site.value.name}` })
const EMPTY: AdminSiteSettings = { name: '', icon_url: '', announcement: '', homepage_video_url: '', contact_email: '', api_base_urls: [], auto_disable_failed_channels: false, maintenance_mode: false, featured_enabled: true, featured_model: '', featured_copy: emptyFeaturedCopy(), captcha_provider: '', geetest_captcha_id: '', has_geetest_captcha_key: false, corptcha_site_id: '', has_corptcha_secret: false, smtp_host: '', smtp_port: '', smtp_username: '', has_smtp_password: false, smtp_from: '', public_base_url: '', invitations_enabled: false, inviter_reward: '0', invitee_reward: '0', checkin_base_reward: '1', checkin_streak_bonus: '0.1', checkin_max_bonus_days: 7, registration_email_whitelist_enabled: false, registration_email_whitelist: [], registration_email_alias_blocked: false }
const { data, pending, error, refresh } = useResource(() => endpoints.getAdminSiteSettings(), { ...EMPTY })
const form = reactive({ name: '', icon_url: '', announcement: '', homepage_video_url: '', contact_email: '', maintenance_mode: false, auto_disable_failed_channels: false, api_base_urls: '', captcha_provider: '', geetest_captcha_id: '', corptcha_site_id: '', smtp_host: '', smtp_port: '', smtp_username: '', smtp_from: '', public_base_url: '', invitations_enabled: false, inviter_reward: '0', invitee_reward: '0', checkin_base_reward: '1', checkin_streak_bonus: '0.1', checkin_max_bonus_days: 7, registration_email_whitelist_enabled: false, registration_email_whitelist: '', registration_email_alias_blocked: false, featured_enabled: true, featured_model: '', featured_copy: emptyFeaturedCopy() })
const geetestKey = ref(''); const corptchaSecret = ref(''); const smtpPassword = ref('')
watch(data, next => { Object.assign(form, { name: next.name, icon_url: next.icon_url, announcement: next.announcement, homepage_video_url: next.homepage_video_url ?? '', contact_email: next.contact_email, maintenance_mode: next.maintenance_mode, api_base_urls: (next.api_base_urls ?? []).join('\n'), auto_disable_failed_channels: next.auto_disable_failed_channels, captcha_provider: next.captcha_provider, geetest_captcha_id: next.geetest_captcha_id, corptcha_site_id: next.corptcha_site_id, smtp_host: next.smtp_host, smtp_port: next.smtp_port, smtp_username: next.smtp_username, smtp_from: next.smtp_from, public_base_url: next.public_base_url, invitations_enabled: next.invitations_enabled, inviter_reward: next.inviter_reward, invitee_reward: next.invitee_reward, checkin_base_reward: next.checkin_base_reward, checkin_streak_bonus: next.checkin_streak_bonus, checkin_max_bonus_days: next.checkin_max_bonus_days, registration_email_whitelist_enabled: next.registration_email_whitelist_enabled, registration_email_whitelist: next.registration_email_whitelist.join('\n'), registration_email_alias_blocked: next.registration_email_alias_blocked, featured_enabled: next.featured_enabled ?? true, featured_model: next.featured_model ?? '', featured_copy: JSON.parse(JSON.stringify(next.featured_copy ?? emptyFeaturedCopy())) }); geetestKey.value = ''; corptchaSecret.value = ''; smtpPassword.value = '' }, { immediate: true })
async function save() { const payload = { ...form, api_base_urls: form.api_base_urls.split(/\r?\n/), registration_email_whitelist: form.registration_email_whitelist.split(/\r?\n/), inviter_reward: Number(form.inviter_reward), invitee_reward: Number(form.invitee_reward), checkin_base_reward: Number(form.checkin_base_reward), checkin_streak_bonus: Number(form.checkin_streak_bonus), geetest_captcha_key: geetestKey.value, corptcha_secret: corptchaSecret.value, smtp_password: smtpPassword.value, featured_copy: form.featured_copy } as never; const ok = await run(() => endpoints.updateAdminSiteSettings(payload)); if (!ok) { toast.error(t('common.actionFailed')); return }; clearPublicCache('site-settings'); toast.success(t('system.siteSettingsSaved')); await refresh() }
</script>

<template>
  <ConsoleSystemGate permission="system.manage">
    <div class="space-y-4">
      <div>
        <h2 class="text-lg font-semibold text-ink">{{ t('system.siteSettingsTitle') }}</h2>
        <p class="mt-1 text-[13px] text-muted">{{ t('system.siteSettingsLead') }}</p>
      </div>
      <UiAlert v-if="error" tone="danger" :title="t('common.loadFailed')">{{ error }}</UiAlert>
      <div v-else-if="pending"><UiSkeleton :rows="5" /></div>
      <form v-else class="space-y-4" @submit.prevent="save">
        <UiCard :title="t('system.basicSection')">
          <div class="grid gap-4 md:grid-cols-2">
            <UiField :label="t('system.siteName')" :hint="t('system.siteNameHint')" required><UiInput v-model="form.name" /></UiField>
            <UiField :label="t('system.siteIcon')" :hint="t('system.siteIconHint')"><UiInput v-model="form.icon_url" type="url" /></UiField>
            <UiField :label="t('system.homepageVideo')" :hint="t('system.homepageVideoHint')"><UiInput v-model="form.homepage_video_url" type="url" /></UiField>
            <UiField :label="t('system.contactEmail')" :hint="t('system.contactEmailHint')"><UiInput v-model="form.contact_email" type="email" /></UiField>
            <UiField :label="t('system.announcement')" :hint="t('system.announcementHint')" class="md:col-span-2"><UiTextarea v-model="form.announcement" :rows="3" /></UiField>
            <UiField :label="t('system.maintenanceMode')" :hint="t('system.maintenanceModeHint')"><UiSwitch v-model="form.maintenance_mode" :label="t('system.maintenanceMode')" /></UiField>
            <UiField :label="t('system.autoDisableFailedChannels')" :hint="t('system.autoDisableFailedChannelsHint')"><UiSwitch v-model="form.auto_disable_failed_channels" :label="t('system.autoDisableFailedChannels')" /></UiField>
            <UiField :label="t('system.apiBaseUrls')" :hint="t('system.apiBaseUrlsHint')" class="md:col-span-2"><UiTextarea v-model="form.api_base_urls" :placeholder="t('system.apiBaseUrlsPlaceholder')" :rows="3" /></UiField>
            <UiField :label="t('system.publicBaseUrl')" :hint="t('system.publicBaseUrlHint')" class="md:col-span-2"><UiInput v-model="form.public_base_url" type="url" /></UiField>
          </div>
        </UiCard>

        <UiCard :title="t('system.captcha')">
          <div class="grid gap-4 md:grid-cols-2">
            <UiField :label="t('system.captchaProvider')" :hint="t('system.captchaProviderHint')"><UiSelect v-model="form.captcha_provider" :options="[{ value: '', label: t('system.captchaProviderAuto') }, { value: 'geetest', label: t('system.captchaProviderGeetest') }, { value: 'corptcha', label: t('system.captchaProviderCorptcha') }]" /></UiField>
            <UiField :label="t('system.geetestId')" :hint="t('system.geetestIdHint')"><UiInput v-model="form.geetest_captcha_id" /></UiField>
            <UiField :label="t('system.geetestKey')" :hint="`${t('system.secretWriteOnlyHint')} ${data.has_geetest_captcha_key ? t('system.configured') : t('system.notConfigured')}`"><UiInput v-model="geetestKey" type="password" autocomplete="new-password" /></UiField>
            <UiField :label="t('system.corptchaSiteId')" :hint="t('system.corptchaSiteIdHint')"><UiInput v-model="form.corptcha_site_id" /></UiField>
            <UiField :label="t('system.corptchaSecret')" :hint="`${t('system.secretWriteOnlyHint')} ${data.has_corptcha_secret ? t('system.configured') : t('system.notConfigured')}`"><UiInput v-model="corptchaSecret" type="password" autocomplete="new-password" /></UiField>
          </div>
        </UiCard>

        <UiCard :title="t('system.smtp')">
          <div class="grid gap-4 md:grid-cols-2">
            <UiField :label="t('system.smtpHost')" :hint="t('system.smtpHostHint')"><UiInput v-model="form.smtp_host" /></UiField>
            <UiField :label="t('system.smtpPort')" :hint="t('system.smtpPortHint')"><UiInput v-model="form.smtp_port" inputmode="numeric" /></UiField>
            <UiField :label="t('system.smtpUsername')" :hint="t('system.smtpUsernameHint')"><UiInput v-model="form.smtp_username" autocomplete="username" /></UiField>
            <UiField :label="t('system.smtpPassword')" :hint="`${t('system.secretWriteOnlyHint')} ${data.has_smtp_password ? t('system.configured') : t('system.notConfigured')}`"><UiInput v-model="smtpPassword" type="password" autocomplete="new-password" /></UiField>
            <UiField :label="t('system.smtpFrom')" :hint="t('system.smtpFromHint')"><UiInput v-model="form.smtp_from" type="email" /></UiField>
          </div>
        </UiCard>

        <UiCard :title="t('system.registrationWhitelist')">
          <div class="space-y-4">
            <UiField :label="t('system.registrationWhitelistEnabled')" :hint="t('system.registrationWhitelistEnabledHint')"><UiSwitch v-model="form.registration_email_whitelist_enabled" :label="t('system.registrationWhitelistEnabled')" /></UiField>
            <UiField :label="t('system.registrationWhitelistEmails')" :hint="t('system.registrationWhitelistEmailsHint')"><UiTextarea v-model="form.registration_email_whitelist" :rows="4" /></UiField>
            <UiField :label="t('system.registrationAliasBlocked')" :hint="t('system.registrationAliasBlockedHint')"><UiSwitch v-model="form.registration_email_alias_blocked" :label="t('system.registrationAliasBlocked')" /></UiField>
          </div>
        </UiCard>

        <UiCard :title="t('system.invitations')">
          <div class="space-y-4">
            <UiField :label="t('system.invitationsEnabled')" :hint="t('system.invitationsEnabledHint')"><UiSwitch v-model="form.invitations_enabled" :label="t('system.invitationsEnabled')" /></UiField>
            <div class="grid gap-4 md:grid-cols-2">
              <UiField :label="t('system.inviterReward')" :hint="t('system.inviterRewardHint')"><UiInput v-model="form.inviter_reward" type="number" min="0" step="any" /></UiField>
              <UiField :label="t('system.inviteeReward')" :hint="t('system.inviteeRewardHint')"><UiInput v-model="form.invitee_reward" type="number" min="0" step="any" /></UiField>
            </div>
          </div>
        </UiCard>

        <UiCard :title="t('system.checkinRewards')">
          <div class="grid gap-4 md:grid-cols-3">
            <UiField :label="t('system.checkinBaseReward')" :hint="t('system.checkinBaseRewardHint')"><UiInput v-model="form.checkin_base_reward" type="number" min="0" step="any" /></UiField>
            <UiField :label="t('system.checkinStreakBonus')" :hint="t('system.checkinStreakBonusHint')"><UiInput v-model="form.checkin_streak_bonus" type="number" min="0" step="any" /></UiField>
            <UiField :label="t('system.checkinMaxBonusDays')" :hint="t('system.checkinMaxBonusDaysHint')"><UiInput v-model="form.checkin_max_bonus_days" type="number" min="1" max="365" step="1" /></UiField>
          </div>
        </UiCard>

        <UiCard :title="t('system.featuredSectionTitle')" :description="t('system.featuredSectionLead')">
          <div class="space-y-4">
            <UiField :label="t('system.featuredEnabled')" :hint="t('system.featuredEnabledHint')"><UiSwitch v-model="form.featured_enabled" :label="t('system.featuredEnabled')" /></UiField>
            <UiField :label="t('system.featuredModel')" :hint="t('system.featuredModelHint')"><UiInput v-model="form.featured_model" :placeholder="t('system.featuredModelAutomatic')" /></UiField>
            <div v-for="locale in [{ code: 'zh', label: t('system.featuredLocaleZh') }, { code: 'zh-Hant', label: t('system.featuredLocaleHant') }, { code: 'en', label: t('system.featuredLocaleEn') }]" :key="locale.code" class="space-y-3 border-t border-line pt-4">
              <h3 class="text-sm font-semibold text-ink">{{ locale.label }}</h3>
              <div class="grid gap-4 md:grid-cols-2">
                <UiField :label="t('system.featuredBadge')"><UiInput v-model="form.featured_copy[locale.code].badge" /></UiField>
                <UiField :label="t('system.featuredTitle')"><UiInput v-model="form.featured_copy[locale.code].title" /></UiField>
                <UiField :label="t('system.featuredBody')" class="md:col-span-2"><UiTextarea v-model="form.featured_copy[locale.code].body" :rows="2" /></UiField>
                <UiField :label="t('system.featuredCta')"><UiInput v-model="form.featured_copy[locale.code].cta" /></UiField>
              </div>
            </div>
          </div>
        </UiCard>

        <div class="flex justify-end"><UiButton type="submit" :loading="busy">{{ t('common.save') }}</UiButton></div>
      </form>
    </div>
  </ConsoleSystemGate>
</template>

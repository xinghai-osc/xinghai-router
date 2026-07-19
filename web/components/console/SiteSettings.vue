<script setup lang="ts">
import { useConsoleStore } from '~/composables/useConsoleStore'

const store = useConsoleStore()
const { t, busy, siteSettingsForm, saveSiteSettings } = store
</script>

<template>
  <section class="toolbar">
    <div>
      <h2>{{ t('siteSettings') }}</h2>
      <p>{{ t('siteSettingsDesc') }}</p>
    </div>
  </section>
  <form class="panel pricing-form" @submit.prevent="saveSiteSettings">
    <label>{{ t('siteName') }}<input v-model="siteSettingsForm.name" required maxlength="100" /></label>
    <label>{{ t('siteIconUrl') }}<input v-model="siteSettingsForm.icon_url" type="url" placeholder="https://example.com/favicon.png" /></label>
    <label><input v-model="siteSettingsForm.auto_disable_failed_channels" type="checkbox" /> Auto-disable after 3 consecutive failures</label>
    <h3 class="reliability-section-title">{{ t('geetestSettings') }}</h3>
    <label>{{ t('geetestCaptchaId') }}<input v-model="siteSettingsForm.geetest_captcha_id" maxlength="64" placeholder="captcha_id" /><small>{{ t('geetestHint') }}</small></label>
    <label>{{ t('geetestCaptchaKey') }}<input v-model="siteSettingsForm.geetest_captcha_key" type="password" maxlength="64" autocomplete="new-password" :placeholder="siteSettingsForm.has_geetest_captcha_key ? t('keepUnchanged') : 'captcha_key'" /></label>
    <h3 class="reliability-section-title">{{ t('smtpSettings') }}</h3>
    <label>{{ t('smtpHost') }}<input v-model="siteSettingsForm.smtp_host" maxlength="200" placeholder="smtp.example.com" /><small>{{ t('smtpHint') }}</small></label>
    <label>{{ t('smtpPort') }}<input v-model="siteSettingsForm.smtp_port" maxlength="5" placeholder="465" /></label>
    <label>{{ t('smtpUsername') }}<input v-model="siteSettingsForm.smtp_username" maxlength="200" autocomplete="off" /></label>
    <label>{{ t('smtpPassword') }}<input v-model="siteSettingsForm.smtp_password" type="password" maxlength="200" autocomplete="new-password" :placeholder="siteSettingsForm.has_smtp_password ? t('keepUnchanged') : ''" /></label>
    <label>{{ t('smtpFrom') }}<input v-model="siteSettingsForm.smtp_from" type="email" maxlength="200" placeholder="noreply@example.com" /></label>
    <button class="button primary" :disabled="busy">{{ t('saveSettings') }}</button>
  </form>
</template>

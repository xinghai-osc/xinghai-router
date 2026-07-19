<script setup lang="ts">
import { useConsoleStore } from '~/composables/useConsoleStore'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Checkbox } from '@/components/ui/checkbox'
import { Card, CardContent } from '@/components/ui/card'

const store = useConsoleStore()
const { t, busy, siteSettingsForm, saveSiteSettings } = store
</script>

<template>
  <section class="flex flex-wrap items-center justify-between gap-4">
    <div>
      <h2 class="text-lg font-semibold">{{ t('siteSettings') }}</h2>
      <p class="text-sm text-muted-foreground">{{ t('siteSettingsDesc') }}</p>
    </div>
  </section>
  <Card class="mt-4">
    <CardContent>
      <form class="flex flex-col gap-6" @submit.prevent="saveSiteSettings">
        <div class="flex flex-col gap-2">
          <Label>{{ t('siteName') }}</Label>
          <Input v-model="siteSettingsForm.name" required maxlength="100" class="max-w-md" />
        </div>
        <div class="flex flex-col gap-2">
          <Label>{{ t('siteIconUrl') }}</Label>
          <Input v-model="siteSettingsForm.icon_url" type="url" placeholder="https://example.com/favicon.png" class="max-w-md" />
        </div>
        <div class="flex items-center gap-2">
          <Checkbox id="auto-disable" :model-value="siteSettingsForm.auto_disable_failed_channels" @update:model-value="v => siteSettingsForm.auto_disable_failed_channels = !!v" />
          <Label for="auto-disable">Auto-disable after 3 consecutive failures</Label>
        </div>

        <div class="flex flex-col gap-4">
          <h3 class="text-sm font-medium">{{ t('geetestSettings') }}</h3>
          <div class="flex flex-col gap-2">
            <Label>{{ t('geetestCaptchaId') }}</Label>
            <Input v-model="siteSettingsForm.geetest_captcha_id" maxlength="64" placeholder="captcha_id" class="max-w-md" />
            <p class="text-xs text-muted-foreground">{{ t('geetestHint') }}</p>
          </div>
          <div class="flex flex-col gap-2">
            <Label>{{ t('geetestCaptchaKey') }}</Label>
            <Input v-model="siteSettingsForm.geetest_captcha_key" type="password" maxlength="64" autocomplete="new-password" :placeholder="siteSettingsForm.has_geetest_captcha_key ? t('keepUnchanged') : 'captcha_key'" class="max-w-md" />
          </div>
        </div>

        <div class="flex flex-col gap-4">
          <h3 class="text-sm font-medium">{{ t('smtpSettings') }}</h3>
          <div class="grid gap-4 sm:grid-cols-2">
            <div class="flex flex-col gap-2">
              <Label>{{ t('smtpHost') }}</Label>
              <Input v-model="siteSettingsForm.smtp_host" maxlength="200" placeholder="smtp.example.com" />
              <p class="text-xs text-muted-foreground">{{ t('smtpHint') }}</p>
            </div>
            <div class="flex flex-col gap-2">
              <Label>{{ t('smtpPort') }}</Label>
              <Input v-model="siteSettingsForm.smtp_port" maxlength="5" placeholder="465" />
            </div>
            <div class="flex flex-col gap-2">
              <Label>{{ t('smtpUsername') }}</Label>
              <Input v-model="siteSettingsForm.smtp_username" maxlength="200" autocomplete="off" />
            </div>
            <div class="flex flex-col gap-2">
              <Label>{{ t('smtpPassword') }}</Label>
              <Input v-model="siteSettingsForm.smtp_password" type="password" maxlength="200" autocomplete="new-password" :placeholder="siteSettingsForm.has_smtp_password ? t('keepUnchanged') : ''" />
            </div>
            <div class="flex flex-col gap-2 sm:col-span-2">
              <Label>{{ t('smtpFrom') }}</Label>
              <Input v-model="siteSettingsForm.smtp_from" type="email" maxlength="200" placeholder="noreply@example.com" />
            </div>
          </div>
        </div>

        <Button type="submit" :disabled="busy" class="w-fit">{{ t('saveSettings') }}</Button>
      </form>
    </CardContent>
  </Card>
</template>

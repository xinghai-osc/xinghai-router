<script setup lang="ts">
import { endpoints, type AdminInvoiceSettings } from '~/src/api'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { settings: site } = useSiteSettings()
const { toast } = useToast()
const { busy, run } = useAction()

useHead({ title: () => `${t('system.invoiceSettingsTitle')} · ${site.value.name}` })

const EMPTY: AdminInvoiceSettings = {
  enabled: false,
  base_url: '',
  client_id: '',
  has_client_secret: false,
  need_pay_tax: false,
}

const { data, pending, error, refresh } = useResource(
  () => endpoints.getAdminInvoiceSettings(),
  { ...EMPTY },
)

const enabled = ref(false)
const baseUrl = ref('')
const clientId = ref('')
const needPayTax = ref(false)
/** Write-only: never seeded from the API, cleared again after every save. */
const clientSecret = ref('')

watch(data, (next) => {
  enabled.value = next.enabled
  baseUrl.value = next.base_url
  clientId.value = next.client_id
  needPayTax.value = next.need_pay_tax
  clientSecret.value = ''
}, { immediate: true })

async function saveSettings() {
  if (!baseUrl.value.trim()) {
    toast.error(t('system.invoiceSettingsBaseUrlRequired'))
    return
  }
  if (!clientId.value.trim()) {
    toast.error(t('system.invoiceSettingsClientIdRequired'))
    return
  }
  const ok = await run(() => endpoints.updateAdminInvoiceSettings({
    enabled: enabled.value,
    base_url: baseUrl.value.trim(),
    client_id: clientId.value.trim(),
    client_secret: clientSecret.value,
    need_pay_tax: needPayTax.value,
  }))
  if (!ok) {
    toast.error(t('common.actionFailed'))
    return
  }
  clientSecret.value = ''
  await refresh()
  toast.success(t('system.invoiceSettingsSaved'))
}
</script>

<template>
  <ConsoleSystemGate permission="system.manage">
    <div class="space-y-4">
      <div class="min-w-0 space-y-1">
        <h2 class="text-lg font-semibold text-ink">{{ t('system.invoiceSettingsTitle') }}</h2>
        <p class="text-[13px] text-muted">{{ t('system.invoiceSettingsLead') }}</p>
      </div>

      <UiAlert v-if="error" tone="danger" :title="t('common.loadFailed')">{{ error }}</UiAlert>

      <UiCard v-else-if="pending">
        <UiSkeleton :rows="5" class="h-10" />
      </UiCard>

      <UiCard v-else :title="t('system.invoiceSettingsTitle')">
        <form class="space-y-4" @submit.prevent="saveSettings">
          <div class="grid gap-4 sm:grid-cols-2">
            <UiField :label="t('system.invoiceSettingsEnabled')" :hint="t('system.invoiceSettingsEnabledHint')">
              <UiSwitch v-model="enabled" />
            </UiField>

            <UiField :label="t('system.invoiceSettingsNeedPayTax')" :hint="t('system.invoiceSettingsNeedPayTaxHint')">
              <UiSwitch v-model="needPayTax" />
            </UiField>
          </div>

          <UiField :label="t('system.invoiceSettingsBaseUrl')" :hint="t('system.invoiceSettingsBaseUrlHint')" for="invoice-base-url">
            <UiInput id="invoice-base-url" v-model="baseUrl" type="url" />
          </UiField>

          <div class="grid gap-4 sm:grid-cols-2">
            <UiField :label="t('system.invoiceSettingsClientId')" for="invoice-client-id">
              <UiInput id="invoice-client-id" v-model="clientId" mono />
            </UiField>

            <ConsoleSystemSecretField
              id="invoice-client-secret"
              v-model="clientSecret"
              :label="t('system.invoiceSettingsClientSecret')"
              :hint="t('system.invoiceSettingsClientSecretHint')"
              :configured="data.has_client_secret"
            />
          </div>

          <div class="flex justify-end">
            <UiButton type="submit" :loading="busy">{{ t('common.save') }}</UiButton>
          </div>
        </form>
      </UiCard>
    </div>
  </ConsoleSystemGate>
</template>
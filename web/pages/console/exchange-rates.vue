<script setup lang="ts">
import { ArrowRightLeft } from 'lucide-vue-next'
import { endpoints, type ExchangeRate, type ExchangeRateForm } from '~/src/api'
import { formatDateTime } from '~/src/format'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { can } = useAccount()
const { toast } = useToast()
const { busy, run } = useAction()

const allowed = computed(() => can('pricing.read'))
const canManage = computed(() => can('pricing.manage'))
const rates = useResource(() => endpoints.getAdminExchangeRates(), { data: [] as ExchangeRate[] })
const panelOpen = ref(false)
const editing = ref(false)
const formError = ref('')
const form = reactive({ currency: '', rate_to_base: '1', enabled: true })
const isBaseCurrency = computed(() => form.currency.trim().toUpperCase() === 'CNY')

function openCreate() {
  editing.value = false
  formError.value = ''
  form.currency = ''
  form.rate_to_base = '1'
  form.enabled = true
  panelOpen.value = true
}

function openEdit(rate: ExchangeRate) {
  editing.value = true
  formError.value = ''
  form.currency = rate.currency
  form.rate_to_base = String(rate.rate_to_base)
  form.enabled = rate.enabled
  panelOpen.value = true
}

async function save() {
  formError.value = ''
  const currency = form.currency.trim().toUpperCase()
  const rate = Number(form.rate_to_base)
  if (!/^[A-Z]{3,8}$/.test(currency)) {
    formError.value = t('admin.currencyInvalid')
    return
  }
  if (!Number.isFinite(rate) || rate <= 0 || rate > 1_000_000_000) {
    formError.value = t('admin.exchangeRateInvalid')
    return
  }
  const enabled = currency === 'CNY' ? true : form.enabled
  const payload: ExchangeRateForm = { currency, rate_to_base: currency === 'CNY' ? 1 : rate, enabled }
  const ok = await run(() => endpoints.saveExchangeRate(payload))
  if (!ok) {
    toast.error(t('common.actionFailed'))
    return
  }
  toast.success(t('admin.exchangeRateSaved'))
  panelOpen.value = false
  await rates.refresh()
}
</script>

<template>
  <ConsoleOpsDenied v-if="!allowed" />
  <div v-else class="space-y-4">
    <ConsoleOpsPageHeader :lead="t('admin.exchangeRatesLead')">
      <template #actions>
        <UiButton variant="secondary" size="sm" @click="rates.refresh()">{{ t('common.refresh') }}</UiButton>
        <UiButton v-if="canManage" size="sm" @click="openCreate">{{ t('admin.createExchangeRate') }}</UiButton>
      </template>
    </ConsoleOpsPageHeader>

    <UiAlert v-if="!canManage" tone="info">{{ t('admin.readOnlyNotice') }}</UiAlert>

    <ConsoleOpsListState
      :pending="rates.pending.value"
      :error="rates.error.value"
      :empty="!rates.data.value.data.length"
      :empty-icon="ArrowRightLeft"
      :empty-title="t('admin.exchangeRatesEmptyTitle')"
      :empty-description="t('admin.exchangeRatesEmptyBody')"
    >
      <UiTable>
        <thead>
          <tr>
            <th>{{ t('admin.currency') }}</th>
            <th class="num">{{ t('admin.rateToBase') }}</th>
            <th>{{ t('common.status') }}</th>
            <th>{{ t('common.updatedAt') }}</th>
            <th v-if="canManage">{{ t('common.actions') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="rate in rates.data.value.data" :key="rate.currency">
            <td class="font-mono font-medium text-ink">{{ rate.currency }}</td>
            <td class="num">{{ rate.rate_to_base }}</td>
            <td><UiBadge :tone="rate.enabled ? 'success' : 'neutral'" dot>{{ rate.enabled ? t('common.enabled') : t('common.disabled') }}</UiBadge></td>
            <td class="text-muted whitespace-nowrap">{{ formatDateTime(rate.updated_at) }}</td>
            <td v-if="canManage"><UiButton variant="ghost" size="sm" @click="openEdit(rate)">{{ t('common.edit') }}</UiButton></td>
          </tr>
        </tbody>
      </UiTable>
    </ConsoleOpsListState>

    <UiSlidePanel v-model:open="panelOpen" :title="t('admin.editExchangeRate')">
      <div class="space-y-4">
        <UiAlert v-if="formError" tone="danger">{{ formError }}</UiAlert>
        <UiField :label="t('admin.currency')" :hint="t('admin.currencyHint')" required>
          <UiInput v-model="form.currency" mono maxlength="8" :disabled="editing" />
        </UiField>
        <UiField :label="t('admin.rateToBase')" :hint="t('admin.rateToBaseHint')" required>
          <UiInput v-model="form.rate_to_base" type="number" min="0" step="any" mono :disabled="isBaseCurrency" />
        </UiField>
        <UiField :label="t('common.status')"><UiSwitch v-model="form.enabled" :disabled="isBaseCurrency" /></UiField>
      </div>
      <template #footer>
        <UiButton variant="secondary" @click="panelOpen = false">{{ t('common.cancel') }}</UiButton>
        <UiButton :loading="busy" @click="save">{{ t('common.save') }}</UiButton>
      </template>
    </UiSlidePanel>
  </div>
</template>

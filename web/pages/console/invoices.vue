<script setup lang="ts">
import { Download, FileText, RefreshCw, X } from 'lucide-vue-next'
import { endpoints, type InvoiceApplication, type InvoiceCheckout, type InvoiceEligibleOrder, type InvoiceValidation } from '~/src/api'
import { formatDateTime, formatMoney } from '~/src/format'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const MAX_ORDERS = 50

const { t } = useI18n()
const { toast } = useToast()
const { account } = useAccount()
const { settings: siteSettings } = useSiteSettings()

useHead({ title: () => `${t('nav.invoices')} · ${siteSettings.value.name}` })

const { data: settings, pending: settingsPending, error: settingsError } = useResource(
  () => endpoints.getInvoiceSettings(),
  { enabled: false, need_pay_tax: false },
)

const { data: orders, pending, error: ordersError, refresh: refreshOrders } = useResource(
  () => endpoints.getInvoiceEligibleOrders(),
  { data: [] as InvoiceEligibleOrder[] },
)

const { data: applications, pending: historyPending, error: historyError, refresh: refreshHistory } = useResource(
  () => endpoints.getInvoices(),
  { data: [] as InvoiceApplication[] },
)
const { busy, run } = useAction()

const selectedOrderNos = ref<string[]>([])
const buyerType = ref<'individual' | 'company'>('individual')
const buyerName = ref('')
const taxpayerId = ref('')
const buyerAddress = ref('')
const buyerPhone = ref('')
const buyerBank = ref('')
const buyerBankAccount = ref('')
const recipientEmail = ref(account.value?.email ?? '')
const needPayTax = ref(false)
watch(() => settings.value.need_pay_tax, value => { needPayTax.value = value }, { immediate: true })

const validation = ref<InvoiceValidation | null>(null)
const taxOrderNo = ref('')
const taxPaid = ref(false)
const confirmTarget = ref<InvoiceApplication | null>(null)
const pdfBusy = ref('')

const buyerTypeOptions = computed(() => [
  { value: 'individual', label: t('console.invoiceBuyerIndividual') },
  { value: 'company', label: t('console.invoiceBuyerCompany') },
])

const selectedCount = computed(() => selectedOrderNos.value.length)
const allEligible = computed(() => orders.value.map(order => order.order_no))
const canSubmit = computed(() => {
  if (!selectedOrderNos.value.length || !validation.value) return false
  if (needPayTax.value && !taxPaid.value) return false
  return true
})

const statusLabels: Record<string, string> = {
  'pending': t('console.invoiceStatusPending'),
  'approved': t('console.invoiceStatusApproved'),
  'rejected': t('console.invoiceStatusRejected'),
  'completed': t('console.invoiceStatusCompleted'),
  'canceled': t('console.invoiceStatusCanceled'),
}
const statusTones: Record<string, 'warn' | 'clay' | 'danger' | 'success' | 'neutral'> = {
  'pending': 'warn',
  'approved': 'clay',
  'rejected': 'danger',
  'completed': 'success',
  'canceled': 'neutral',
}

function formatAmount(value: string | number | null | undefined) {
  return formatMoney(Number(value || 0))
}

function isSelected(orderNo: string) {
  return selectedOrderNos.value.includes(orderNo)
}

function toggleAll(checked: boolean) {
  selectedOrderNos.value = checked ? allEligible.value.slice(0, MAX_ORDERS) : []
}

function toggleOrder(orderNo: string) {
  const index = selectedOrderNos.value.indexOf(orderNo)
  if (index >= 0) {
    selectedOrderNos.value.splice(index, 1)
    return
  }
  if (selectedOrderNos.value.length < MAX_ORDERS) {
    selectedOrderNos.value.push(orderNo)
  }
}

async function validateOrders() {
  if (!selectedOrderNos.value.length) {
    toast.error(t('console.invoiceSelectRequired'))
    return
  }
  validation.value = null
  taxPaid.value = false
  taxOrderNo.value = ''
  const taxNos = taxOrderNo.value ? [taxOrderNo.value] : []
  const result = await run(() => endpoints.validateInvoiceOrders(selectedOrderNos.value, needPayTax.value, taxNos))
  if (!result) {
    toast.error(t('console.invoiceValidationFailed'))
    return
  }
  validation.value = result
  if (result.taxDueAmount === '0.00') taxPaid.value = true
}

async function payTax(channel: InvoiceCheckout) {
  taxOrderNo.value = channel.taxOrderNo
  window.open(channel.payUrl, '_blank', 'noopener')
}

async function confirmTaxPaid() {
  if (!taxOrderNo.value) return
  const status = await run(() => endpoints.getInvoiceTaxStatus(taxOrderNo.value))
  if (!status) {
    toast.error(t('common.actionFailed'))
    return
  }
  const result = await run(() => endpoints.validateInvoiceOrders(selectedOrderNos.value, true, [taxOrderNo.value]))
  if (!result) {
    toast.error(t('console.invoiceValidationFailed'))
    return
  }
  validation.value = result
  taxPaid.value = result.taxDueAmount === '0.00'
  if (taxPaid.value) toast.success(t('console.invoiceTaxPaidOk'))
}

async function submit() {
  if (!buyerName.value.trim()) {
    toast.error(t('console.invoiceBuyerRequired'))
    return
  }
  if (buyerType.value === 'company' && !taxpayerId.value.trim()) {
    toast.error(t('console.invoiceTaxIdRequired'))
    return
  }
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(recipientEmail.value.trim())) {
    toast.error(t('console.invoiceEmailInvalid'))
    return
  }
  const taxNos = needPayTax.value && taxPaid.value ? [taxOrderNo.value] : []
  const ok = await run(() => endpoints.createInvoiceApplication({
    orderNos: selectedOrderNos.value,
    needPayTax: needPayTax.value,
    taxOrderNos: taxNos,
    buyerType: buyerType.value,
    title: buyerName.value.trim(),
    taxpayerId: taxpayerId.value.trim(),
    buyerAddress: buyerAddress.value.trim(),
    buyerPhone: buyerPhone.value.trim(),
    buyerBank: buyerBank.value.trim(),
    buyerBankAccount: buyerBankAccount.value.trim(),
    recipientEmail: recipientEmail.value.trim(),
  }))
  if (!ok) {
    toast.error(t('common.actionFailed'))
    return
  }
  toast.success(t('console.invoiceSubmitted'))
  validation.value = null
  taxPaid.value = false
  taxOrderNo.value = ''
  selectedOrderNos.value = []
  buyerName.value = ''
  taxpayerId.value = ''
  await Promise.all([refreshOrders(), refreshHistory()])
}

async function cancelApplication() {
  if (!confirmTarget.value) return
  const target = confirmTarget.value
  confirmTarget.value = null
  const ok = await run(() => endpoints.cancelInvoice(target.id))
  if (!ok) {
    toast.error(t('common.actionFailed'))
    return
  }
  toast.success(t('console.invoiceCancelled'))
  await Promise.all([refreshOrders(), refreshHistory()])
}

async function downloadPDF(app: InvoiceApplication) {
  pdfBusy.value = app.id
  try {
    const blob = await endpoints.downloadInvoicePDF(app.id)
    const url = URL.createObjectURL(blob)
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = `${app.application_id}.pdf`
    anchor.click()
    URL.revokeObjectURL(url)
  } catch {
    toast.error(t('console.invoiceNotFound'))
  } finally {
    pdfBusy.value = ''
  }
}

async function syncStatuses() {
  const ok = await run(() => endpoints.getInvoices(true))
  if (!ok) {
    toast.error(t('common.actionFailed'))
    return
  }
  toast.success(t('console.invoiceRefreshDone'))
  await refreshHistory()
}
</script>

<template>
  <div class="space-y-4">
    <UiCard v-if="settingsError" :title="t('nav.invoices')">
      <UiAlert tone="danger" :title="t('common.loadFailed')">{{ settingsError }}</UiAlert>
    </UiCard>

    <UiCard v-else-if="!settingsPending && !settings.enabled" :title="t('nav.invoices')">
      <UiEmptyState
        :icon="FileText"
        :title="t('console.invoiceDisabledTitle')"
        :description="t('console.invoiceDisabledBody')"
      />
    </UiCard>

    <template v-else>
      <UiCard :title="t('console.invoiceEligible')" :description="t('console.invoiceEligibleHint')" flush>
        <template #actions>
          <UiButton variant="secondary" size="sm" :disabled="busy" @click="refreshOrders">
            <RefreshCw class="size-4" />
            {{ t('common.refresh') }}
          </UiButton>
        </template>

        <div class="px-5 py-4">
          <ConsoleUserDataState
            :pending="pending"
            :error="ordersError"
            :empty="!orders.length"
            :rows="5"
            :empty-icon="FileText"
            :empty-title="t('console.invoiceNoOrders')"
            :empty-description="t('console.invoiceNoOrdersBody')"
          >
            <UiTable>
              <thead>
                <tr>
                  <th class="w-12">
                    <UiCheckbox
                      :model-value="selectedCount > 0"
                      :disabled="!allEligible.length"
                      @update:model-value="toggleAll($event ? true : false)"
                    />
                  </th>
                  <th>{{ t('console.invoiceOrderNo') }}</th>
                  <th>{{ t('console.invoiceKind') }}</th>
                  <th class="num">{{ t('console.invoiceAmount') }}</th>
                  <th>{{ t('console.invoicePaidAt') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="order in orders" :key="order.order_no">
                  <td>
                    <UiCheckbox
                      :model-value="isSelected(order.order_no)"
                      :disabled="!isSelected(order.order_no) && selectedCount >= MAX_ORDERS"
                      @update:model-value="toggleOrder(order.order_no)"
                    />
                  </td>
                  <td><code class="font-mono text-[13px] text-muted">{{ order.order_no }}</code></td>
                  <td>
                    <span class="text-muted">
                      {{ order.order_type === 'subscription' ? t('console.invoiceKindSubscription') : t('console.invoiceKindPayment') }}
                      <template v-if="order.plan_name"> · {{ order.plan_name }}</template>
                    </span>
                  </td>
                  <td class="num font-medium">{{ formatAmount(order.amount) }}</td>
                  <td class="text-muted">{{ formatDateTime(order.paid_at) }}</td>
                </tr>
              </tbody>
            </UiTable>
          </ConsoleUserDataState>
        </div>
      </UiCard>

      <UiCard :title="t('console.invoiceBuyerTitle')" :description="t('console.invoiceBuyerTitleHint')">
        <div class="grid gap-4 md:grid-cols-2">
          <UiField :label="t('console.invoiceBuyerType')" required>
            <UiSelect v-model="buyerType" :options="buyerTypeOptions" />
          </UiField>

          <UiField :label="t('console.invoiceBuyerName')" required>
            <UiInput v-model="buyerName" :placeholder="t('console.invoiceBuyerNamePlaceholder')" />
          </UiField>

          <UiField v-if="buyerType === 'company'" :label="t('console.invoiceTaxId')" :hint="t('console.invoiceTaxIdHint')" required>
            <UiInput v-model="taxpayerId" />
          </UiField>

          <UiField v-if="buyerType === 'company'" :label="t('console.invoiceBuyerBank')">
            <UiInput v-model="buyerBank" />
          </UiField>

          <UiField v-if="buyerType === 'company'" :label="t('console.invoiceBuyerAccount')">
            <UiInput v-model="buyerBankAccount" />
          </UiField>

          <UiField :label="t('console.invoiceBuyerAddress')">
            <UiInput v-model="buyerAddress" />
          </UiField>

          <UiField :label="t('console.invoiceBuyerPhone')">
            <UiInput v-model="buyerPhone" />
          </UiField>

          <UiField :label="t('console.invoiceRecipientEmail')" :hint="t('console.invoiceRecipientEmailHint')" required>
            <UiInput v-model="recipientEmail" type="email" />
          </UiField>

          <UiField :label="t('console.invoiceNeedPayTax')" :hint="t('console.invoiceNeedPayTaxHint', { pct: '1%' })">
            <UiSwitch v-model="needPayTax" />
          </UiField>
        </div>

        <template #footer>
          <UiButton :loading="busy" @click="validateOrders">
            {{ t('console.invoiceValidate') }} · {{ selectedCount }}
          </UiButton>
        </template>
      </UiCard>

      <UiCard v-if="validation" :title="t('console.invoiceValidationResult')">
        <dl class="grid gap-4 md:grid-cols-4">
          <div>
            <dt class="text-2xs tracking-wide text-faint uppercase">{{ t('console.invoiceTotalAmount') }}</dt>
            <dd class="numeric mt-1 text-lg font-semibold text-ink">{{ formatAmount(validation.totalAmount) }}</dd>
          </div>
          <div>
            <dt class="text-2xs tracking-wide text-faint uppercase">{{ t('console.invoiceTaxAmount') }}</dt>
            <dd class="numeric mt-1 text-lg font-semibold text-ink">{{ formatAmount(validation.taxAmount) }}</dd>
          </div>
          <div>
            <dt class="text-2xs tracking-wide text-faint uppercase">{{ t('console.invoiceTaxPaidAmount') }}</dt>
            <dd class="numeric mt-1 text-lg font-semibold text-ink">{{ formatAmount(validation.taxPaidAmount) }}</dd>
          </div>
          <div>
            <dt class="text-2xs tracking-wide text-faint uppercase">{{ t('console.invoiceTaxDueAmount') }}</dt>
            <dd class="numeric mt-1 text-lg font-semibold text-ink">{{ formatAmount(validation.taxDueAmount) }}</dd>
          </div>
        </dl>

        <div class="mt-4 flex flex-wrap items-center gap-2">
          <template v-if="needPayTax && validation.taxDueAmount !== '0.00' && !taxPaid">
            <span class="text-sm text-muted">
              {{ t('console.invoiceTaxCheckout', { amount: formatAmount(validation.taxDueAmount) }) }}
            </span>
            <UiButton
              v-for="(channel, key) in validation.taxPayments"
              :key="key"
              variant="secondary"
              @click="payTax(channel)"
            >
              {{ key === 'alipay' ? t('console.invoicePayTaxAlipay') : t('console.invoicePayTaxWxpay') }}
            </UiButton>
            <UiButton :disabled="!taxOrderNo" :loading="busy" @click="confirmTaxPaid">
              {{ t('console.invoiceConfirmPaid') }}
            </UiButton>
            <span class="text-[13px] text-faint">{{ t('console.invoiceConfirmPaidHint') }}</span>
          </template>

          <UiBadge v-if="taxPaid" tone="success" dot>{{ t('console.invoiceTaxPaidOk') }}</UiBadge>
        </div>

        <template #footer>
          <UiButton :disabled="!canSubmit" :loading="busy" @click="submit">
            {{ t('console.invoiceSubmit') }} · {{ validation.totalAmount ? formatAmount(validation.totalAmount) : formatAmount(0) }}
          </UiButton>
        </template>
      </UiCard>

      <UiCard :title="t('console.invoiceHistory')" :description="t('console.invoiceHistoryHint')" flush>
        <template #actions>
          <UiButton variant="secondary" size="sm" :disabled="busy" @click="syncStatuses">
            <RefreshCw class="size-4" />
            {{ t('common.refresh') }}
          </UiButton>
        </template>

        <div class="px-5 py-4">
          <ConsoleUserDataState
            :pending="historyPending"
            :error="historyError"
            :empty="!applications.length"
            :rows="4"
            :empty-icon="FileText"
            :empty-title="t('console.invoiceHistoryEmptyTitle')"
            :empty-description="t('console.invoiceHistoryEmptyBody')"
          >
            <UiTable>
              <thead>
                <tr>
                  <th>{{ t('console.invoiceId') }}</th>
                  <th>{{ t('console.invoiceBuyerName') }}</th>
                  <th class="num">{{ t('console.invoiceAmount') }}</th>
                  <th>{{ t('common.status') }}</th>
                  <th>{{ t('common.createdAt') }}</th>
                  <th class="text-right">{{ t('common.actions') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="app in applications" :key="app.id">
                  <td><code class="font-mono text-[13px] text-muted">{{ app.application_id }}</code></td>
                  <td>{{ app.title }}</td>
                  <td class="num font-medium">{{ formatAmount(app.total_amount) }}</td>
                  <td><UiBadge :tone="statusTones[app.status] ?? 'neutral'">{{ statusLabels[app.status] ?? app.status }}</UiBadge></td>
                  <td class="text-muted">{{ formatDateTime(app.created_at) }}</td>
                  <td class="text-right whitespace-nowrap">
                    <UiButton
                      v-if="['approved', 'completed'].includes(app.status)"
                      size="sm"
                      variant="secondary"
                      :loading="pdfBusy === app.id"
                      @click="downloadPDF(app)"
                    >
                      <Download class="size-4" />
                      {{ t('console.invoiceDownload') }}
                    </UiButton>
                    <UiButton v-if="app.status === 'pending'" size="sm" variant="ghost" @click="confirmTarget = app">
                      <X class="size-4" />
                      {{ t('console.invoiceCancelApplication') }}
                    </UiButton>
                  </td>
                </tr>
              </tbody>
            </UiTable>
          </ConsoleUserDataState>
        </div>
      </UiCard>
    </template>

    <UiDialog
      :open="!!confirmTarget"
      @update:open="(v: boolean) => { if (!v) confirmTarget = null }"
      :title="t('console.invoiceCancelTitle')"
      :description="t('console.invoiceCancelBody')"
    >
      <template #footer>
        <UiButton variant="secondary" @click="confirmTarget = null">{{ t('common.cancel') }}</UiButton>
        <UiButton variant="danger" :loading="busy" @click="cancelApplication">{{ t('console.invoiceCancelApplication') }}</UiButton>
      </template>
    </UiDialog>
  </div>
</template>
<script setup lang="ts">
import { CreditCard, Wallet } from 'lucide-vue-next'
import { endpoints, type PaymentMethod, type PaymentOrder } from '~/src/api'
import { formatDateTime, formatMoney } from '~/src/format'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const QUICK_AMOUNTS = [10, 50, 100, 500]
const MIN_AMOUNT = 1
const MAX_AMOUNT = 100000

const { t } = useI18n()
const { toast } = useToast()
const { account, loadAccount } = useAccount()
const { settings } = useSiteSettings()

useHead({ title: () => `${t('nav.wallet')} · ${settings.value.name}` })

const { data: payments, pending, error, refresh } = useResource(
  () => endpoints.getAccountPayments(),
  { enabled: false, payment_methods: [] as PaymentMethod[], data: [] as PaymentOrder[] },
)
const { busy, run } = useAction()

const amount = ref('')
const method = ref('')
const formError = ref('')

const methodOptions = computed(() =>
  payments.value.payment_methods.map(entry => ({ value: entry.code, label: entry.name })))

watch(methodOptions, (options) => {
  if (!method.value && options.length) method.value = options[0].value
})

function pick(value: number) {
  amount.value = String(value)
  formError.value = ''
}

async function submit() {
  const value = Number(amount.value)
  if (!Number.isFinite(value) || value < MIN_AMOUNT || value > MAX_AMOUNT) {
    formError.value = t('console.amountInvalid')
    return
  }
  if (!method.value) {
    formError.value = t('console.methodRequired')
    return
  }
  formError.value = ''

  let payUrl = ''
  const ok = await run(async () => {
    payUrl = (await endpoints.createAccountPayment(value.toFixed(2), method.value)).pay_url
  })
  if (!ok) { toast.error(t('common.actionFailed')); return }

  const opened = window.open(payUrl, '_blank', 'noopener')
  if (opened) toast.success(t('console.topUpRedirect'))
  else toast.warn(t('console.popupBlocked'))

  amount.value = ''
  await Promise.all([refresh(), loadAccount(true)])
}
</script>

<template>
  <div class="space-y-4">
    <div class="grid gap-4 lg:grid-cols-[20rem_1fr]">
      <ConsoleUserStatCard
        :label="t('console.currentBalance')"
        :value="formatMoney(account?.balance ?? 0)"
        :hint="t('console.balanceHint')"
        :icon="Wallet"
        :loading="!account"
      />

      <UiCard :title="t('nav.wallet')" :description="t('console.walletDescription')">
        <UiSkeleton v-if="pending" :rows="4" />

        <UiAlert v-else-if="error" tone="danger" :title="t('common.loadFailed')">{{ error }}</UiAlert>

        <UiEmptyState
          v-else-if="!payments.enabled"
          :icon="CreditCard"
          :title="t('console.topUpDisabledTitle')"
          :description="t('console.topUpDisabledBody')"
        />

        <form v-else class="space-y-4" @submit.prevent="submit">
          <UiField
            :label="t('console.topUpAmount')"
            :hint="t('console.topUpAmountHint')"
            :error="formError"
            required
          >
            <UiInput
              v-model="amount"
              type="number"
              :placeholder="t('console.topUpAmountPlaceholder')"
            >
              <template #leading>
                <span class="text-[13px]">¥</span>
              </template>
            </UiInput>
          </UiField>

          <div class="space-y-1.5">
            <p class="text-[13px] font-medium text-ink">{{ t('console.quickPick') }}</p>
            <div class="flex flex-wrap gap-2">
              <UiButton
                v-for="value in QUICK_AMOUNTS"
                :key="value"
                variant="secondary"
                size="sm"
                @click="pick(value)"
              >{{ formatMoney(value, 0) }}</UiButton>
            </div>
          </div>

          <UiField :label="t('console.paymentMethod')" required>
            <UiSelect
              v-model="method"
              :options="methodOptions"
              :placeholder="t('common.selectPlaceholder')"
            />
          </UiField>

          <UiButton type="submit" :loading="busy">{{ t('console.topUpSubmit') }}</UiButton>
        </form>
      </UiCard>
    </div>

    <UiCard :title="t('console.orderHistory')" flush>
      <template #actions>
        <UiButton variant="secondary" size="sm" @click="refresh">{{ t('common.refresh') }}</UiButton>
      </template>

      <div class="px-5 py-4">
        <ConsoleUserDataState
          :pending="pending"
          :error="error"
          :empty="!payments.data.length"
          :rows="5"
          :empty-icon="CreditCard"
          :empty-title="t('console.ordersEmptyTitle')"
          :empty-description="t('console.ordersEmptyBody')"
        >
          <UiTable>
            <thead>
              <tr>
                <th>{{ t('console.orderNo') }}</th>
                <th class="num">{{ t('console.amount') }}</th>
                <th>{{ t('console.orderMethod') }}</th>
                <th>{{ t('common.status') }}</th>
                <th>{{ t('console.paidAt') }}</th>
                <th>{{ t('common.createdAt') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="order in payments.data" :key="order.order_no">
                <td><code class="font-mono text-[13px] text-muted">{{ order.order_no }}</code></td>
                <td class="num font-medium">{{ formatMoney(order.amount) }}</td>
                <td class="text-muted">{{ order.payment_type }}</td>
                <td><ConsoleUserStatusBadge :status="order.status" /></td>
                <td class="text-muted">{{ formatDateTime(order.paid_at) }}</td>
                <td class="text-muted">{{ formatDateTime(order.created_at) }}</td>
              </tr>
            </tbody>
          </UiTable>
        </ConsoleUserDataState>
      </div>
    </UiCard>
  </div>
</template>

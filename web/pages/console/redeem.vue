<script setup lang="ts">
import { Ticket } from 'lucide-vue-next'
import { endpoints, type RedemptionCodeRedemption, type RedemptionResult } from '~/src/api'
import { formatDateTime, formatMoney } from '~/src/format'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { settings } = useSiteSettings()
const { toast } = useToast()
const { account, loadAccount } = useAccount()
const { busy, run } = useAction()

useHead({ title: () => `${t('nav.redeem')} · ${settings.value.name}` })

const { data: redemptions, pending, error, refresh } = useResource(
  () => endpoints.getAccountRedemptions(),
  { data: [] as RedemptionCodeRedemption[] },
)

const code = ref('')
const formError = ref('')

async function submit() {
  const value = code.value.trim()
  if (!value) {
    formError.value = t('console.redeemCodeRequired')
    return
  }
  formError.value = ''

  let result: RedemptionResult | null = null
  const ok = await run(async () => {
    result = await endpoints.redeemCode(value)
  })
  if (!ok) {
    toast.error(t('common.actionFailed'))
    return
  }

  if (result?.reward_type === 'balance' && result.amount) {
    toast.success(t('console.redeemSuccessBalance', { amount: formatMoney(result.amount) }))
  } else {
    toast.success(t('console.redeemSuccessSubscription'))
  }

  code.value = ''
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
        :icon="Ticket"
        :loading="!account"
      />

      <UiCard :title="t('console.redeemTitle')" :description="t('console.redeemDescription')">
        <form class="space-y-4" @submit.prevent="submit">
          <UiField
            :label="t('console.redeemTitle')"
            :error="formError"
            required
          >
            <UiInput
              v-model="code"
              :placeholder="t('console.redeemCodePlaceholder')"
            >
              <template #leading>
                <Ticket class="size-4 text-muted" />
              </template>
            </UiInput>
          </UiField>

          <UiButton type="submit" :loading="busy">{{ t('console.redeemSubmit') }}</UiButton>
        </form>
      </UiCard>
    </div>

    <UiCard :title="t('console.redeemHistoryTitle')" flush>
      <template #actions>
        <UiButton variant="secondary" size="sm" @click="refresh">{{ t('common.refresh') }}</UiButton>
      </template>

      <div class="px-5 py-4">
        <ConsoleUserDataState
          :pending="pending"
          :error="error"
          :empty="!redemptions.data.length"
          :rows="4"
          :empty-icon="Ticket"
          :empty-title="t('console.redeemHistoryEmptyTitle')"
          :empty-description="t('console.redeemHistoryEmptyBody')"
        >
          <UiTable>
            <thead>
              <tr>
                <th>{{ t('console.redeemCode') }}</th>
                <th>{{ t('console.redeemRewardType') }}</th>
                <th class="num">{{ t('console.redeemAmount') }}</th>
                <th>{{ t('console.redeemPlan') }}</th>
                <th>{{ t('console.redeemRedeemedAt') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="item in redemptions.data" :key="item.id">
                <td><code class="font-mono text-[13px] text-muted">{{ item.code }}</code></td>
                <td>
                  <UiBadge :tone="item.plan_id ? 'clay' : 'neutral'">
                    {{ item.plan_id ? t('console.redeemRewardSubscription') : t('console.redeemRewardBalance') }}
                  </UiBadge>
                </td>
                <td class="num">{{ item.amount ? formatMoney(item.amount) : '—' }}</td>
                <td class="text-muted">{{ item.plan_name || '—' }}</td>
                <td class="text-muted">{{ formatDateTime(item.created_at) }}</td>
              </tr>
            </tbody>
          </UiTable>
        </ConsoleUserDataState>
      </div>
    </UiCard>
  </div>
</template>

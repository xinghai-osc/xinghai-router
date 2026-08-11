<script setup lang="ts">
import { RefreshCw, ShoppingCart } from 'lucide-vue-next'
import { endpoints, type OrderRecord } from '~/src/api'
import { formatDateTime, formatMoney } from '~/src/format'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { settings } = useSiteSettings()

useHead({ title: () => `${t('nav.orders')} · ${settings.value.name}` })

const { data: orders, pending, error, refresh } = useResource(
  () => endpoints.getAccountOrders(),
  { data: [] as OrderRecord[] },
)

function typeLabel(order: OrderRecord): string {
  return order.order_type === 'subscription'
    ? t('console.orderTypeSubscription')
    : t('console.orderTypePayment')
}
</script>

<template>
  <div class="space-y-4">
    <UiCard :title="t('nav.orders')" :description="t('console.ordersDescription')" flush>
      <template #actions>
        <UiButton variant="secondary" size="sm" @click="refresh">
          <RefreshCw class="size-4" />
          {{ t('common.refresh') }}
        </UiButton>
      </template>

      <div class="px-5 py-4">
        <ConsoleUserDataState
          :pending="pending"
          :error="error"
          :empty="!orders.data.length"
          :rows="6"
          :empty-icon="ShoppingCart"
          :empty-title="t('console.allOrdersEmptyTitle')"
          :empty-description="t('console.allOrdersEmptyBody')"
        >
          <UiTable>
            <thead>
              <tr>
                <th>{{ t('console.orderNo') }}</th>
                <th>{{ t('console.orderKind') }}</th>
                <th>{{ t('console.plan') }}</th>
                <th class="num">{{ t('console.amount') }}</th>
                <th>{{ t('console.orderMethod') }}</th>
                <th>{{ t('common.status') }}</th>
                <th>{{ t('console.paidAt') }}</th>
                <th>{{ t('common.createdAt') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="order in orders.data" :key="order.order_no">
                <td><code class="font-mono text-[13px] text-muted">{{ order.order_no }}</code></td>
                <td>
                  <UiBadge :tone="order.order_type === 'subscription' ? 'clay' : 'neutral'">
                    {{ typeLabel(order) }}
                  </UiBadge>
                </td>
                <td class="text-muted">{{ order.plan_name || '—' }}</td>
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

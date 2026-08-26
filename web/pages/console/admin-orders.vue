<script setup lang="ts">
import { RefreshCw, ShoppingCart } from 'lucide-vue-next'
import { endpoints, type AdminOrder } from '~/src/api'
import { formatDateTime, formatMoney } from '~/src/format'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { settings } = useSiteSettings()
const { can } = useAccount()
const allowed = computed(() => can('users.read'))
const page = ref(1)
const pageSize = ref('50')
const search = ref('')

useHead({ title: () => `${t('nav.adminOrders')} · ${settings.value.name}` })

function query() {
  const params = new URLSearchParams({ page: String(page.value), page_size: pageSize.value })
  if (search.value.trim()) params.set('q', search.value.trim())
  return `?${params.toString()}`
}

const orders = useResource(() => endpoints.getAdminOrders(query()), { data: [] as AdminOrder[], total: 0, page: 1, page_size: 50 })
watch(page, () => { void orders.refresh() })
watch(search, () => { page.value = 1; void orders.refresh() })
watch(pageSize, () => { page.value = 1; void orders.refresh() })

function typeLabel(order: AdminOrder) {
  return order.order_type === 'subscription' ? t('console.orderTypeSubscription') : t('console.orderTypePayment')
}
</script>

<template>
  <div v-if="!allowed">
    <UiEmptyState :icon="ShoppingCart" :title="t('admin.noAccessTitle')" :description="t('admin.noAccessBody')" />
  </div>
  <div v-else class="space-y-4">
    <UiCard :title="t('nav.adminOrders')" :description="t('admin.adminOrdersLead')" flush>
      <template #actions>
        <UiButton variant="secondary" size="sm" @click="orders.refresh"><RefreshCw class="size-4" />{{ t('common.refresh') }}</UiButton>
      </template>
      <div class="space-y-4 px-5 py-4">
        <UiInput v-model="search" :placeholder="t('admin.adminOrdersSearchPlaceholder')" />
        <ConsoleOpsListState :pending="orders.pending.value" :error="orders.error.value" :empty="!orders.data.value.data.length" :rows="6" :empty-icon="ShoppingCart" :empty-title="t('admin.adminOrdersEmptyTitle')" :empty-description="t('admin.adminOrdersEmptyBody')">
          <UiTable>
            <thead><tr><th>{{ t('console.orderNo') }}</th><th>{{ t('admin.adminOrdersUser') }}</th><th>{{ t('admin.adminOrdersType') }}</th><th>{{ t('console.plan') }}</th><th class="num">{{ t('console.amount') }}</th><th>{{ t('common.status') }}</th><th>{{ t('common.createdAt') }}</th></tr></thead>
            <tbody>
              <tr v-for="order in orders.data.value.data" :key="`${order.order_type}-${order.order_no}`">
                <td><code class="font-mono text-[13px] text-muted">{{ order.order_no }}</code></td>
                <td><div class="font-medium text-ink">{{ order.user_name }}</div><div class="text-[12px] text-muted">{{ order.user_email }}</div></td>
                <td><UiBadge :tone="order.order_type === 'subscription' ? 'clay' : 'neutral'">{{ typeLabel(order) }}</UiBadge></td>
                <td class="text-muted">{{ order.plan_name || t('common.none') }}</td>
                <td class="num font-medium">{{ formatMoney(order.amount) }}</td>
                <td><ConsoleUserStatusBadge :status="order.status" /></td>
                <td class="text-muted whitespace-nowrap">{{ formatDateTime(order.created_at) }}</td>
              </tr>
            </tbody>
          </UiTable>
          <ConsoleOpsPagination v-model:page="page" v-model:pageSize="pageSize" :total="orders.data.value.total" :page-size-options="['20', '50', '100']" />
        </ConsoleOpsListState>
      </div>
    </UiCard>
  </div>
</template>

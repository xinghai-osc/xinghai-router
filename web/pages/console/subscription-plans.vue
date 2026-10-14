<script setup lang="ts">
import { Plus, Sparkles } from 'lucide-vue-next'
import {
  ApiError, clearPublicCache, endpoints,
  type Group, type SubscriptionProduct, type SubscriptionProductForm, type SubscriptionTier,
} from '~/src/api'
import { formatMoney, formatNumber } from '~/src/format'
import { periodLabel, sortProducts, sortTiers, tierPrices } from '~/src/subscriptions'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { settings } = useSiteSettings()
const { toast } = useToast()
const { busy, run } = useAction()

useHead({ title: () => `${t('system.productsTitle')} · ${settings.value.name}` })

const { data: productsData, pending, error, refresh } = useResource(
  () => endpoints.getAdminSubscriptionProducts(),
  { data: [] as SubscriptionProduct[] },
)
const { data: groupsData } = useResource(
  () => endpoints.getAdminGroups('?page_size=100'),
  { data: [] as Group[], total: 0, page: 1, page_size: 100 },
)

const products = computed(() => sortProducts(productsData.value.data))
const groups = computed(() => groupsData.value.data)

const dialogOpen = ref(false)
const editing = ref<SubscriptionProduct | null>(null)
const removing = ref<SubscriptionProduct | null>(null)
const forcing = ref<SubscriptionProduct | null>(null)

function openCreate() {
  editing.value = null
  dialogOpen.value = true
}

function openEdit(product: SubscriptionProduct) {
  editing.value = product
  dialogOpen.value = true
}

function priceSummary(tier: SubscriptionTier): string {
  const prices = tierPrices(tier)
  if (!prices.length) return '—'
  return prices
    .map(price => `${periodLabel(t, price.billing_period, 'system')} ${formatMoney(price.price)}`)
    .join(' · ')
}

async function save(form: SubscriptionProductForm) {
  const target = editing.value
  const ok = await run(() => (target
    ? endpoints.updateSubscriptionProduct(target.id, form)
    : endpoints.createSubscriptionProduct(form)))
  if (!ok) {
    toast.error(t('common.actionFailed'))
    return
  }
  clearPublicCache('subscription-plans')
  toast.success(target ? t('system.productUpdated') : t('system.productCreated'))
  dialogOpen.value = false
  await refresh()
}

async function remove(force: boolean) {
  const target = force ? forcing.value : removing.value
  if (!target) return
  let code = ''
  const ok = await run(async () => {
    try {
      await endpoints.deleteSubscriptionProduct(target.id, force)
    } catch (cause) {
      if (cause instanceof ApiError) code = cause.code
      throw cause
    }
  })
  if (!ok) {
    if (code === 'subscription_in_use') {
      removing.value = null
      await nextTick()
      forcing.value = target
      return
    }
    toast.error(t('common.actionFailed'))
    return
  }
  clearPublicCache('subscription-plans')
  toast.success(t('system.productDeleted'))
  removing.value = null
  forcing.value = null
  await refresh()
}
</script>

<template>
  <ConsoleSystemGate permission="system.manage">
    <div class="space-y-4">
      <div class="flex flex-wrap items-end justify-between gap-3">
        <div class="min-w-0 space-y-1">
          <h2 class="text-lg font-semibold text-ink">{{ t('system.productsTitle') }}</h2>
          <p class="text-[13px] text-muted">{{ t('system.productsLead') }}</p>
        </div>
        <UiButton size="sm" @click="openCreate">
          <Plus class="size-4" />
          {{ t('system.newProduct') }}
        </UiButton>
      </div>

      <ConsoleSystemDataState
        :pending="pending"
        :error="error"
        :empty="products.length === 0"
        :empty-icon="Sparkles"
        :empty-title="t('system.productsEmptyTitle')"
        :empty-description="t('system.productsEmptyBody')"
      >
        <UiTable>
          <thead>
            <tr>
              <th>{{ t('common.name') }}</th>
              <th>{{ t('system.tiersTitle') }}</th>
              <th class="num">{{ t('system.productSortOrder') }}</th>
              <th>{{ t('common.status') }}</th>
              <th>{{ t('common.actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="product in products" :key="product.id">
              <td>
                <p class="font-medium text-ink">{{ product.name }}</p>
                <p v-if="product.description" class="mt-0.5 text-[13px] text-muted">{{ product.description }}</p>
              </td>
              <td>
                <p class="text-[13px] text-muted">
                  {{ t('system.tierCount', { count: formatNumber(product.tiers.length) }) }}
                </p>
                <div v-if="product.tiers.length" class="mt-1 space-y-0.5">
                  <p
                    v-for="tier in sortTiers(product.tiers)"
                    :key="tier.id"
                    class="flex flex-wrap items-baseline gap-x-2 text-[13px]"
                  >
                    <span class="numeric text-faint">{{ tier.level }}</span>
                    <span class="font-medium text-ink">{{ tier.name || '—' }}</span>
                    <span class="text-muted">{{ priceSummary(tier) }}</span>
                    <span v-if="!tier.enabled" class="text-faint">{{ t('common.disabled') }}</span>
                  </p>
                </div>
              </td>
              <td class="num">{{ product.sort_order }}</td>
              <td>
                <UiBadge :tone="product.enabled ? 'success' : 'neutral'" dot>
                  {{ product.enabled ? t('common.enabled') : t('common.disabled') }}
                </UiBadge>
              </td>
              <td>
                <div class="flex items-center gap-1">
                  <UiButton variant="ghost" size="sm" @click="openEdit(product)">{{ t('common.edit') }}</UiButton>
                  <UiButton variant="ghost" size="sm" @click="removing = product">{{ t('common.delete') }}</UiButton>
                </div>
              </td>
            </tr>
          </tbody>
        </UiTable>
      </ConsoleSystemDataState>
    </div>

    <ConsoleSystemSubscriptionProductDialog
      v-model:open="dialogOpen"
      :product="editing"
      :groups="groups"
      :busy="busy"
      @submit="save"
    />

    <ConsoleSystemConfirmDialog
      :open="removing !== null"
      :body="t('system.deleteProductBody')"
      :busy="busy"
      @update:open="value => { if (!value) removing = null }"
      @confirm="remove(false)"
    />

    <ConsoleSystemConfirmDialog
      :open="forcing !== null"
      :title="t('system.productInUse')"
      :body="t('system.deleteProductBody')"
      :confirm-label="t('system.forceDeleteProduct')"
      :busy="busy"
      @update:open="value => { if (!value) forcing = null }"
      @confirm="remove(true)"
    />
  </ConsoleSystemGate>
</template>

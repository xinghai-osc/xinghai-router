<script setup lang="ts">
import { CreditCard, Plus } from 'lucide-vue-next'
import { endpoints, type PaymentMethod, type PaymentMethodForm, type PaymentSettings } from '~/src/api'
import { formatDateTime } from '~/src/format'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { settings: site } = useSiteSettings()
const { toast } = useToast()
const { busy, run } = useAction()

useHead({ title: () => `${t('system.paymentTitle')} · ${site.value.name}` })

const EMPTY: PaymentSettings = {
  enabled: false,
  base_url: '',
  merchant_id: '',
  has_merchant_key: false,
  public_base_url: '',
  methods: [],
}

const { data, pending, error, refresh } = useResource(
  () => endpoints.getAdminPaymentSettings(),
  { ...EMPTY },
)

const enabled = ref(false)
const baseUrl = ref('')
const merchantId = ref('')
const publicBaseUrl = ref('')
/** Write-only: never seeded from the API, cleared again after every save. */
const merchantKey = ref('')

watch(data, (next) => {
  enabled.value = next.enabled
  baseUrl.value = next.base_url
  merchantId.value = next.merchant_id
  publicBaseUrl.value = next.public_base_url
  merchantKey.value = ''
}, { immediate: true })

const methods = computed(() => data.value.methods ?? [])

async function saveSettings() {
  const ok = await run(() => endpoints.updateAdminPaymentSettings({
    enabled: enabled.value,
    base_url: baseUrl.value.trim(),
    merchant_id: merchantId.value.trim(),
    merchant_key: merchantKey.value,
    public_base_url: publicBaseUrl.value.trim(),
  }))
  if (!ok) {
    toast.error(t('common.actionFailed'))
    return
  }
  merchantKey.value = ''
  toast.success(t('system.paymentSaved'))
  await refresh()
}

const methodDialogOpen = ref(false)
const editingMethod = ref<PaymentMethod | null>(null)
const removingMethod = ref<PaymentMethod | null>(null)

function openCreateMethod() {
  editingMethod.value = null
  methodDialogOpen.value = true
}

function openEditMethod(method: PaymentMethod) {
  editingMethod.value = method
  methodDialogOpen.value = true
}

async function submitMethod(form: PaymentMethodForm) {
  const target = editingMethod.value
  const ok = await run(() => (target
    ? endpoints.updatePaymentMethod(target.id, form)
    : endpoints.createPaymentMethod(form)))
  if (!ok) {
    toast.error(t('common.actionFailed'))
    return
  }
  toast.success(target ? t('system.methodUpdated') : t('system.methodCreated'))
  methodDialogOpen.value = false
  await refresh()
}

async function confirmDeleteMethod() {
  const target = removingMethod.value
  if (!target) return
  const ok = await run(() => endpoints.deletePaymentMethod(target.id))
  if (!ok) {
    toast.error(t('common.actionFailed'))
    return
  }
  toast.success(t('system.methodDeleted'))
  removingMethod.value = null
  await refresh()
}
</script>

<template>
  <ConsoleSystemGate permission="system.manage">
    <div class="space-y-4">
      <div class="min-w-0 space-y-1">
        <h2 class="text-lg font-semibold text-ink">{{ t('system.paymentTitle') }}</h2>
        <p class="text-[13px] text-muted">{{ t('system.paymentLead') }}</p>
      </div>

      <UiAlert v-if="error" tone="danger" :title="t('common.loadFailed')">{{ error }}</UiAlert>

      <UiCard v-else-if="pending">
        <UiSkeleton :rows="5" class="h-10" />
      </UiCard>

      <UiCard v-else :title="t('system.payments')">
        <form class="space-y-4" @submit.prevent="saveSettings">
          <UiField :label="t('system.paymentEnabled')" :hint="t('system.paymentEnabledHint')">
            <UiSwitch v-model="enabled" :label="t('system.paymentEnabled')" />
          </UiField>

          <div class="grid gap-4 sm:grid-cols-2">
            <UiField :label="t('system.paymentBaseUrl')" :hint="t('system.paymentBaseUrlHint')" for="pay-base-url">
              <UiInput id="pay-base-url" v-model="baseUrl" type="url" />
            </UiField>

            <UiField :label="t('system.publicBaseUrl')" :hint="t('system.publicBaseUrlHint')" for="pay-public-url">
              <UiInput id="pay-public-url" v-model="publicBaseUrl" type="url" />
            </UiField>
          </div>

          <div class="grid gap-4 sm:grid-cols-2">
            <UiField :label="t('system.merchantId')" :hint="t('system.merchantIdHint')" for="pay-merchant-id">
              <UiInput id="pay-merchant-id" v-model="merchantId" mono />
            </UiField>

            <ConsoleSystemSecretField
              id="pay-merchant-key"
              v-model="merchantKey"
              :label="t('system.merchantKey')"
              :configured="data.has_merchant_key"
            />
          </div>

          <div class="flex justify-end">
            <UiButton type="submit" :loading="busy">{{ t('common.save') }}</UiButton>
          </div>
        </form>
      </UiCard>

      <div class="flex flex-wrap items-end justify-between gap-3 pt-2">
        <div class="min-w-0 space-y-1">
          <h3 class="text-[15px] font-semibold text-ink">{{ t('system.paymentMethods') }}</h3>
          <p class="text-[13px] text-muted">{{ t('system.paymentMethodsLead') }}</p>
        </div>
        <UiButton size="sm" variant="secondary" @click="openCreateMethod">
          <Plus class="size-4" />
          {{ t('system.newMethod') }}
        </UiButton>
      </div>

      <ConsoleSystemDataState
        :pending="pending"
        :error="error"
        :empty="methods.length === 0"
        :empty-icon="CreditCard"
        :empty-title="t('system.methodsEmptyTitle')"
        :empty-description="t('system.methodsEmptyBody')"
        :rows="3"
      >
        <UiTable>
          <thead>
            <tr>
              <th>{{ t('system.methodCode') }}</th>
              <th>{{ t('system.methodName') }}</th>
              <th>{{ t('common.status') }}</th>
              <th class="num">{{ t('common.createdAt') }}</th>
              <th>{{ t('common.actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="method in methods" :key="method.id">
              <td class="font-mono text-[13px]">{{ method.code }}</td>
              <td>{{ method.name }}</td>
              <td>
                <UiBadge :tone="method.enabled ? 'success' : 'neutral'" dot>
                  {{ method.enabled ? t('common.enabled') : t('common.disabled') }}
                </UiBadge>
              </td>
              <td class="num">{{ formatDateTime(method.created_at) }}</td>
              <td>
                <div class="flex items-center gap-1">
                  <UiButton variant="ghost" size="sm" @click="openEditMethod(method)">
                    {{ t('common.edit') }}
                  </UiButton>
                  <UiButton variant="ghost" size="sm" @click="removingMethod = method">
                    {{ t('common.delete') }}
                  </UiButton>
                </div>
              </td>
            </tr>
          </tbody>
        </UiTable>
      </ConsoleSystemDataState>
    </div>

    <ConsoleSystemPaymentMethodDialog
      v-model:open="methodDialogOpen"
      :method="editingMethod"
      :busy="busy"
      @submit="submitMethod"
    />

    <ConsoleSystemConfirmDialog
      :open="removingMethod !== null"
      :body="t('system.deleteMethodBody', { name: removingMethod?.name ?? removingMethod?.code ?? '' })"
      :busy="busy"
      @update:open="value => { if (!value) removingMethod = null }"
      @confirm="confirmDeleteMethod"
    />
  </ConsoleSystemGate>
</template>

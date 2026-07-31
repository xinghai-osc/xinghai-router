<script setup lang="ts">
import { BadgeDollarSign } from 'lucide-vue-next'
import { endpoints, type Pricing, type PricingForm } from '~/src/api'
import { formatDateTime, formatMoney } from '~/src/format'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { can } = useAccount()
const { toast } = useToast()
const { busy, run } = useAction()

const allowed = computed(() => can('pricing.read'))
const canManage = computed(() => can('pricing.manage'))

const pricing = useResource(() => endpoints.getAdminPricing(), { data: [] as Pricing[] })

const search = ref('')

const filtered = computed(() => {
  const term = search.value.trim().toLowerCase()
  if (!term) return pricing.data.value.data
  return pricing.data.value.data.filter(rule => rule.model.toLowerCase().includes(term))
})

const dialogOpen = ref(false)
const editing = ref(false)
const formError = ref('')
const form = reactive({
  model: '',
  input_per_million: '0',
  cached_input_per_million: '0',
  output_per_million: '0',
  multiplier: '1',
})

function openCreate() {
  editing.value = false
  formError.value = ''
  form.model = ''
  form.input_per_million = '0'
  form.cached_input_per_million = '0'
  form.output_per_million = '0'
  form.multiplier = '1'
  dialogOpen.value = true
}

function openEdit(rule: Pricing) {
  editing.value = true
  formError.value = ''
  form.model = rule.model
  form.input_per_million = String(rule.input_per_million)
  form.cached_input_per_million = String(rule.cached_input_per_million)
  form.output_per_million = String(rule.output_per_million)
  form.multiplier = String(rule.multiplier)
  dialogOpen.value = true
}

function rate(value: string): number | null {
  const numeric = Number(value)
  return Number.isFinite(numeric) && numeric >= 0 ? numeric : null
}

async function save() {
  formError.value = ''
  const model = form.model.trim()
  if (!model) { formError.value = t('admin.modelRequired'); return }

  const input = rate(form.input_per_million)
  const cached = rate(form.cached_input_per_million)
  const output = rate(form.output_per_million)
  if (input === null || cached === null || output === null) { formError.value = t('admin.rateInvalid'); return }

  const multiplier = Number(form.multiplier)
  if (!Number.isFinite(multiplier) || multiplier <= 0 || multiplier > 1000) {
    formError.value = t('admin.multiplierInvalid')
    return
  }

  const payload: PricingForm = {
    model,
    input_per_million: input,
    cached_input_per_million: cached,
    output_per_million: output,
    multiplier,
  }

  const ok = await run(() => endpoints.savePricing(payload))
  if (!ok) { toast.error(t('common.actionFailed')); return }
  toast.success(t('admin.pricingSaved'))
  dialogOpen.value = false
  await pricing.refresh()
}

const syncOpen = ref(false)
const syncError = ref('')
const syncForm = reactive({ base_url: '', api_key: '', price_per_quota_unit: '0.002' })

function openSync() {
  syncError.value = ''
  syncForm.base_url = ''
  syncForm.api_key = ''
  syncForm.price_per_quota_unit = '0.002'
  syncOpen.value = true
}

async function sync() {
  syncError.value = ''
  const baseUrl = syncForm.base_url.trim().replace(/\/+$/, '')
  if (!baseUrl) { syncError.value = t('admin.baseUrlRequired'); return }
  const apiKey = syncForm.api_key.trim()
  if (!apiKey) { syncError.value = t('admin.apiKeyRequired'); return }
  const unitPrice = Number(syncForm.price_per_quota_unit)
  if (!Number.isFinite(unitPrice) || unitPrice <= 0) {
    syncError.value = t('admin.pricePerQuotaUnitInvalid')
    return
  }

  let synced = 0
  const ok = await run(async () => {
    const result = await endpoints.syncNewApiPricing({
      base_url: baseUrl,
      api_key: apiKey,
      price_per_quota_unit: unitPrice,
    })
    synced = result.synced
  })
  if (!ok) { toast.error(t('common.actionFailed')); return }

  toast.success(t('admin.syncDone', { count: synced }))
  syncOpen.value = false
  await pricing.refresh()
}
</script>

<template>
  <ConsoleOpsDenied v-if="!allowed" />

  <div v-else class="space-y-4">
    <ConsoleOpsPageHeader :lead="t('admin.pricingLead')">
      <template #actions>
        <ConsoleOpsSearch v-model="search" :placeholder="t('admin.pricingSearchPlaceholder')" />
        <UiButton variant="secondary" size="sm" @click="pricing.refresh()">{{ t('common.refresh') }}</UiButton>
        <template v-if="canManage">
          <UiButton variant="secondary" size="sm" @click="openSync">{{ t('admin.syncNewApi') }}</UiButton>
          <UiButton size="sm" @click="openCreate">{{ t('admin.createPricing') }}</UiButton>
        </template>
      </template>
    </ConsoleOpsPageHeader>

    <UiAlert v-if="!canManage" tone="info">{{ t('admin.readOnlyNotice') }}</UiAlert>

    <ConsoleOpsListState
      :pending="pricing.pending.value"
      :error="pricing.error.value"
      :empty="!pricing.data.value.data.length"
      :empty-icon="BadgeDollarSign"
      :empty-title="t('admin.pricingEmptyTitle')"
      :empty-description="t('admin.pricingEmptyBody')"
    >
      <div v-if="!filtered.length" class="rounded-card border border-line bg-surface">
        <UiEmptyState :title="t('admin.noResultsTitle')" :description="t('admin.noResultsBody')" />
      </div>

      <UiTable v-else>
        <thead>
          <tr>
            <th>{{ t('admin.model') }}</th>
            <th class="num">{{ t('admin.inputPerMillion') }}</th>
            <th class="num">{{ t('admin.cachedInputPerMillion') }}</th>
            <th class="num">{{ t('admin.outputPerMillion') }}</th>
            <th class="num">{{ t('admin.multiplier') }}</th>
            <th>{{ t('common.status') }}</th>
            <th>{{ t('common.updatedAt') }}</th>
            <th v-if="canManage">{{ t('common.actions') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="rule in filtered" :key="rule.id">
            <td class="font-medium text-ink">{{ rule.model }}</td>
            <td class="num">{{ formatMoney(rule.input_per_million, 4) }}</td>
            <td class="num">{{ formatMoney(rule.cached_input_per_million, 4) }}</td>
            <td class="num">{{ formatMoney(rule.output_per_million, 4) }}</td>
            <td class="num">{{ rule.multiplier }}</td>
            <td>
              <UiBadge :tone="rule.enabled ? 'success' : 'neutral'" dot>
                {{ rule.enabled ? t('common.enabled') : t('common.disabled') }}
              </UiBadge>
            </td>
            <td class="text-muted whitespace-nowrap">{{ formatDateTime(rule.updated_at) }}</td>
            <td v-if="canManage">
              <UiButton variant="ghost" size="sm" @click="openEdit(rule)">{{ t('common.edit') }}</UiButton>
            </td>
          </tr>
        </tbody>
      </UiTable>
    </ConsoleOpsListState>

    <UiSlidePanel v-model:open="dialogOpen" :title="editing ? t('admin.editPricing') : t('admin.createPricing')">
      <div class="space-y-4">
        <UiAlert v-if="formError" tone="danger">{{ formError }}</UiAlert>

        <UiField :label="t('admin.model')" required>
          <UiInput v-model="form.model" mono :disabled="editing" />
        </UiField>

        <div class="grid gap-4 sm:grid-cols-2">
          <UiField :label="t('admin.inputPerMillion')" required>
            <UiInput v-model="form.input_per_million" type="number" mono />
          </UiField>
          <UiField :label="t('admin.cachedInputPerMillion')" required>
            <UiInput v-model="form.cached_input_per_million" type="number" mono />
          </UiField>
          <UiField :label="t('admin.outputPerMillion')" required>
            <UiInput v-model="form.output_per_million" type="number" mono />
          </UiField>
          <UiField :label="t('admin.multiplier')" required>
            <UiInput v-model="form.multiplier" type="number" mono />
          </UiField>
        </div>
      </div>

      <template #footer>
        <UiButton variant="secondary" @click="dialogOpen = false">{{ t('common.cancel') }}</UiButton>
        <UiButton :loading="busy" @click="save">{{ t('common.save') }}</UiButton>
      </template>
    </UiSlidePanel>

    <UiSlidePanel v-model:open="syncOpen" :title="t('admin.syncNewApi')" :description="t('admin.syncNewApiLead')">
      <div class="space-y-4">
        <UiAlert v-if="syncError" tone="danger">{{ syncError }}</UiAlert>

        <UiField :label="t('admin.baseUrl')" required>
          <UiInput v-model="syncForm.base_url" mono :placeholder="t('admin.baseUrlPlaceholder')" />
        </UiField>
        <UiField :label="t('admin.apiKey')" :hint="t('admin.apiKeyHintCreate')" required>
          <UiInput v-model="syncForm.api_key" type="password" mono autocomplete="off" />
        </UiField>
        <UiField :label="t('admin.pricePerQuotaUnit')" :hint="t('admin.pricePerQuotaUnitHint')" required>
          <UiInput v-model="syncForm.price_per_quota_unit" type="number" mono />
        </UiField>
      </div>

      <template #footer>
        <UiButton variant="secondary" @click="syncOpen = false">{{ t('common.cancel') }}</UiButton>
        <UiButton :loading="busy" @click="sync">{{ t('admin.startSync') }}</UiButton>
      </template>
    </UiSlidePanel>
  </div>
</template>

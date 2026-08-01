<script setup lang="ts">
import { BadgeDollarSign } from 'lucide-vue-next'
import { endpoints, type Pricing, type PricingForm, type PricingTier, type PricingTierForm, type PricingTimeRule, type PricingTimeRuleForm } from '~/src/api'
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

// --- Tiered pricing ---

const tierPanelOpen = ref(false)
const tierModel = ref('')
const tiers = ref<PricingTier[]>([])
const tierError = ref('')
const tierForm = reactive({
  id: '',
  from_tokens: '0',
  input_per_million: '0',
  cached_input_per_million: '0',
  output_per_million: '0',
})
const editingTier = ref(false)

async function openTiers(rule: Pricing) {
  tierModel.value = rule.model
  tierError.value = ''
  tierForm.id = ''
  tierForm.from_tokens = '0'
  tierForm.input_per_million = '0'
  tierForm.cached_input_per_million = '0'
  tierForm.output_per_million = '0'
  editingTier.value = false
  const ok = await run(async () => {
    const result = await endpoints.getAdminPricingTiers(rule.model)
    tiers.value = result.data
  })
  if (!ok) { toast.error(t('common.actionFailed')); return }
  tierPanelOpen.value = true
}

function startAddTier() {
  editingTier.value = false
  tierError.value = ''
  tierForm.id = ''
  tierForm.from_tokens = '0'
  tierForm.input_per_million = String(tiers.value.length > 0 ? tiers.value[tiers.value.length - 1]?.input_per_million ?? 0 : 0)
  tierForm.cached_input_per_million = String(tiers.value.length > 0 ? tiers.value[tiers.value.length - 1]?.cached_input_per_million ?? 0 : 0)
  tierForm.output_per_million = String(tiers.value.length > 0 ? tiers.value[tiers.value.length - 1]?.output_per_million ?? 0 : 0)
}

function startEditTier(tier: PricingTier) {
  editingTier.value = true
  tierError.value = ''
  tierForm.id = tier.id
  tierForm.from_tokens = String(tier.from_tokens)
  tierForm.input_per_million = String(tier.input_per_million)
  tierForm.cached_input_per_million = String(tier.cached_input_per_million)
  tierForm.output_per_million = String(tier.output_per_million)
}

async function saveTier() {
  tierError.value = ''
  const fromTokens = Number(tierForm.from_tokens)
  if (!Number.isFinite(fromTokens) || fromTokens < 0) { tierError.value = t('admin.tierFromInvalid'); return }
  const input = rate(tierForm.input_per_million)
  const cached = rate(tierForm.cached_input_per_million)
  const output = rate(tierForm.output_per_million)
  if (input === null || cached === null || output === null) { tierError.value = t('admin.rateInvalid'); return }

  const payload: PricingTierForm = {
    id: editingTier.value ? tierForm.id : undefined,
    model: tierModel.value,
    from_tokens: fromTokens,
    input_per_million: input,
    cached_input_per_million: cached,
    output_per_million: output,
  }
  const ok = await run(() => endpoints.savePricingTier(payload))
  if (!ok) { tierError.value = t('common.actionFailed'); return }
  toast.success(t('admin.tierSaved'))
  const refreshOk = await run(async () => {
    const result = await endpoints.getAdminPricingTiers(tierModel.value)
    tiers.value = result.data
  })
  if (!refreshOk) { toast.error(t('common.actionFailed')); return }
  tierForm.id = ''
  editingTier.value = false
}

async function deleteTier(tier: PricingTier) {
  const ok = await run(() => endpoints.deletePricingTier(tier.id, tierModel.value))
  if (!ok) { toast.error(t('common.actionFailed')); return }
  toast.success(t('admin.tierDeleted'))
  const refreshOk = await run(async () => {
    const result = await endpoints.getAdminPricingTiers(tierModel.value)
    tiers.value = result.data
  })
  if (!refreshOk) { toast.error(t('common.actionFailed')); return }
}

// --- Time-based pricing ---

const timePanelOpen = ref(false)
const timeModel = ref('')
const timeRules = ref<PricingTimeRule[]>([])
const timeError = ref('')
const timeForm = reactive({
  id: '',
  name: '',
  start_minute: '0',
  end_minute: '1440',
  weekdays: '1111111',
  input_per_million: '0',
  cached_input_per_million: '0',
  output_per_million: '0',
  enabled: true,
})
const editingTimeRule = ref(false)

const weekdayLabels = computed(() => [
  t('admin.weekdayMon'),
  t('admin.weekdayTue'),
  t('admin.weekdayWed'),
  t('admin.weekdayThu'),
  t('admin.weekdayFri'),
  t('admin.weekdaySat'),
  t('admin.weekdaySun'),
])

const weekdaySelected = computed(() => {
  const arr = Array.from({ length: 7 }, (_, i) => timeForm.weekdays[i] === '1')
  return arr
})

function toggleWeekday(index: number) {
  const chars = timeForm.weekdays.split('')
  chars[index] = chars[index] === '1' ? '0' : '1'
  timeForm.weekdays = chars.join('')
}

function formatMinute(minute: number): string {
  const h = Math.floor(minute / 60)
  const m = minute % 60
  return `${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}`
}

function formatWeekdays(wd: string): string {
  const days = [t('admin.weekdayMon'), t('admin.weekdayTue'), t('admin.weekdayWed'), t('admin.weekdayThu'), t('admin.weekdayFri'), t('admin.weekdaySat'), t('admin.weekdaySun')]
  const active: string[] = []
  for (let i = 0; i < 7; i++) {
    if (wd[i] === '1') active.push(days[i])
  }
  if (active.length === 7) return t('admin.everyDay')
  return active.join(', ')
}

async function openTimeRules(rule: Pricing) {
  timeModel.value = rule.model
  timeError.value = ''
  resetTimeForm()
  const ok = await run(async () => {
    const result = await endpoints.getAdminPricingTimeRules(rule.model)
    timeRules.value = result.data
  })
  if (!ok) { toast.error(t('common.actionFailed')); return }
  timePanelOpen.value = true
}

function resetTimeForm() {
  editingTimeRule.value = false
  timeForm.id = ''
  timeForm.name = ''
  timeForm.start_minute = '0'
  timeForm.end_minute = '1440'
  timeForm.weekdays = '1111111'
  timeForm.input_per_million = '0'
  timeForm.cached_input_per_million = '0'
  timeForm.output_per_million = '0'
  timeForm.enabled = true
}

function startEditTimeRule(rule: PricingTimeRule) {
  editingTimeRule.value = true
  timeError.value = ''
  timeForm.id = rule.id
  timeForm.name = rule.name
  timeForm.start_minute = String(rule.start_minute)
  timeForm.end_minute = String(rule.end_minute)
  timeForm.weekdays = rule.weekdays
  timeForm.input_per_million = String(rule.input_per_million)
  timeForm.cached_input_per_million = String(rule.cached_input_per_million)
  timeForm.output_per_million = String(rule.output_per_million)
  timeForm.enabled = rule.enabled
}

async function saveTimeRule() {
  timeError.value = ''
  const name = timeForm.name.trim()
  if (name.length > 100) { timeError.value = t('admin.timeRuleNameTooLong'); return }
  const start = Number(timeForm.start_minute)
  const end = Number(timeForm.end_minute)
  if (!Number.isFinite(start) || !Number.isFinite(end) || start < 0 || start >= 1440 || end <= 0 || end > 1440 || start === end) {
    timeError.value = t('admin.timeWindowInvalid')
    return
  }
  if (timeForm.weekdays.length !== 7 || !/^[01]{7}$/.test(timeForm.weekdays)) {
    timeError.value = t('admin.weekdaysInvalid')
    return
  }
  const input = rate(timeForm.input_per_million)
  const cached = rate(timeForm.cached_input_per_million)
  const output = rate(timeForm.output_per_million)
  if (input === null || cached === null || output === null) { timeError.value = t('admin.rateInvalid'); return }

  const payload: PricingTimeRuleForm = {
    id: editingTimeRule.value ? timeForm.id : undefined,
    model: timeModel.value,
    name,
    start_minute: start,
    end_minute: end,
    weekdays: timeForm.weekdays,
    input_per_million: input,
    cached_input_per_million: cached,
    output_per_million: output,
    enabled: timeForm.enabled,
  }
  const ok = await run(() => endpoints.savePricingTimeRule(payload))
  if (!ok) { timeError.value = t('common.actionFailed'); return }
  toast.success(t('admin.timeRuleSaved'))
  const refreshOk = await run(async () => {
    const result = await endpoints.getAdminPricingTimeRules(timeModel.value)
    timeRules.value = result.data
  })
  if (!refreshOk) { toast.error(t('common.actionFailed')); return }
  resetTimeForm()
}

async function deleteTimeRule(rule: PricingTimeRule) {
  const ok = await run(() => endpoints.deletePricingTimeRule(rule.id, timeModel.value))
  if (!ok) { toast.error(t('common.actionFailed')); return }
  toast.success(t('admin.timeRuleDeleted'))
  const refreshOk = await run(async () => {
    const result = await endpoints.getAdminPricingTimeRules(timeModel.value)
    timeRules.value = result.data
  })
  if (!refreshOk) { toast.error(t('common.actionFailed')); return }
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
              <div class="flex gap-1">
                <UiButton variant="ghost" size="sm" @click="openEdit(rule)">{{ t('common.edit') }}</UiButton>
                <UiButton variant="ghost" size="sm" @click="openTiers(rule)">{{ t('admin.tiers') }}</UiButton>
                <UiButton variant="ghost" size="sm" @click="openTimeRules(rule)">{{ t('admin.timeRules') }}</UiButton>
              </div>
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

    <!-- Tiered pricing panel -->
    <UiSlidePanel v-model:open="tierPanelOpen" :title="t('admin.tieredPricing')" :description="t('admin.tieredPricingLead', { model: tierModel })">
      <div class="space-y-4">
        <UiAlert v-if="tierError" tone="danger">{{ tierError }}</UiAlert>

        <div v-if="!tiers.length" class="rounded-card border border-line bg-surface">
          <UiEmptyState :title="t('admin.tierEmptyTitle')" :description="t('admin.tierEmptyBody')" />
        </div>

        <UiTable v-else>
          <thead>
            <tr>
              <th class="num">{{ t('admin.fromTokens') }}</th>
              <th class="num">{{ t('admin.inputPerMillion') }}</th>
              <th class="num">{{ t('admin.cachedInputPerMillion') }}</th>
              <th class="num">{{ t('admin.outputPerMillion') }}</th>
              <th v-if="canManage">{{ t('common.actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="tier in tiers" :key="tier.id">
              <td class="num">{{ tier.from_tokens.toLocaleString() }}</td>
              <td class="num">{{ formatMoney(tier.input_per_million, 4) }}</td>
              <td class="num">{{ formatMoney(tier.cached_input_per_million, 4) }}</td>
              <td class="num">{{ formatMoney(tier.output_per_million, 4) }}</td>
              <td v-if="canManage">
                <div class="flex gap-1">
                  <UiButton variant="ghost" size="sm" @click="startEditTier(tier)">{{ t('common.edit') }}</UiButton>
                  <UiButton variant="ghost" size="sm" @click="deleteTier(tier)">{{ t('common.delete') }}</UiButton>
                </div>
              </td>
            </tr>
          </tbody>
        </UiTable>

        <template v-if="canManage">
          <div class="border-t border-line pt-4">
            <h4 class="text-sm font-medium text-ink mb-3">
              {{ editingTier ? t('admin.editTier') : t('admin.addTier') }}
            </h4>
            <div class="grid gap-4 sm:grid-cols-2">
              <UiField :label="t('admin.fromTokens')" :hint="t('admin.fromTokensHint')" required>
                <UiInput v-model="tierForm.from_tokens" type="number" mono />
              </UiField>
              <UiField :label="t('admin.inputPerMillion')" required>
                <UiInput v-model="tierForm.input_per_million" type="number" mono />
              </UiField>
              <UiField :label="t('admin.cachedInputPerMillion')" required>
                <UiInput v-model="tierForm.cached_input_per_million" type="number" mono />
              </UiField>
              <UiField :label="t('admin.outputPerMillion')" required>
                <UiInput v-model="tierForm.output_per_million" type="number" mono />
              </UiField>
            </div>
            <div class="flex gap-2 mt-4">
              <UiButton v-if="editingTier" variant="secondary" size="sm" @click="startAddTier">{{ t('common.cancel') }}</UiButton>
              <UiButton size="sm" :loading="busy" @click="saveTier">{{ editingTier ? t('common.save') : t('admin.addTier') }}</UiButton>
            </div>
          </div>
        </template>
      </div>
    </UiSlidePanel>

    <!-- Time-based pricing panel -->
    <UiSlidePanel v-model:open="timePanelOpen" :title="t('admin.timeBasedPricing')" :description="t('admin.timeBasedPricingLead', { model: timeModel })">
      <div class="space-y-4">
        <UiAlert v-if="timeError" tone="danger">{{ timeError }}</UiAlert>

        <div v-if="!timeRules.length" class="rounded-card border border-line bg-surface">
          <UiEmptyState :title="t('admin.timeRuleEmptyTitle')" :description="t('admin.timeRuleEmptyBody')" />
        </div>

        <UiTable v-else>
          <thead>
            <tr>
              <th>{{ t('admin.timeRuleName') }}</th>
              <th>{{ t('admin.timeWindow') }}</th>
              <th>{{ t('admin.weekdays') }}</th>
              <th class="num">{{ t('admin.inputPerMillion') }}</th>
              <th class="num">{{ t('admin.outputPerMillion') }}</th>
              <th>{{ t('common.status') }}</th>
              <th v-if="canManage">{{ t('common.actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="rule in timeRules" :key="rule.id">
              <td class="font-medium text-ink">{{ rule.name || '—' }}</td>
              <td class="whitespace-nowrap">{{ formatMinute(rule.start_minute) }}–{{ formatMinute(rule.end_minute) }}</td>
              <td class="text-muted text-sm">{{ formatWeekdays(rule.weekdays) }}</td>
              <td class="num">{{ formatMoney(rule.input_per_million, 4) }}</td>
              <td class="num">{{ formatMoney(rule.output_per_million, 4) }}</td>
              <td>
                <UiBadge :tone="rule.enabled ? 'success' : 'neutral'" dot>
                  {{ rule.enabled ? t('common.enabled') : t('common.disabled') }}
                </UiBadge>
              </td>
              <td v-if="canManage">
                <div class="flex gap-1">
                  <UiButton variant="ghost" size="sm" @click="startEditTimeRule(rule)">{{ t('common.edit') }}</UiButton>
                  <UiButton variant="ghost" size="sm" @click="deleteTimeRule(rule)">{{ t('common.delete') }}</UiButton>
                </div>
              </td>
            </tr>
          </tbody>
        </UiTable>

        <template v-if="canManage">
          <div class="border-t border-line pt-4">
            <h4 class="text-sm font-medium text-ink mb-3">
              {{ editingTimeRule ? t('admin.editTimeRule') : t('admin.addTimeRule') }}
            </h4>
            <div class="space-y-4">
              <UiField :label="t('admin.timeRuleName')" :hint="t('admin.timeRuleNameHint')">
                <UiInput v-model="timeForm.name" mono />
              </UiField>
              <div class="grid gap-4 sm:grid-cols-2">
                <UiField :label="t('admin.startTime')" required>
                  <UiInput v-model="timeForm.start_minute" type="number" mono :placeholder="'0-1439'" />
                </UiField>
                <UiField :label="t('admin.endTime')" required>
                  <UiInput v-model="timeForm.end_minute" type="number" mono :placeholder="'1-1440'" />
                </UiField>
              </div>
              <UiField :label="t('admin.weekdays')">
                <div class="flex gap-1">
                  <UiCheckbox
                    v-for="(label, i) in weekdayLabels"
                    :key="i"
                    :model-value="weekdaySelected[i]"
                    @update:model-value="toggleWeekday(i)"
                  >
                    {{ label }}
                  </UiCheckbox>
                </div>
              </UiField>
              <div class="grid gap-4 sm:grid-cols-2">
                <UiField :label="t('admin.inputPerMillion')" required>
                  <UiInput v-model="timeForm.input_per_million" type="number" mono />
                </UiField>
                <UiField :label="t('admin.cachedInputPerMillion')" required>
                  <UiInput v-model="timeForm.cached_input_per_million" type="number" mono />
                </UiField>
                <UiField :label="t('admin.outputPerMillion')" required>
                  <UiInput v-model="timeForm.output_per_million" type="number" mono />
                </UiField>
                <UiField :label="t('common.status')">
                  <UiSwitch v-model="timeForm.enabled" />
                </UiField>
              </div>
            </div>
            <div class="flex gap-2 mt-4">
              <UiButton v-if="editingTimeRule" variant="secondary" size="sm" @click="resetTimeForm">{{ t('common.cancel') }}</UiButton>
              <UiButton size="sm" :loading="busy" @click="saveTimeRule">{{ editingTimeRule ? t('common.save') : t('admin.addTimeRule') }}</UiButton>
            </div>
          </div>
        </template>
      </div>
    </UiSlidePanel>
  </div>
</template>

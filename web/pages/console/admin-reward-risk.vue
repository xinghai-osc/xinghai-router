<script setup lang="ts">
import { ShieldAlert } from 'lucide-vue-next'
import { endpoints, type Page, type RewardRiskBan, type RewardRiskClaim, type RewardRiskEvent, type RewardRiskSettings, type RewardRiskSettingsForm } from '~/src/api'
import { formatDateTime, formatMoney } from '~/src/format'
import { validStunUrl } from '~/src/risk-context'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { can } = useAccount()
const { settings: site } = useSiteSettings()
const { toast } = useToast()
const { rewardLabel, rewardTone } = useRewardStatus()
const { busy, run, error: actionError } = useAction()
const tabs = computed(() => [
  ...(can('users.read') ? [{ value: 'claims', label: t('admin.rewardRiskClaims') }] : []),
  ...(can('audit.read') ? [{ value: 'events', label: t('admin.rewardRiskEvents') }] : []),
  ...(can('users.read') ? [{ value: 'bans', label: t('admin.rewardRiskBans') }] : []),
  ...(can('system.manage') ? [{ value: 'settings', label: t('system.rewardRiskSettings') }] : []),
])
const tab = ref(tabs.value[0]?.value ?? '')
const allowed = computed(() => tabs.value.length > 0)
const page = ref(1)
const search = ref('')
const appliedSearch = ref('')
const status = ref('pending')
const source = ref('')
const decision = ref('')
const banStatus = ref('active')
const filterStatus = ref('pending')
const filterSource = ref('')
const filterDecision = ref('')
const filterBanStatus = ref('active')
const mutationError = ref('')

useHead({ title: () => `${t('admin.rewardRiskTitle')} · ${site.value.name}` })

function emptyPage<T>(): Page<T> { return { data: [], total: 0, page: 1, page_size: 50 } }
function query(kind: string) {
  const params = new URLSearchParams({ page: String(page.value) })
  if (appliedSearch.value) params.set('q', appliedSearch.value)
  if (kind === 'claims') {
    if (status.value) params.set('status', status.value)
    if (source.value) params.set('source', source.value)
  }
  if (kind === 'events' && decision.value) params.set('decision', decision.value)
  if (kind === 'bans' && banStatus.value) params.set('status', banStatus.value)
  return `?${params}`
}
const claims = useResource(() => endpoints.getRewardRiskClaims(query('claims')), emptyPage<RewardRiskClaim>(), { immediate: false })
const events = useResource(() => endpoints.getRewardRiskEvents(query('events')), emptyPage<RewardRiskEvent>(), { immediate: false })
const bans = useResource(() => endpoints.getRewardRiskBans(query('bans')), emptyPage<RewardRiskBan>(), { immediate: false })
const defaults: RewardRiskSettings = {
  enabled: false, auto_ban_enabled: false, webrtc_enabled: false, stun_urls: [],
  window_hours: 24, username_similarity: 0.85, similar_accounts: 3, burst_minutes: 10,
  burst_accounts: 5, registrations_per_ip: 3, checkins_per_ip: 3, checkins_per_browser: 1,
  invitations_per_inviter: 10, version: 0,
}
const settings = useResource(() => endpoints.getRewardRiskSettings(), { ...defaults }, { immediate: false })
const form = ref<RewardRiskSettingsForm>(writableSettings(defaults))
const stunUrls = ref('')
const settingsLoaded = ref(false)
function writableSettings(value: RewardRiskSettings): RewardRiskSettingsForm {
  const { version: _version, ...writable } = value
  return { ...writable, stun_urls: [...value.stun_urls] }
}
watch(settings.data, (value) => {
  form.value = writableSettings(value)
  stunUrls.value = value.stun_urls.join('\n')
  settingsLoaded.value = true
})
const numericFields: { key: Exclude<keyof RewardRiskSettingsForm, 'enabled' | 'auto_ban_enabled' | 'webrtc_enabled' | 'stun_urls'>; label: string; min: number; max: number }[] = [
  { key: 'window_hours', label: 'system.rewardRiskWindowHours', min: 1, max: 720 },
  { key: 'username_similarity', label: 'system.rewardRiskUsernameSimilarity', min: 0.7, max: 1 },
  { key: 'similar_accounts', label: 'system.rewardRiskSimilarAccounts', min: 3, max: 500 },
  { key: 'burst_minutes', label: 'system.rewardRiskBurstMinutes', min: 1, max: 1440 },
  { key: 'burst_accounts', label: 'system.rewardRiskBurstAccounts', min: 3, max: 500 },
  { key: 'registrations_per_ip', label: 'system.rewardRiskRegistrationsPerIp', min: 1, max: 10000 },
  { key: 'checkins_per_ip', label: 'system.rewardRiskCheckinsPerIp', min: 1, max: 10000 },
  { key: 'checkins_per_browser', label: 'system.rewardRiskCheckinsPerBrowser', min: 1, max: 10000 },
  { key: 'invitations_per_inviter', label: 'system.rewardRiskInvitationsPerInviter', min: 1, max: 10000 },
]
const activeList = computed(() => tab.value === 'claims' ? claims : tab.value === 'events' ? events : tab.value === 'bans' ? bans : null)
const pending = computed(() => tab.value === 'settings' ? settings.pending.value : activeList.value?.pending.value ?? false)
const loadError = computed(() => tab.value === 'settings' ? settings.error.value : activeList.value?.error.value ?? '')
const total = computed(() => activeList.value?.data.value.total ?? 0)
const empty = computed(() => !activeList.value?.data.value.data.length)
const hasNext = computed(() => {
  const result = activeList.value?.data.value
  return result ? result.page * result.page_size < result.total : false
})
const allOption = computed(() => ({ value: '', label: t('admin.rewardRiskAll') }))
const statusOptions = computed(() => [allOption.value, ...['pending', 'credited', 'rejected', 'withdrawn'].map(value => ({ value, label: rewardLabel(value) }))])
const sourceOptions = computed(() => [allOption.value, { value: 'invitation', label: t('admin.rewardRiskInvitation') }, { value: 'checkin', label: t('admin.rewardRiskCheckin') }])
const decisionOptions = computed(() => [allOption.value, ...['allow', 'review', 'ban'].map(value => ({ value, label: decisionLabel(value) }))])
const banOptions = computed(() => [allOption.value, { value: 'active', label: t('admin.rewardRiskActive') }, { value: 'released', label: t('admin.rewardRiskReleased') }])

async function refresh() {
  if (tab.value === 'claims' && can('users.read')) await claims.refresh()
  else if (tab.value === 'events' && can('audit.read')) await events.refresh()
  else if (tab.value === 'bans' && can('users.read')) await bans.refresh()
  else if (tab.value === 'settings' && can('system.manage')) await settings.refresh()
}
onMounted(refresh)
watch(tabs, (items) => { if (!items.some(item => item.value === tab.value)) tab.value = items[0]?.value ?? '' })
watch(tab, () => {
  page.value = 1
  target.value = null
  evidence.value = null
  mutationError.value = ''
  if (import.meta.client) void refresh()
})
function applyFilters() {
  if (pending.value) return
  appliedSearch.value = search.value.trim()
  status.value = filterStatus.value
  source.value = filterSource.value
  decision.value = filterDecision.value
  banStatus.value = filterBanStatus.value
  page.value = 1
  void refresh()
}
function changePage(delta: number) {
  if (pending.value) return
  page.value = Math.max(1, page.value + delta)
  void refresh()
}

const reasonKeys = new Set(['registration_ip_limit', 'checkin_ip_limit', 'checkin_browser_limit', 'inviter_daily_limit', 'shared_browser_invitation', 'username_cluster', 'webrtc_username_cluster', 'account_burst', 'automatic_ban', 'candidate_overflow', 'unverified_context'])
function reasonLabel(reason: string) { return reasonKeys.has(reason) ? t(`admin.rewardRiskReason_${reason}`) : reason }
function decisionLabel(value: string) {
  const keys: Record<string, string> = { allow: 'admin.rewardRiskAllow', review: 'admin.rewardRiskReview', ban: 'admin.rewardRiskBan' }
  return keys[value] ? t(keys[value]) : value
}
function sourceLabel(value: string) { return value === 'invitation' ? t('admin.rewardRiskInvitation') : value === 'checkin' ? t('admin.rewardRiskCheckin') : value }
const evidence = ref<RewardRiskEvent | RewardRiskBan | null>(null)
const target = ref<{ kind: 'approve' | 'reject'; claim: RewardRiskClaim } | { kind: 'unban'; ban: RewardRiskBan } | null>(null)
const reason = ref('')
const actionTitle = computed(() => target.value?.kind === 'unban' ? t('admin.rewardRiskUnban') : target.value?.kind === 'reject' ? t('admin.rewardRiskReject') : t('admin.rewardRiskApprove'))
const actionHint = computed(() => target.value?.kind === 'unban' ? t('admin.rewardRiskUnbanHint') : target.value?.kind === 'reject' ? t('admin.rewardRiskRejectHint') : t('admin.rewardRiskApproveHint'))
function openClaim(claim: RewardRiskClaim, kind: 'approve' | 'reject') {
  if (!can('wallets.manage') || claim.status !== 'pending') return
  target.value = { kind, claim }
  reason.value = ''
  mutationError.value = ''
}
function openUnban(ban: RewardRiskBan) {
  if (!can('users.manage') || ban.released_at) return
  target.value = { kind: 'unban', ban }
  reason.value = ''
  mutationError.value = ''
}
async function submitDecision() {
  const action = target.value
  if (!action || busy.value || !can(action.kind === 'unban' ? 'users.manage' : 'wallets.manage')) return
  mutationError.value = ''
  const note = reason.value.trim()
  if (action.kind === 'unban' && !note) { mutationError.value = t('admin.rewardRiskReasonRequired'); return }
  const ok = await run(() => action.kind === 'unban'
    ? endpoints.unbanRewardRiskUser(action.ban.user_id, action.ban.id, note)
    : action.kind === 'approve' ? endpoints.approveRewardRiskClaim(action.claim.id, note || undefined) : endpoints.rejectRewardRiskClaim(action.claim.id, note || undefined))
  if (!ok) { mutationError.value = actionError.value; return }
  target.value = null
  toast.success(t('admin.rewardRiskActionSaved'))
  await refresh()
}
async function saveSettings() {
  if (!can('system.manage') || busy.value || !settingsLoaded.value) return
  mutationError.value = ''
  for (const field of numericFields) {
    const value = Number(form.value[field.key])
    if (!Number.isFinite(value) || value < field.min || value > field.max || (field.key !== 'username_similarity' && !Number.isInteger(value))) {
      mutationError.value = t('system.rewardRiskInvalid')
      return
    }
    form.value[field.key] = value
  }
  if (form.value.burst_minutes > form.value.window_hours * 60 || form.value.burst_accounts < form.value.similar_accounts) {
    mutationError.value = t('system.rewardRiskInvalid')
    return
  }
  const urls = [...new Set(stunUrls.value.split('\n').map(value => value.trim()).filter(Boolean))]
  if (urls.length > 2 || urls.some(value => !validStunUrl(value))) { mutationError.value = t('system.rewardRiskStunInvalid'); return }
  const ok = await run(() => endpoints.updateRewardRiskSettings({ ...form.value, stun_urls: urls }))
  if (!ok) { mutationError.value = actionError.value; return }
  toast.success(t('system.rewardRiskSaved'))
  await settings.refresh()
}
</script>

<template>
  <UiEmptyState v-if="!allowed" :icon="ShieldAlert" :title="t('admin.noAccessTitle')" :description="t('admin.noAccessBody')" />
  <div v-else class="space-y-4">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div><h2 class="text-lg font-semibold text-ink">{{ t('admin.rewardRiskTitle') }}</h2><p class="mt-1 text-sm text-muted">{{ t('admin.rewardRiskLead') }}</p></div>
      <UiButton variant="secondary" :disabled="pending || busy" @click="refresh">{{ t('common.refresh') }}</UiButton>
    </div>
    <UiTabs v-model="tab" :items="tabs" />
    <form v-if="tab !== 'settings'" class="flex flex-wrap items-end gap-3" @submit.prevent="applyFilters">
      <UiField :label="t('common.search')" class="min-w-48 flex-1"><UiInput v-model="search" :placeholder="t('admin.rewardRiskSearch')" :disabled="pending" /></UiField>
      <UiField v-if="tab === 'claims'" :label="t('common.status')"><UiSelect v-model="filterStatus" :options="statusOptions" :disabled="pending" /></UiField>
      <UiField v-if="tab === 'claims'" :label="t('admin.rewardRiskSource')"><UiSelect v-model="filterSource" :options="sourceOptions" :disabled="pending" /></UiField>
      <UiField v-if="tab === 'events'" :label="t('admin.rewardRiskDecision')"><UiSelect v-model="filterDecision" :options="decisionOptions" :disabled="pending" /></UiField>
      <UiField v-if="tab === 'bans'" :label="t('common.status')"><UiSelect v-model="filterBanStatus" :options="banOptions" :disabled="pending" /></UiField>
      <UiButton type="submit" variant="secondary" :disabled="pending">{{ t('common.filter') }}</UiButton>
    </form>
    <UiAlert v-if="loadError" tone="danger" :title="t('common.loadFailed')">{{ loadError }}<UiButton variant="link" :disabled="pending" @click="refresh">{{ t('common.retry') }}</UiButton></UiAlert>
    <UiSkeleton v-else-if="pending" :rows="6" />
    <template v-else-if="tab === 'settings' && can('system.manage') && settingsLoaded">
      <UiCard :title="t('system.rewardRiskSettings')" :description="t('system.rewardRiskLead')">
        <form class="space-y-5" @submit.prevent="saveSettings">
          <p class="text-xs text-muted">{{ t('system.rewardRiskVersion') }}: <span class="numeric">{{ settings.data.value.version }}</span></p>
          <UiSwitch v-model="form.enabled" :label="t('system.rewardRiskEnabled')" />
          <UiField :hint="t('system.rewardRiskAutoBanHint')"><UiSwitch v-model="form.auto_ban_enabled" :label="t('system.rewardRiskAutoBan')" /></UiField>
          <UiField :hint="t('system.rewardRiskWebrtcHint')"><UiSwitch v-model="form.webrtc_enabled" :label="t('system.rewardRiskWebrtc')" /></UiField>
          <UiField :label="t('system.rewardRiskStun')" :hint="t('system.rewardRiskStunHint')"><UiTextarea v-model="stunUrls" :rows="3" mono /></UiField>
          <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
            <UiField v-for="field in numericFields" :key="field.key" :label="t(field.label)" :hint="t('system.rewardRiskRange', { min: field.min, max: field.max })" required><UiInput v-model.number="form[field.key]" type="number" :min="field.min" :max="field.max" :step="field.key === 'username_similarity' ? 0.01 : 1" /></UiField>
          </div>
          <UiAlert v-if="mutationError" tone="danger">{{ mutationError }}</UiAlert>
          <div class="flex justify-end"><UiButton type="submit" :loading="busy">{{ t('common.save') }}</UiButton></div>
        </form>
      </UiCard>
    </template>
    <template v-else-if="tab !== 'settings'">
      <UiEmptyState v-if="empty" :icon="ShieldAlert" :title="t('admin.rewardRiskEmpty')" :description="t('admin.rewardRiskEmptyHint')" />
      <UiCard v-else flush>
        <UiTable v-if="tab === 'claims' && can('users.read')">
          <thead><tr><th>{{ t('common.name') }}</th><th>{{ t('admin.rewardRiskSource') }}</th><th class="num">{{ t('console.checkinReward') }}</th><th>{{ t('common.status') }}</th><th>{{ t('admin.rewardRiskReasons') }}</th><th>{{ t('common.createdAt') }}</th><th>{{ t('common.actions') }}</th></tr></thead>
          <tbody><tr v-for="claim in claims.data.value.data" :key="claim.id">
            <td><p class="font-medium text-ink">{{ claim.user_name }}</p><p class="text-xs text-muted">{{ claim.email }}</p><p class="numeric text-xs text-faint">{{ claim.user_id }}</p></td>
            <td>{{ sourceLabel(claim.source) }}<p class="text-xs text-muted">{{ t('admin.rewardRiskOriginUser') }}: {{ claim.origin_user_id }}</p><p class="text-xs text-muted">{{ t('admin.rewardRiskSourceId') }}: {{ claim.source_id }}</p></td>
            <td class="num">{{ formatMoney(claim.amount, 4) }}</td><td><UiBadge :tone="rewardTone(claim.status)">{{ rewardLabel(claim.status) }}</UiBadge></td>
            <td><ul class="space-y-1 text-xs text-muted"><li v-for="item in claim.reasons" :key="item">{{ reasonLabel(item) }}</li></ul></td><td class="whitespace-nowrap text-muted">{{ formatDateTime(claim.created_at) }}</td>
            <td><div v-if="claim.status === 'pending' && can('wallets.manage')" class="flex gap-2"><UiButton size="sm" :disabled="busy" @click="openClaim(claim, 'approve')">{{ t('admin.rewardRiskApprove') }}</UiButton><UiButton size="sm" variant="danger" :disabled="busy" @click="openClaim(claim, 'reject')">{{ t('admin.rewardRiskReject') }}</UiButton></div></td>
          </tr></tbody>
        </UiTable>
        <UiTable v-else-if="tab === 'events' && can('audit.read')">
          <thead><tr><th>{{ t('common.name') }}</th><th>{{ t('admin.rewardRiskAction') }}</th><th>{{ t('admin.rewardRiskDecision') }}</th><th>{{ t('admin.rewardRiskReasons') }}</th><th>{{ t('common.createdAt') }}</th><th>{{ t('common.detail') }}</th></tr></thead>
          <tbody><tr v-for="event in events.data.value.data" :key="event.id">
            <td><p>{{ event.user_name }}</p><p class="text-xs text-muted">{{ event.email }}</p><p class="numeric text-xs text-faint">{{ event.user_id }}</p></td><td>{{ event.action }}</td><td><UiBadge :tone="event.decision === 'allow' ? 'success' : event.decision === 'ban' ? 'danger' : 'warn'">{{ decisionLabel(event.decision) }}</UiBadge></td>
            <td><ul class="space-y-1 text-xs text-muted"><li v-for="item in event.reasons" :key="item">{{ reasonLabel(item) }}</li></ul></td><td class="whitespace-nowrap text-muted">{{ formatDateTime(event.created_at) }}</td><td><UiButton size="sm" variant="secondary" @click="evidence = event">{{ t('common.detail') }}</UiButton></td>
          </tr></tbody>
        </UiTable>
        <UiTable v-else-if="tab === 'bans' && can('users.read')">
          <thead><tr><th>{{ t('common.name') }}</th><th>{{ t('common.status') }}</th><th>{{ t('system.rewardRiskVersion') }}</th><th>{{ t('admin.rewardRiskBannedAt') }}</th><th>{{ t('admin.rewardRiskReleasedAt') }}</th><th>{{ t('common.actions') }}</th></tr></thead>
          <tbody><tr v-for="ban in bans.data.value.data" :key="ban.id">
            <td><p>{{ ban.user_name }}</p><p class="text-xs text-muted">{{ ban.email }}</p><p class="numeric text-xs text-faint">{{ ban.user_id }}</p></td><td><UiBadge :tone="ban.released_at ? 'neutral' : 'danger'">{{ ban.released_at ? t('admin.rewardRiskReleased') : t('admin.rewardRiskActive') }}</UiBadge></td><td class="num">{{ ban.rule_version }}</td><td class="whitespace-nowrap text-muted">{{ formatDateTime(ban.banned_at) }}</td><td class="text-muted"><template v-if="ban.released_at">{{ formatDateTime(ban.released_at) }}<p class="text-xs">{{ t('admin.rewardRiskReleasedBy') }}: {{ ban.released_by }}</p><p class="text-xs">{{ ban.release_reason }}</p></template></td>
            <td><div class="flex gap-2"><UiButton v-if="can('audit.read')" size="sm" variant="secondary" @click="evidence = ban">{{ t('common.detail') }}</UiButton><UiButton v-if="!ban.released_at && can('users.manage')" size="sm" :disabled="busy" @click="openUnban(ban)">{{ t('admin.rewardRiskUnban') }}</UiButton></div></td>
          </tr></tbody>
        </UiTable>
      </UiCard>
      <div class="flex flex-wrap items-center justify-between gap-3">
        <p class="text-xs text-muted">{{ t('admin.rewardRiskPage', { page, total }) }}</p>
        <div class="flex gap-2"><UiButton variant="secondary" size="sm" :disabled="page <= 1 || pending" @click="changePage(-1)">{{ t('common.prev') }}</UiButton><UiButton variant="secondary" size="sm" :disabled="!hasNext || pending" @click="changePage(1)">{{ t('common.next') }}</UiButton></div>
      </div>
    </template>
    <UiDialog :open="target !== null" :title="actionTitle" :description="actionHint" @update:open="value => { if (!value && !busy) target = null }">
      <div v-if="target" class="space-y-4">
        <p class="text-sm text-ink">{{ target.kind === 'unban' ? target.ban.email : target.claim.email }}</p>
        <p v-if="target.kind !== 'unban'" class="numeric text-sm">{{ formatMoney(target.claim.amount, 4) }}</p>
        <UiField :label="t('admin.rewardRiskReason')" :hint="t('admin.rewardRiskReasonHint')" :required="target.kind === 'unban'"><UiTextarea v-model="reason" :rows="3" :disabled="busy" /></UiField>
        <UiAlert v-if="mutationError" tone="danger">{{ mutationError }}</UiAlert>
      </div>
      <template #footer><UiButton variant="secondary" :disabled="busy" @click="target = null">{{ t('common.cancel') }}</UiButton><UiButton :variant="target?.kind === 'reject' ? 'danger' : 'primary'" :loading="busy" @click="submitDecision">{{ actionTitle }}</UiButton></template>
    </UiDialog>
    <UiDialog v-if="can('audit.read')" :open="evidence !== null" size="lg" :title="t('admin.rewardRiskDetails')" @update:open="value => { if (!value) evidence = null }">
      <template v-if="evidence"><p class="mb-3 text-sm text-muted">{{ evidence.email }}</p><pre class="overflow-auto rounded-control bg-sunken p-4 text-xs text-ink">{{ JSON.stringify(evidence, null, 2) }}</pre></template>
    </UiDialog>
  </div>
</template>

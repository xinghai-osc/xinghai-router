<script setup lang="ts">
import { ShieldAlert } from 'lucide-vue-next'
import { endpoints, type ContentPolicyRule, type ContentPolicySettings } from '~/src/api'
import { formatDateTime } from '~/src/format'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { settings: site } = useSiteSettings()
const { toast } = useToast()
const { busy, run } = useAction()

useHead({ title: () => `${t('system.contentPolicyTitle')} · ${site.value.name}` })

const defaults: ContentPolicySettings = { request_audit_enabled: true, request_audit_store_mode: 'hash', request_audit_retention_days: 30, content_policy_mode: 'off' }
const settings = useResource(() => endpoints.getContentPolicySettings(), { ...defaults })
const rules = useResource(() => endpoints.getContentPolicyRules(), { data: [] as ContentPolicyRule[] })
const form = reactive<ContentPolicySettings>({ ...defaults })
const draft = reactive({ name: '', term: '', action: 'block' as ContentPolicyRule['action'], case_sensitive: false, enabled: true, priority: 100 })
const editing = ref<string | null>(null)

watch(settings.data, value => Object.assign(form, value), { immediate: true })

const modeOptions = computed(() => [
  { value: 'off', label: t('system.contentPolicyModeOff') },
  { value: 'audit', label: t('system.contentPolicyModeAudit') },
  { value: 'block', label: t('system.contentPolicyModeBlock') },
])
const storeOptions = computed(() => [
  { value: 'none', label: t('system.contentPolicyStoreNone') },
  { value: 'hash', label: t('system.contentPolicyStoreHash') },
  { value: 'excerpt', label: t('system.contentPolicyStoreExcerpt') },
])

async function saveSettings() {
  const ok = await run(() => endpoints.updateContentPolicySettings(form))
  if (ok) { toast.success(t('system.contentPolicySaved')); await settings.refresh() }
  else toast.error(t('common.actionFailed'))
}

function resetDraft() {
  draft.name = ''; draft.term = ''; draft.action = 'block'; draft.case_sensitive = false; draft.enabled = true; draft.priority = 100; editing.value = null
}

function editRule(rule: ContentPolicyRule) {
  editing.value = rule.id
  draft.name = rule.name; draft.term = rule.term; draft.action = rule.action; draft.case_sensitive = rule.case_sensitive; draft.enabled = rule.enabled; draft.priority = rule.priority
}

async function saveRule() {
  if (!draft.name.trim() || !draft.term.trim()) { toast.error(t('system.contentPolicyRuleRequired')); return }
  const payload = { name: draft.name.trim(), term: draft.term.trim(), action: draft.action, case_sensitive: draft.case_sensitive, enabled: draft.enabled, priority: Number(draft.priority) || 0 }
  const ok = await run(() => editing.value ? endpoints.updateContentPolicyRule(editing.value, payload) : endpoints.createContentPolicyRule(payload))
  if (ok) { toast.success(editing.value ? t('system.contentPolicyRuleUpdated') : t('system.contentPolicyRuleCreated')); resetDraft(); await rules.refresh() }
  else toast.error(t('common.actionFailed'))
}

async function removeRule(rule: ContentPolicyRule) {
  if (!window.confirm(t('system.contentPolicyRuleDeleteConfirm', { name: rule.name }))) return
  const ok = await run(() => endpoints.deleteContentPolicyRule(rule.id))
  if (ok) { toast.success(t('system.contentPolicyRuleDeleted')); await rules.refresh() }
  else toast.error(t('common.actionFailed'))
}
</script>

<template>
  <ConsoleSystemGate permission="system.manage">
    <div class="space-y-4">
      <div class="min-w-0 space-y-1">
        <h2 class="text-lg font-semibold text-ink">{{ t('system.contentPolicyTitle') }}</h2>
        <p class="text-[13px] text-muted">{{ t('system.contentPolicyLead') }}</p>
      </div>
      <UiAlert v-if="settings.error.value || rules.error.value" tone="danger" :title="t('common.loadFailed')">{{ settings.error.value || rules.error.value }}</UiAlert>
      <UiCard :title="t('system.contentPolicySettingsSection')">
        <form class="space-y-4" @submit.prevent="saveSettings">
          <UiField :label="t('system.contentPolicyMode')" :hint="t('system.contentPolicyModeHint')">
            <UiSelect v-model="form.content_policy_mode" :options="modeOptions" :placeholder="t('common.selectPlaceholder')" />
          </UiField>
          <div class="grid gap-4 sm:grid-cols-2">
            <UiField :label="t('system.contentPolicyStoreMode')" :hint="t('system.contentPolicyStoreHint')">
              <UiSelect v-model="form.request_audit_store_mode" :options="storeOptions" :placeholder="t('common.selectPlaceholder')" />
            </UiField>
            <UiField :label="t('system.contentPolicyRetention')" :hint="t('system.contentPolicyRetentionHint')">
              <UiInput v-model.number="form.request_audit_retention_days" type="number" min="1" max="3650" />
            </UiField>
          </div>
          <UiSwitch v-model="form.request_audit_enabled" :label="t('system.contentPolicyAuditEnabled')" />
          <div class="flex justify-end"><UiButton type="submit" :loading="busy">{{ t('common.save') }}</UiButton></div>
        </form>
      </UiCard>

      <UiCard :title="t('system.contentPolicyRulesSection')" :description="t('system.contentPolicyRulesHint')">
        <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-6">
          <UiInput v-model="draft.name" :placeholder="t('system.contentPolicyRuleName')" />
          <UiInput v-model="draft.term" :placeholder="t('system.contentPolicyRuleTerm')" />
          <UiSelect v-model="draft.action" :options="[{ value: 'block', label: t('system.contentPolicyActionBlock') }, { value: 'audit', label: t('system.contentPolicyActionAudit') }]" />
          <UiInput v-model.number="draft.priority" type="number" :placeholder="t('system.contentPolicyPriority')" />
          <UiSwitch v-model="draft.enabled" :label="t('common.enabled')" />
          <div class="flex gap-2"><UiButton size="sm" :loading="busy" @click="saveRule">{{ editing ? t('common.save') : t('system.contentPolicyAddRule') }}</UiButton><UiButton v-if="editing" variant="secondary" size="sm" @click="resetDraft">{{ t('common.cancel') }}</UiButton></div>
        </div>
        <div class="mt-4">
          <UiSkeleton v-if="rules.pending.value" :rows="3" class="h-10" />
          <UiEmptyState v-else-if="!rules.data.value.data.length" :icon="ShieldAlert" :title="t('system.contentPolicyRulesEmpty')" :description="t('system.contentPolicyRulesEmptyBody')" />
          <UiTable v-else dense>
            <thead><tr><th>{{ t('system.contentPolicyRuleName') }}</th><th>{{ t('system.contentPolicyRuleTerm') }}</th><th>{{ t('system.contentPolicyAction') }}</th><th>{{ t('common.status') }}</th><th>{{ t('common.updatedAt') }}</th><th>{{ t('common.actions') }}</th></tr></thead>
            <tbody><tr v-for="rule in rules.data.value.data" :key="rule.id"><td class="font-medium text-ink">{{ rule.name }}</td><td class="font-mono text-muted">{{ rule.term }}</td><td><UiBadge :tone="rule.action === 'block' ? 'danger' : 'warn'">{{ rule.action === 'block' ? t('system.contentPolicyActionBlock') : t('system.contentPolicyActionAudit') }}</UiBadge></td><td><UiBadge :tone="rule.enabled ? 'success' : 'neutral'">{{ rule.enabled ? t('common.enabled') : t('common.disabled') }}</UiBadge></td><td class="text-muted">{{ formatDateTime(rule.updated_at) }}</td><td class="flex gap-1"><UiButton variant="ghost" size="sm" @click="editRule(rule)">{{ t('common.edit') }}</UiButton><UiButton variant="ghost" size="sm" @click="removeRule(rule)">{{ t('common.delete') }}</UiButton></td></tr></tbody>
          </UiTable>
        </div>
      </UiCard>
    </div>
  </ConsoleSystemGate>
</template>

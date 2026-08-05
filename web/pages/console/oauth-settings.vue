<script setup lang="ts">
import { LogIn, Plus } from 'lucide-vue-next'
import { endpoints, type OAuthProvider, type OAuthProviderForm } from '~/src/api'
import { formatDateTime } from '~/src/format'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { settings: site } = useSiteSettings()
const { toast } = useToast()
const { busy, run } = useAction()

useHead({ title: () => `${t('system.oauthTitle')} · ${site.value.name}` })

const { data, pending, error, refresh } = useResource(
  () => endpoints.getOAuthProviders(),
  { data: [] as OAuthProvider[] },
)

const providers = computed(() => data.value.data ?? [])

const dialogOpen = ref(false)
const editingProvider = ref<OAuthProvider | null>(null)
const removingProvider = ref<OAuthProvider | null>(null)

function openCreate() {
  editingProvider.value = null
  dialogOpen.value = true
}

function openEdit(provider: OAuthProvider) {
  editingProvider.value = provider
  dialogOpen.value = true
}

async function submitProvider(payload: { provider: string; form: OAuthProviderForm }) {
  const ok = await run(() => endpoints.saveOAuthProvider(payload.provider, payload.form))
  if (!ok) {
    toast.error(t('common.actionFailed'))
    return
  }
  toast.success(t('system.oauthProviderSaved'))
  dialogOpen.value = false
  await refresh()
}

async function confirmDeleteProvider() {
  const target = removingProvider.value
  if (!target) return
  const ok = await run(() => endpoints.deleteOAuthProvider(target.id))
  if (!ok) {
    toast.error(t('common.actionFailed'))
    return
  }
  toast.success(t('system.oauthProviderDeleted'))
  removingProvider.value = null
  await refresh()
}
</script>

<template>
  <ConsoleSystemGate permission="system.manage">
    <div class="space-y-4">
      <div class="flex flex-wrap items-end justify-between gap-3">
        <div class="min-w-0 space-y-1">
          <h2 class="text-lg font-semibold text-ink">{{ t('system.oauthTitle') }}</h2>
          <p class="text-[13px] text-muted">{{ t('system.oauthLead') }}</p>
        </div>
        <UiButton size="sm" @click="openCreate">
          <Plus class="size-4" />
          {{ t('system.oauthAddProvider') }}
        </UiButton>
      </div>

      <ConsoleSystemDataState
        :pending="pending"
        :error="error"
        :empty="providers.length === 0"
        :empty-icon="LogIn"
        :empty-title="t('system.oauthNoProviders')"
        :empty-description="t('system.oauthEmptyBody')"
        :rows="3"
      >
        <UiTable>
          <thead>
            <tr>
              <th>{{ t('system.oauthProviderId') }}</th>
              <th>{{ t('common.status') }}</th>
              <th>{{ t('system.oauthClientId') }}</th>
              <th class="num">{{ t('common.createdAt') }}</th>
              <th>{{ t('common.actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="provider in providers" :key="provider.id">
              <td class="font-mono text-[13px]">{{ provider.id }}</td>
              <td>
                <UiBadge :tone="provider.enabled ? 'success' : 'neutral'" dot>
                  {{ provider.enabled ? t('common.enabled') : t('common.disabled') }}
                </UiBadge>
              </td>
              <td class="font-mono text-[13px] text-muted">{{ provider.client_id }}</td>
              <td class="num">{{ formatDateTime(provider.created_at) }}</td>
              <td>
                <div class="flex items-center gap-1">
                  <UiButton variant="ghost" size="sm" @click="openEdit(provider)">
                    {{ t('common.edit') }}
                  </UiButton>
                  <UiButton variant="ghost" size="sm" @click="removingProvider = provider">
                    {{ t('common.delete') }}
                  </UiButton>
                </div>
              </td>
            </tr>
          </tbody>
        </UiTable>
      </ConsoleSystemDataState>
    </div>

    <ConsoleSystemOAuthProviderDialog
      v-model:open="dialogOpen"
      :provider="editingProvider"
      :busy="busy"
      @submit="submitProvider"
    />

    <ConsoleSystemConfirmDialog
      :open="removingProvider !== null"
      :body="t('system.oauthDeleteProviderBody', { name: removingProvider?.id ?? '' })"
      :busy="busy"
      @update:open="value => { if (!value) removingProvider = null }"
      @confirm="confirmDeleteProvider"
    />
  </ConsoleSystemGate>
</template>

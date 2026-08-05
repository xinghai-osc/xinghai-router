<script setup lang="ts">
import { Lock, ScrollText } from 'lucide-vue-next'
import { api, getToken, type PublicActivityItem } from '~/src/api'
import { formatDateTime, formatNumber } from '~/src/format'

const { t } = useI18n()
const { settings } = useSiteSettings()

useHead({
  title: () => `${t('site.activityMetaTitle')} · ${settings.value.name}`,
  meta: [{ name: 'description', content: () => t('site.activityMetaDescription') }],
})

const data = ref<PublicActivityItem[]>([])
const pending = ref(false)
const failure = ref('')
const signedIn = ref(!!getToken())

async function load() {
  if (!signedIn.value) return
  pending.value = true
  failure.value = ''
  try {
    const res = await api<{ data: PublicActivityItem[] }>('/public/activity')
    data.value = res.data
  } catch (cause) {
    if (cause instanceof Error && cause.message === 'invalid or expired session') {
      signedIn.value = false
    } else {
      failure.value = cause instanceof Error ? cause.message : t('common.loadFailed')
    }
  } finally {
    pending.value = false
  }
}

const statusTone = (code: number): 'danger' | 'warn' | 'success' =>
  code >= 400 ? 'danger' : code >= 300 ? 'warn' : 'success'

onMounted(load)
</script>

<template>
  <div>
    <section class="shell pt-16 pb-10 md:pt-20">
      <div class="max-w-2xl space-y-3">
        <p class="text-2xs font-medium tracking-wide text-clay uppercase">{{ t('site.activityEyebrow') }}</p>
        <h1 class="display text-4xl text-ink md:text-5xl">{{ t('site.activityTitle') }}</h1>
        <p class="text-muted">{{ t('site.activityLead') }}</p>
      </div>
    </section>

    <section class="shell pb-24">
      <UiAlert v-if="failure" tone="danger" :title="t('site.activityErrorTitle')">
        {{ failure }}
        <UiButton variant="link" size="sm" class="ml-1 h-auto p-0" @click="load">{{ t('common.retry') }}</UiButton>
      </UiAlert>

      <UiEmptyState
        v-else-if="!signedIn"
        class="rounded-card border border-line bg-surface"
        :icon="Lock"
        :title="t('site.activitySignInTitle')"
        :description="t('site.activitySignInBody')"
      >
        <UiButton to="/auth" size="sm">{{ t('common.signIn') }}</UiButton>
      </UiEmptyState>

      <div v-else-if="pending && !data.length" class="rounded-card border border-line bg-surface p-5">
        <UiSkeleton :rows="8" />
      </div>

      <UiEmptyState
        v-else-if="!data.length"
        class="rounded-card border border-line bg-surface"
        :icon="ScrollText"
        :title="t('site.activityEmptyTitle')"
        :description="t('site.activityEmptyBody')"
      />

      <div v-else class="overflow-x-auto rounded-card border border-line bg-surface">
        <UiTable dense>
          <thead>
            <tr>
              <th>{{ t('admin.time') }}</th>
              <th>{{ t('admin.model') }}</th>
              <th>{{ t('common.status') }}</th>
              <th class="num">{{ t('admin.statPromptTokens') }}</th>
              <th class="num">{{ t('admin.statCompletionTokens') }}</th>
              <th class="num">{{ t('admin.statTotalTokens') }}</th>
              <th class="num">{{ t('admin.duration') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="log in data" :key="`${log.model}-${log.created_at}`">
              <td class="whitespace-nowrap text-muted">{{ formatDateTime(log.created_at) }}</td>
              <td class="font-medium text-ink">{{ log.model }}</td>
              <td><UiBadge :tone="statusTone(log.status_code)">{{ log.status_code }}</UiBadge></td>
              <td class="num">{{ formatNumber(log.prompt_tokens) }}</td>
              <td class="num">{{ formatNumber(log.completion_tokens) }}</td>
              <td class="num">{{ formatNumber(log.total_tokens) }}</td>
              <td class="num">{{ t('admin.durationMs', { value: log.duration_ms }) }}</td>
            </tr>
          </tbody>
        </UiTable>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { api, type Rankings } from '~/src/api'
import { formatCompact, formatDateTime, formatNumber } from '~/src/format'

type Period = 'today' | 'week' | 'month' | 'year'

/** Public leaderboard feed. Kept local so `src/api.ts` stays untouched. */
function getRankings(period: Period) {
  return api<Rankings>(`/rankings?period=${encodeURIComponent(period)}`)
}

const { t } = useI18n()
const { settings } = useSiteSettings()

useHead({
  title: () => `${t('site.rkMetaTitle')} · ${settings.value.name}`,
  meta: [{ name: 'description', content: () => t('site.rkMetaDescription') }],
})

const PERIODS: { value: Period; labelKey: string }[] = [
  { value: 'today', labelKey: 'site.rkPeriodToday' },
  { value: 'week', labelKey: 'site.rkPeriodWeek' },
  { value: 'month', labelKey: 'site.rkPeriodMonth' },
  { value: 'year', labelKey: 'site.rkPeriodYear' },
]

const period = ref<Period>('week')
const tab = ref('models')
const rankings = ref<Rankings | null>(null)
const pending = ref(false)
const failure = ref('')

async function load() {
  pending.value = true
  failure.value = ''
  try {
    rankings.value = await getRankings(period.value)
  } catch (cause) {
    failure.value = cause instanceof Error ? cause.message : t('common.loadFailed')
  } finally {
    pending.value = false
  }
}

const tabs = computed(() => [
  { value: 'models', label: t('site.rkTabModels'), count: rankings.value?.models.length ?? 0 },
  { value: 'vendors', label: t('site.rkTabVendors'), count: rankings.value?.vendors.length ?? 0 },
  { value: 'users', label: t('site.rkTabUsers'), count: rankings.value?.users.length ?? 0 },
])

const stats = computed(() => [
  { key: 'site.rkTotalTokens', value: formatCompact(rankings.value?.total_tokens ?? 0) },
  { key: 'site.rkRankedModels', value: formatNumber(rankings.value?.models.length ?? 0) },
])

const hasTraffic = computed(() => Boolean(rankings.value && rankings.value.models.length > 0))

watch(period, load)
onMounted(load)
</script>

<template>
  <div>
    <section class="shell pt-16 pb-10 md:pt-20">
      <div class="flex flex-wrap items-end justify-between gap-6">
        <div class="max-w-2xl space-y-3">
          <p class="text-2xs font-medium tracking-wide text-clay uppercase">{{ t('site.rkEyebrow') }}</p>
          <h1 class="display text-4xl text-ink md:text-5xl">{{ t('site.rkTitle') }}</h1>
          <p class="text-muted">{{ t('site.rkLead') }}</p>
        </div>

        <div
          class="inline-flex items-center gap-0.5 rounded-control border border-line-strong bg-surface p-0.5"
          role="group"
          :aria-label="t('site.rkPeriodLabel')"
        >
          <button
            v-for="item in PERIODS"
            :key="item.value"
            type="button"
            :class="[
              'rounded-[7px] px-3 py-1.5 text-[13px] transition-colors duration-150',
              period === item.value ? 'bg-sunken text-ink' : 'text-muted hover:text-ink',
            ]"
            :aria-pressed="period === item.value"
            @click="period = item.value"
          >{{ t(item.labelKey) }}</button>
        </div>
      </div>
    </section>

    <section class="shell pb-24">
      <UiAlert v-if="failure" tone="danger" :title="t('site.rkErrorTitle')">
        {{ failure }}
        <UiButton variant="link" size="sm" class="ml-1 h-auto p-0" @click="load">{{ t('common.retry') }}</UiButton>
      </UiAlert>

      <div v-else-if="pending && !rankings" class="space-y-6">
        <div class="grid gap-4 sm:grid-cols-3">
          <div v-for="index in 3" :key="index" class="rounded-card border border-line bg-surface p-5">
            <UiSkeleton :rows="2" />
          </div>
        </div>
        <div class="rounded-card border border-line bg-surface p-5">
          <UiSkeleton :rows="8" />
        </div>
      </div>

      <UiEmptyState
        v-else-if="!hasTraffic"
        class="rounded-card border border-line bg-surface"
        :title="t('site.rkEmptyTitle')"
        :description="t('site.rkEmptyBody')"
      />

      <template v-else-if="rankings">
        <div class="grid gap-4 sm:grid-cols-3">
          <div v-for="item in stats" :key="item.key" class="rounded-card border border-line bg-surface px-5 py-4">
            <p class="text-2xs text-faint">{{ t(item.key) }}</p>
            <p class="numeric mt-1 text-2xl text-ink">{{ item.value }}</p>
          </div>
          <div class="rounded-card border border-line bg-sunken px-5 py-4">
            <p class="text-2xs text-faint">{{ t('common.updatedAt') }}</p>
            <p class="numeric mt-1 text-[13px] text-muted">
              {{ t('site.rkUpdatedAt', { time: formatDateTime(rankings.updated_at) }) }}
            </p>
          </div>
        </div>

        <UiTabs v-model="tab" :items="tabs" class="mt-10">
          <div class="pt-5">
            <RankingsModelTable v-if="tab === 'models'" :rows="rankings.models" />

            <RankingsVendorTable v-else-if="tab === 'vendors'" :rows="rankings.vendors" />

            <template v-else>
              <UiEmptyState
                v-if="!rankings.users.length"
                class="rounded-card border border-line bg-surface"
                :title="t('site.rkUsersEmptyTitle')"
                :description="t('site.rkUsersEmptyBody')"
              />
              <div v-else class="space-y-3">
                <RankingsUserTable :rows="rankings.users" />
                <p class="text-2xs text-faint">{{ t('site.rkPrivacyNote') }}</p>
              </div>
            </template>
          </div>
        </UiTabs>

        <div class="mt-10 grid gap-4 md:grid-cols-2">
          <RankingsMoverList
            :rows="rankings.top_movers"
            direction="up"
            :title="t('site.rkMoversTitle')"
            :description="t('site.rkMoversLead')"
          />
          <RankingsMoverList
            :rows="rankings.top_droppers"
            direction="down"
            :title="t('site.rkDroppersTitle')"
            :description="t('site.rkDroppersLead')"
          />
        </div>
      </template>
    </section>
  </div>
</template>

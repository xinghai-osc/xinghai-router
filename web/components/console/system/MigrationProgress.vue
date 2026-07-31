<script setup lang="ts">
import { DatabaseZap } from 'lucide-vue-next'
import type { MigrationStatus } from '~/src/api'
import { formatDateTime } from '~/src/format'

const props = defineProps<{ status: MigrationStatus | null }>()

const { t } = useI18n()

/** The Go zero time is serialised verbatim; treat it as "never". */
function displayTime(value: string | null | undefined): string {
  if (!value || value.startsWith('0001-')) return '—'
  return formatDateTime(value)
}

/** A migration that has never run reports an empty status rather than `idle`. */
const state = computed(() => props.status?.status || 'idle')
const started = computed(() => state.value !== 'idle')

const STATE_TONES = {
  idle: 'neutral',
  running: 'clay',
  completed: 'success',
  failed: 'danger',
} as const

const STATE_KEYS = {
  idle: 'system.statusIdle',
  running: 'system.statusRunning',
  completed: 'system.statusCompleted',
  failed: 'system.statusFailed',
} as const

const percent = computed(() => {
  const total = props.status?.total ?? 0
  const current = props.status?.current ?? 0
  if (total <= 0) return state.value === 'completed' ? 100 : 0
  return Math.min(100, Math.max(0, Math.round((current / total) * 100)))
})
</script>

<template>
  <UiCard :title="t('system.migrationProgress')">
    <template #actions>
      <UiBadge :tone="STATE_TONES[state]" dot>{{ t(STATE_KEYS[state]) }}</UiBadge>
    </template>

    <UiEmptyState
      v-if="!started"
      :icon="DatabaseZap"
      :title="t('system.migrationIdleTitle')"
      :description="t('system.migrationIdleBody')"
    />

    <div v-else class="space-y-4">
      <div class="space-y-2">
        <div class="flex items-center justify-between gap-3 text-[13px]">
          <span class="min-w-0 truncate text-ink">{{ status?.step || '—' }}</span>
          <span class="numeric shrink-0 text-muted">
            {{ t('system.progressRatio', { current: status?.current ?? 0, total: status?.total ?? 0 }) }}
          </span>
        </div>

        <div
          class="h-2 w-full overflow-hidden rounded-full bg-sunken"
          role="progressbar"
          :aria-label="t('system.migrationProgress')"
          :aria-valuenow="percent"
          aria-valuemin="0"
          aria-valuemax="100"
        >
          <div
            class="h-full rounded-full bg-clay transition-[width] duration-150 ease-out"
            :style="{ width: `${percent}%` }"
          />
        </div>
      </div>

      <dl class="grid gap-3 text-[13px] sm:grid-cols-2">
        <div>
          <dt class="text-muted">{{ t('system.startedAt') }}</dt>
          <dd class="numeric mt-0.5 text-ink">{{ displayTime(status?.started_at) }}</dd>
        </div>
        <div>
          <dt class="text-muted">{{ t('system.finishedAt') }}</dt>
          <dd class="numeric mt-0.5 text-ink">{{ displayTime(status?.finished_at) }}</dd>
        </div>
      </dl>

      <div v-if="status?.detail">
        <p class="text-[13px] text-muted">{{ t('system.detail') }}</p>
        <p class="mt-0.5 text-[13px] break-words text-ink">{{ status.detail }}</p>
      </div>

      <UiAlert v-if="status?.error" tone="danger" :title="t('system.migrationFailed')">
        {{ status.error }}
      </UiAlert>
    </div>
  </UiCard>
</template>

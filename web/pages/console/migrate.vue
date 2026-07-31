<script setup lang="ts">
import { Database } from 'lucide-vue-next'
import { endpoints, type MigrationRequest, type MigrationStatus } from '~/src/api'
import { formatDateTime, formatNumber } from '~/src/format'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const POLL_INTERVAL_MS = 2000

const { t } = useI18n()
const { settings: site } = useSiteSettings()
const { toast } = useToast()
const { busy, run } = useAction()

useHead({ title: () => `${t('system.migrateTitle')} · ${site.value.name}` })

/** Write-only: the DSN carries credentials and is never read back from the API. */
const sourceDsn = ref('')
const sourceDriver = ref('mysql')
const dsnError = ref('')

const driverOptions = computed(() => [
  { value: 'mysql', label: t('system.driverMysql') },
  { value: 'postgres', label: t('system.driverPostgres') },
  { value: 'sqlite', label: t('system.driverSqlite') },
])

const status = ref<MigrationStatus | null>(null)

const { data: history, pending, error, refresh } = useResource(
  () => endpoints.getMigrationRequests(),
  { data: [] as MigrationRequest[] },
)

const STATUS_KEYS = {
  idle: 'system.statusIdle',
  running: 'system.statusRunning',
  completed: 'system.statusCompleted',
  failed: 'system.statusFailed',
} as const

const STATUS_TONES = {
  idle: 'neutral',
  running: 'clay',
  completed: 'success',
  failed: 'danger',
} as const

let timer: ReturnType<typeof setInterval> | null = null

function stopPolling() {
  if (!timer) return
  clearInterval(timer)
  timer = null
}

function startPolling() {
  if (timer) return
  timer = setInterval(poll, POLL_INTERVAL_MS)
}

async function loadStatus(): Promise<MigrationStatus | null> {
  try {
    status.value = await endpoints.getMigrationStatus()
  } catch {
    status.value = null
  }
  return status.value
}

async function poll() {
  const next = await loadStatus()
  if (next?.status === 'running') return
  stopPolling()
  if (next?.status === 'completed') toast.success(t('system.migrationCompleted'))
  else if (next?.status === 'failed') toast.error(t('system.migrationFailed'), next.error)
  await refresh()
}

onMounted(async () => {
  const next = await loadStatus()
  if (next?.status === 'running') startPolling()
})

onBeforeUnmount(stopPolling)

async function start() {
  if (!sourceDsn.value.trim()) {
    dsnError.value = t('system.dsnRequired')
    return
  }
  dsnError.value = ''
  const ok = await run(() => endpoints.runMigration({
    source_dsn: sourceDsn.value.trim(),
    source_driver: sourceDriver.value,
  }))
  if (!ok) {
    toast.error(t('common.actionFailed'))
    return
  }
  sourceDsn.value = ''
  toast.success(t('system.migrationStarted'))
  await loadStatus()
  startPolling()
  await refresh()
}

/** The Go zero time is serialised verbatim; treat it as "never". */
function displayTime(value: string | null | undefined): string {
  if (!value || value.startsWith('0001-')) return '—'
  return formatDateTime(value)
}
</script>

<template>
  <ConsoleSystemGate permission="system.manage">
    <div class="space-y-4">
      <div class="min-w-0 space-y-1">
        <h2 class="text-lg font-semibold text-ink">{{ t('system.migrateTitle') }}</h2>
        <p class="text-[13px] text-muted">{{ t('system.migrateLead') }}</p>
      </div>

      <UiCard :title="t('system.migration')">
        <form class="space-y-4" @submit.prevent="start">
          <div class="grid gap-4 sm:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]">
            <UiField
              :label="t('system.sourceDsn')"
              :hint="t('system.sourceDsnHint')"
              :error="dsnError"
              required
              for="source-dsn"
            >
              <UiInput
                id="source-dsn"
                v-model="sourceDsn"
                type="password"
                autocomplete="off"
                mono
              />
            </UiField>

            <UiField :label="t('system.sourceDriver')" for="source-driver">
              <UiSelect
                id="source-driver"
                v-model="sourceDriver"
                :options="driverOptions"
                :placeholder="t('common.selectPlaceholder')"
              />
            </UiField>
          </div>

          <div class="flex justify-end">
            <UiButton
              type="submit"
              :loading="busy"
              :disabled="status?.status === 'running'"
            >
              {{ t('system.startMigration') }}
            </UiButton>
          </div>
        </form>
      </UiCard>

      <ConsoleSystemMigrationProgress :status="status" />

      <div class="flex flex-wrap items-end justify-between gap-3 pt-2">
        <h3 class="text-[15px] font-semibold text-ink">{{ t('system.migrationHistory') }}</h3>
        <UiButton variant="secondary" size="sm" :loading="pending" @click="refresh">
          {{ t('common.refresh') }}
        </UiButton>
      </div>

      <ConsoleSystemDataState
        :pending="pending"
        :error="error"
        :empty="history.data.length === 0"
        :empty-icon="Database"
        :empty-title="t('system.historyEmptyTitle')"
        :empty-description="t('system.historyEmptyBody')"
        :rows="3"
      >
        <UiTable>
          <thead>
            <tr>
              <th>{{ t('common.status') }}</th>
              <th>{{ t('system.sourceDriver') }}</th>
              <th>{{ t('system.step') }}</th>
              <th class="num">{{ t('system.migrationProgress') }}</th>
              <th class="num">{{ t('system.startedAt') }}</th>
              <th class="num">{{ t('system.finishedAt') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in history.data" :key="item.id">
              <td>
                <UiBadge :tone="STATUS_TONES[item.status]" dot>{{ t(STATUS_KEYS[item.status]) }}</UiBadge>
                <p v-if="item.error" class="mt-1 text-[13px] break-words text-danger">{{ item.error }}</p>
              </td>
              <td class="font-mono text-[13px]">{{ item.source_driver }}</td>
              <td>
                <p class="text-ink">{{ item.step || '—' }}</p>
                <p v-if="item.detail" class="mt-0.5 text-[13px] text-muted">{{ item.detail }}</p>
              </td>
              <td class="num">
                {{ t('system.progressRatio', {
                  current: formatNumber(item.current),
                  total: formatNumber(item.total),
                }) }}
              </td>
              <td class="num">{{ displayTime(item.started_at) }}</td>
              <td class="num">{{ displayTime(item.finished_at) }}</td>
            </tr>
          </tbody>
        </UiTable>
      </ConsoleSystemDataState>
    </div>
  </ConsoleSystemGate>
</template>

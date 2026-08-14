<script setup lang="ts">
import { Bell, Plus } from 'lucide-vue-next'
import { endpoints, type Notification, type NotificationForm } from '~/src/api'
import { formatDateTime } from '~/src/format'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { settings: site } = useSiteSettings()
const { toast } = useToast()
const { busy, run } = useAction()

useHead({ title: () => `${t('system.notificationsTitle')} · ${site.value.name}` })

const { data, pending, error, refresh } = useResource(
  () => endpoints.getAdminNotifications(),
  { data: [] as Notification[] },
)

const notifications = computed(() => data.value.data ?? [])

const dialogOpen = ref(false)
const editing = ref<Notification | null>(null)
const removing = ref<Notification | null>(null)

function openCreate() {
  editing.value = null
  dialogOpen.value = true
}

function openEdit(notification: Notification) {
  editing.value = notification
  dialogOpen.value = true
}

async function submit(form: NotificationForm) {
  const target = editing.value
  const ok = await run(() => (target
    ? endpoints.updateNotification(target.id, form)
    : endpoints.createNotification(form)))
  if (!ok) {
    toast.error(t('common.actionFailed'))
    return
  }
  toast.success(target ? t('system.notificationUpdated') : t('system.notificationCreated'))
  dialogOpen.value = false
  await refresh()
}

async function confirmDelete() {
  const target = removing.value
  if (!target) return
  const ok = await run(() => endpoints.deleteNotification(target.id))
  if (!ok) {
    toast.error(t('common.actionFailed'))
    return
  }
  toast.success(t('system.notificationDeleted'))
  removing.value = null
  await refresh()
}
</script>

<template>
  <ConsoleSystemGate permission="system.manage">
    <div class="space-y-4">
      <div class="flex flex-wrap items-end justify-between gap-3">
        <div class="min-w-0 space-y-1">
          <h2 class="text-lg font-semibold text-ink">{{ t('system.notificationsTitle') }}</h2>
          <p class="text-[13px] text-muted">{{ t('system.notificationsLead') }}</p>
        </div>
        <UiButton size="sm" @click="openCreate">
          <Plus class="size-4" />
          {{ t('system.newNotification') }}
        </UiButton>
      </div>

      <ConsoleSystemDataState
        :pending="pending"
        :error="error"
        :empty="notifications.length === 0"
        :empty-icon="Bell"
        :empty-title="t('system.notificationsEmptyTitle')"
        :empty-description="t('system.notificationsEmptyBody')"
        :rows="3"
      >
        <UiTable>
          <thead>
            <tr>
              <th>{{ t('system.notificationTitle') }}</th>
              <th>{{ t('system.notificationContent') }}</th>
              <th>{{ t('common.status') }}</th>
              <th class="num">{{ t('system.notificationSortOrder') }}</th>
              <th class="num">{{ t('common.createdAt') }}</th>
              <th>{{ t('common.actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="n in notifications" :key="n.id">
              <td class="font-medium text-ink">{{ n.title }}</td>
              <td class="max-w-[24rem] truncate text-muted">{{ n.content }}</td>
              <td>
                <UiBadge :tone="n.enabled ? 'success' : 'neutral'" dot>
                  {{ n.enabled ? t('common.enabled') : t('common.disabled') }}
                </UiBadge>
              </td>
              <td class="num">{{ n.sort_order }}</td>
              <td class="num">{{ formatDateTime(n.created_at) }}</td>
              <td>
                <div class="flex items-center gap-1">
                  <UiButton variant="ghost" size="sm" @click="openEdit(n)">
                    {{ t('common.edit') }}
                  </UiButton>
                  <UiButton variant="ghost" size="sm" @click="removing = n">
                    {{ t('common.delete') }}
                  </UiButton>
                </div>
              </td>
            </tr>
          </tbody>
        </UiTable>
      </ConsoleSystemDataState>
    </div>

    <ConsoleSystemNotificationDialog
      v-model:open="dialogOpen"
      :notification="editing"
      :busy="busy"
      @submit="submit"
    />

    <ConsoleSystemConfirmDialog
      :open="removing !== null"
      :body="t('system.deleteNotificationBody', { title: removing?.title ?? '' })"
      :busy="busy"
      @update:open="value => { if (!value) removing = null }"
      @confirm="confirmDelete"
    />
  </ConsoleSystemGate>
</template>

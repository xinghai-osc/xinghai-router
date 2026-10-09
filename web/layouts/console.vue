<script setup lang="ts">
import { useMediaQuery } from '@vueuse/core'
import { X } from 'lucide-vue-next'
import { DialogClose, DialogContent, DialogOverlay, DialogPortal, DialogRoot, DialogTitle } from 'reka-ui'

const { settings, loadSiteSettings } = useSiteSettings()
const { account, loading, loaded, error, can, loadAccount } = useAccount()
const { ready: workspaceReady, current: workspace, error: workspaceError, pageKey: workspacePageKey, loadWorkspaces } = useWorkspace()
const { open: notificationsOpen, loadNotifications } = useNotifications()
const { t } = useI18n()
const route = useRoute()
const navOpen = ref(false)
const navCollapsed = ref(false)
const mounted = ref(false)
const desktop = useMediaQuery('(min-width: 1024px)')

onMounted(() => {
  mounted.value = true
  try { navCollapsed.value = localStorage.getItem('xinghai.sidebar-collapsed') === 'true' } catch { return }
})

function toggleSidebar() {
  navCollapsed.value = !navCollapsed.value
  try { localStorage.setItem('xinghai.sidebar-collapsed', String(navCollapsed.value)) } catch { return }
}

watch(desktop, value => { if (value) navOpen.value = false })

useHead({
  meta: [{ name: 'robots', content: 'noindex, nofollow' }],
})

onMounted(async () => {
  loadSiteSettings()
  await loadAccount()
  // Show the notification popup on every fresh login (per page load).
  if (account.value) loadNotifications()
})

watch(() => route.fullPath, () => { navOpen.value = false })
watch(workspacePageKey, () => { navOpen.value = false })
watch([() => account.value?.id, () => account.value?.must_change_password], () => {
  if (import.meta.client && account.value && !account.value.must_change_password) void loadWorkspaces()
}, { immediate: true })

const booting = computed(() => !mounted.value || loading.value || (!account.value && !error.value) || (Boolean(account.value) && !mustChangePassword.value && !workspaceReady.value && !workspaceError.value))

// The backend answers 403 password_change_required on every route except
// /account/me, /account/password and /auth/logout, so nothing else is usable
// until the password is rotated.
const mustChangePassword = computed(() => mounted.value && Boolean(account.value?.must_change_password))
const canNavigate = (permission: string) => mounted.value && can(permission)

watchEffect(() => {
  if (import.meta.client && loaded.value && !account.value && !loading.value && !error.value) {
    navigateTo({ path: '/auth', query: { redirect: route.fullPath } })
  }
  if (mustChangePassword.value && route.path !== '/console/account') navigateTo('/console/account')
})
</script>

<template>
  <div :class="['console-shell min-h-dvh bg-paper lg:grid', navCollapsed ? 'lg:grid-cols-[4.5rem_minmax(0,1fr)]' : 'lg:grid-cols-[16rem_minmax(0,1fr)]']">
    <a href="#console-content" class="sr-only z-50 rounded-control bg-clay px-4 py-2 text-sm text-clay-ink focus:not-sr-only focus:fixed focus:top-2 focus:left-2">{{ t('common.skipToContent') }}</a>
    <aside class="sticky top-0 hidden h-dvh bg-paper lg:block">
      <ConsoleSidebar :can="canNavigate" :site-name="settings.name" :icon-url="settings.icon_url" :collapsed="navCollapsed" collapsible @toggle="toggleSidebar" />
    </aside>

    <DialogRoot v-model:open="navOpen">
      <DialogPortal>
        <DialogOverlay class="animate-fade fixed inset-0 z-50 bg-[var(--overlay)]" />
        <DialogContent class="fixed inset-y-0 left-0 z-50 w-72 max-w-[85vw] border-r border-line bg-surface shadow-pop focus:outline-none" :aria-describedby="undefined">
          <DialogTitle class="sr-only">{{ t('common.menu') }}</DialogTitle>
          <DialogClose class="absolute top-4 right-3 flex size-8 items-center justify-center rounded-control bg-surface text-muted transition-colors duration-150 ease-out hover:bg-sunken hover:text-ink" :aria-label="t('common.close')">
            <X class="size-4" aria-hidden="true" />
          </DialogClose>
          <ConsoleSidebar :can="canNavigate" :site-name="settings.name" :icon-url="settings.icon_url" class="[&>div:first-child]:pr-14" />
        </DialogContent>
      </DialogPortal>
    </DialogRoot>

    <div class="console-workspace flex min-w-0 flex-col bg-sunken/40 lg:my-3 lg:mr-3 lg:rounded-[1.75rem] lg:border lg:border-line">
      <ConsoleHeader @open-nav="navOpen = true">
        <template #account>
          <ConsoleAccountMenu v-if="mounted && account" />
          <UiSkeleton v-else class="size-8 rounded-full" />
        </template>
      </ConsoleHeader>

      <main id="console-content" tabindex="-1" class="mx-auto min-w-0 w-full max-w-[var(--layout-console-width)] flex-1 space-y-6 px-4 py-6 outline-none sm:px-6 lg:px-8 lg:py-8 xl:px-10">
        <UiAlert v-if="mustChangePassword" tone="warn" :title="t('console.mustChangePasswordTitle')">
          {{ t('console.mustChangePasswordBody') }}
        </UiAlert>

        <div v-if="booting" class="space-y-4" aria-busy="true" :aria-label="t('common.loading')">
          <UiSkeleton class="h-7 w-48" />
          <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
            <UiCard v-for="tile in 4" :key="tile">
              <UiSkeleton class="h-8 w-24" />
            </UiCard>
          </div>
          <UiCard>
            <UiSkeleton :rows="6" />
          </UiCard>
        </div>
        <UiAlert v-else-if="error" tone="danger">
          {{ error }}
          <UiButton variant="link" size="sm" @click="loadAccount(true)">{{ t('common.retry') }}</UiButton>
        </UiAlert>
        <UiAlert v-else-if="workspaceError && !mustChangePassword" tone="danger" :title="t('console.workspaceLoadFailed')">
          {{ workspaceError }}
          <UiButton variant="link" size="sm" @click="loadWorkspaces(true)">{{ t('common.retry') }}</UiButton>
        </UiAlert>
        <template v-else-if="account && (workspaceReady || mustChangePassword)">
          <UiAlert v-if="workspace && !workspace.is_personal" :title="t('console.workspaceActive', { name: workspace.name })">
            {{ t('console.workspaceScopeNotice') }}
          </UiAlert>
          <div :key="workspacePageKey"><slot /></div>
        </template>
      </main>
    </div>

    <UiToaster />
    <LazyConsoleNotificationsDialog v-if="notificationsOpen" />
  </div>
</template>

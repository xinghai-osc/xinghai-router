<script setup lang="ts">
const { settings, loadSiteSettings } = useSiteSettings()
const { account, loading, error, can, loadAccount } = useAccount()
const { t } = useI18n()
const route = useRoute()
const navOpen = ref(false)

onMounted(() => {
  loadSiteSettings()
  loadAccount()
})

watch(() => route.fullPath, () => { navOpen.value = false })

// A failed load means the session expired; useAccount already dropped the token.
watch(error, (message) => { if (message) navigateTo('/auth') })

const booting = computed(() => loading.value || (!account.value && !error.value))

// The backend answers 403 password_change_required on every route except
// /account/me, /account/password and /auth/logout, so nothing else is usable
// until the password is rotated.
const mustChangePassword = computed(() => Boolean(account.value?.must_change_password))

watchEffect(() => {
  if (mustChangePassword.value && route.path !== '/console/account') navigateTo('/console/account')
})
</script>

<template>
  <div class="min-h-dvh bg-paper lg:grid lg:grid-cols-[15rem_1fr]">
    <aside class="sticky top-0 hidden h-dvh border-r border-line lg:block">
      <ConsoleSidebar :can="can" :site-name="settings.name" :icon-url="settings.icon_url" />
    </aside>

    <div v-if="navOpen" class="fixed inset-0 z-50 lg:hidden">
      <div class="animate-fade absolute inset-0 bg-[var(--overlay)]" @click="navOpen = false" />
      <aside class="animate-fade absolute inset-y-0 left-0 w-64 border-r border-line bg-paper">
        <ConsoleSidebar :can="can" :site-name="settings.name" :icon-url="settings.icon_url" />
      </aside>
    </div>

    <div class="flex min-w-0 flex-col">
      <ConsoleHeader @open-nav="navOpen = true">
        <template #account>
          <ConsoleAccountMenu v-if="account" />
          <UiSkeleton v-else class="size-8 rounded-full" />
        </template>
      </ConsoleHeader>

      <main class="flex-1 space-y-4 px-4 py-6 md:px-8 md:py-8">
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
        <slot v-else />
      </main>
    </div>

    <UiToaster />
  </div>
</template>

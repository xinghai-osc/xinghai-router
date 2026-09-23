<script setup lang="ts">
import { Menu, X } from 'lucide-vue-next'

const { settings } = useSiteSettings()
const { account, loadAccount } = useAccount()
const { t } = useI18n()
const route = useRoute()
const open = ref(false)
const menuTrigger = ref<HTMLButtonElement | null>(null)
const menuId = useId()

const accountName = computed(() => account.value?.name?.trim() || account.value?.email?.trim() || '')
const accountInitial = computed(() => accountName.value.slice(0, 1).toUpperCase() || '?')

const links = [
  { to: '/models', key: 'nav.models' },
  { to: '/activity', key: 'nav.activity' },
  { to: '/rankings', key: 'nav.rankings' },
  { to: '/pricing', key: 'nav.pricingPublic' },
]

function closeMenu() {
  if (!open.value) return
  open.value = false
  menuTrigger.value?.focus()
}

onMounted(() => { void loadAccount() })
watch(() => route.fullPath, () => { open.value = false })
</script>

<template>
  <div class="sticky top-0 z-40" @keydown.esc="closeMenu">
    <div v-if="settings.announcement" class="border-b border-clay/15 bg-clay-soft px-4 py-2 text-center text-xs text-clay">
      {{ settings.announcement }}
    </div>
    <header class="border-b border-line bg-paper/95 backdrop-blur-xl">
      <div class="shell">
        <div class="flex h-[4.5rem] items-center justify-between gap-3 lg:gap-6">
          <SiteLogo :name="settings.name" :icon-url="settings.icon_url" class="min-w-0" />

          <nav class="hidden items-center gap-1 rounded-full border border-line bg-sunken/60 p-1 lg:flex" :aria-label="t('common.menu')">
            <NuxtLink
              v-for="link in links"
              :key="link.to"
              :to="link.to"
              class="rounded-full border border-transparent px-4 py-1.5 text-[13px] font-medium text-muted transition-colors duration-150 ease-out hover:bg-surface hover:text-ink"
              active-class="!border-clay/20 bg-clay-soft !text-clay"
            >{{ t(link.key) }}</NuxtLink>
          </nav>

          <div class="flex shrink-0 items-center gap-1.5">
            <div class="hidden items-center gap-1 border-r border-line pr-3 lg:flex">
              <SiteLocaleToggle />
              <SiteThemeToggle />
            </div>
            <template v-if="account">
              <NuxtLink
                to="/console"
                class="hidden max-w-44 items-center gap-2 rounded-full border border-line bg-surface py-1.5 pr-3 pl-1.5 text-sm text-ink transition-colors duration-150 ease-out hover:border-line-strong hover:bg-sunken sm:flex"
                :aria-label="t('common.openConsole')"
              >
                <img v-if="account.avatar_url" :src="account.avatar_url" alt="" class="size-7 shrink-0 rounded-full object-cover">
                <span v-else class="flex size-7 shrink-0 items-center justify-center rounded-full bg-clay-soft text-2xs font-semibold text-clay">{{ accountInitial }}</span>
                <span class="truncate">{{ accountName }}</span>
              </NuxtLink>
            </template>
            <template v-else>
              <div class="hidden sm:block">
                <UiButton to="/auth" variant="ghost" size="sm">{{ t('common.signIn') }}</UiButton>
              </div>
              <UiButton to="/auth?mode=register" size="sm">{{ t('common.getStarted') }}</UiButton>
            </template>
            <button
              ref="menuTrigger"
              type="button"
              class="inline-flex size-9 items-center justify-center rounded-control border border-line text-muted transition-colors duration-150 ease-out hover:bg-sunken hover:text-ink lg:hidden"
              :aria-expanded="open"
              :aria-controls="menuId"
              :aria-label="open ? t('common.close') : t('common.menu')"
              @click="open = !open"
            >
              <X v-if="open" class="size-[18px]" aria-hidden="true" />
              <Menu v-else class="size-[18px]" aria-hidden="true" />
            </button>
          </div>
        </div>

        <div v-if="open" :id="menuId" class="animate-fade max-h-[calc(100dvh-8rem)] overflow-y-auto border-t border-line pb-3 lg:hidden">
          <nav class="flex flex-col gap-1 py-3" :aria-label="t('common.menu')">
            <NuxtLink
              v-for="link in links"
              :key="link.to"
              :to="link.to"
              class="rounded-control px-3 py-2.5 text-sm font-medium text-muted transition-colors duration-150 ease-out hover:bg-sunken hover:text-ink"
              active-class="bg-clay-soft !text-clay"
            >{{ t(link.key) }}</NuxtLink>
            <NuxtLink
              v-if="account"
              to="/console"
              class="flex items-center gap-2 rounded-control px-3 py-2.5 text-sm text-muted hover:bg-sunken hover:text-ink sm:hidden"
              :aria-label="t('common.openConsole')"
            >
              <img v-if="account.avatar_url" :src="account.avatar_url" alt="" class="size-7 shrink-0 rounded-full object-cover">
              <span v-else class="flex size-7 shrink-0 items-center justify-center rounded-full bg-clay-soft text-2xs font-semibold text-clay">{{ accountInitial }}</span>
              <span class="truncate">{{ accountName }}</span>
            </NuxtLink>
            <NuxtLink v-else to="/auth" class="rounded-control px-3 py-2.5 text-sm text-muted hover:bg-sunken hover:text-ink sm:hidden">
              {{ t('common.signIn') }}
            </NuxtLink>
          </nav>
          <div class="flex items-center gap-1 border-t border-line pt-3">
            <SiteLocaleToggle />
            <SiteThemeToggle />
          </div>
        </div>
      </div>
    </header>
  </div>
</template>

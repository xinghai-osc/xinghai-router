<script setup lang="ts">
import { onClickOutside, useMediaQuery } from '@vueuse/core'
import { Menu, X } from 'lucide-vue-next'

const { settings } = useSiteSettings()
const { account, loadAccount } = useAccount()
const { t } = useI18n()
const route = useRoute()
const open = ref(false)
const topbar = ref<HTMLElement | null>(null)
const menuTrigger = ref<HTMLButtonElement | null>(null)
const menuId = useId()
const desktop = useMediaQuery('(min-width: 1280px)')

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

onClickOutside(topbar, () => { open.value = false }, { ignore: ['[role="menu"]'] })
onMounted(() => { void loadAccount() })
watch(() => route.fullPath, () => { open.value = false })
watch(desktop, (value) => { if (value) open.value = false })
</script>

<template>
  <header ref="topbar" class="site-topbar sticky top-2 z-40 mx-2 mt-2 shrink-0 bg-transparent sm:top-3 sm:mx-3 sm:mt-3" @keydown.esc="closeMenu">
    <div v-if="settings.announcement" class="glass-control mx-3 mt-3 rounded-2xl border border-clay/15 px-5 py-2 text-center text-xs leading-5 break-words text-clay">
      {{ settings.announcement }}
    </div>
    <div class="shell">
      <div class="flex min-h-18 items-center justify-between gap-3 py-3 xl:gap-8">
        <div class="header-actions flex min-w-0">
          <SiteLogo :name="settings.name" :icon-url="settings.icon_url" class="glass-control min-h-11 min-w-0 rounded-full border border-line/50 px-3 py-1.5 transition-colors duration-150 ease-out hover:border-line-strong" />
        </div>

        <nav class="glass-control hidden items-center gap-1 rounded-full border border-line/60 p-1 xl:flex" :aria-label="t('common.menu')">
          <NuxtLink
            v-for="link in links"
            :key="link.to"
            :to="link.to"
            class="inline-flex min-h-11 items-center rounded-full px-3 text-sm font-medium whitespace-nowrap text-muted transition-colors duration-150 ease-out hover:bg-sunken hover:text-ink"
            active-class="bg-clay-soft !text-clay"
          >{{ t(link.key) }}</NuxtLink>
        </nav>

        <div class="header-actions flex shrink-0 items-center gap-2">
          <div class="hidden items-center gap-1 border-r border-line pr-3 xl:flex">
            <SiteLocaleToggle />
            <SiteThemeToggle />
          </div>
          <template v-if="account">
            <NuxtLink
              to="/console"
              class="glass-control hidden min-h-11 max-w-44 items-center gap-2 rounded-full border border-line/60 py-1.5 pr-3 pl-2 text-sm text-ink transition-colors duration-150 ease-out hover:bg-sunken sm:flex"
              :aria-label="t('common.openConsole')"
            >
              <img v-if="account.avatar_url" :src="account.avatar_url" alt="" class="size-7 shrink-0 rounded-full object-cover">
              <span v-else class="flex size-7 shrink-0 items-center justify-center rounded-full bg-clay-soft text-2xs font-semibold text-clay">{{ accountInitial }}</span>
              <span class="truncate">{{ accountName }}</span>
            </NuxtLink>
          </template>
          <template v-else>
            <UiButton to="/auth" variant="ghost" size="sm" class="hidden min-h-11 sm:inline-flex">{{ t('common.signIn') }}</UiButton>
            <UiButton to="/auth?mode=register" size="sm" class="min-h-11">{{ t('common.getStarted') }}</UiButton>
          </template>
          <button
            ref="menuTrigger"
            type="button"
            class="glass-control inline-flex size-11 items-center justify-center rounded-full border border-line/60 text-muted transition-colors duration-150 ease-out hover:bg-sunken hover:text-ink xl:hidden"
            :aria-expanded="open"
            :aria-controls="menuId"
            :aria-label="open ? t('common.close') : t('common.menu')"
            @click="open = !open"
          >
            <X v-if="open" class="size-5" aria-hidden="true" />
            <Menu v-else class="size-5" aria-hidden="true" />
          </button>
        </div>
      </div>

      <div v-show="open" :id="menuId" class="glass-control mt-2 max-h-[60dvh] overflow-y-auto overscroll-contain rounded-3xl border border-line px-3 pb-4 xl:hidden">
        <nav class="grid gap-1 py-3 sm:grid-cols-2" :aria-label="t('common.menu')">
          <NuxtLink
            v-for="link in links"
            :key="link.to"
            :to="link.to"
            class="flex min-h-11 items-center rounded-full px-3 py-2.5 text-sm font-medium text-muted transition-colors duration-150 ease-out hover:bg-sunken hover:text-ink"
            active-class="bg-clay-soft !text-clay"
            @click="open = false"
          >{{ t(link.key) }}</NuxtLink>
          <NuxtLink
            v-if="account"
            to="/console"
            class="flex min-h-11 items-center gap-2 rounded-full px-3 py-2.5 text-sm text-muted transition-colors duration-150 ease-out hover:bg-sunken hover:text-ink sm:hidden"
            :aria-label="t('common.openConsole')"
            @click="open = false"
          >
            <img v-if="account.avatar_url" :src="account.avatar_url" alt="" class="size-7 shrink-0 rounded-full object-cover">
            <span v-else class="flex size-7 shrink-0 items-center justify-center rounded-full bg-clay-soft text-2xs font-semibold text-clay">{{ accountInitial }}</span>
            <span class="truncate">{{ accountName }}</span>
          </NuxtLink>
          <NuxtLink v-else to="/auth" class="flex min-h-11 items-center rounded-full px-3 py-2.5 text-sm text-muted transition-colors duration-150 ease-out hover:bg-sunken hover:text-ink sm:hidden" @click="open = false">
            {{ t('common.signIn') }}
          </NuxtLink>
        </nav>
        <div class="flex items-center gap-2 border-t border-line pt-3">
          <SiteLocaleToggle />
          <SiteThemeToggle />
        </div>
      </div>
    </div>
  </header>
</template>

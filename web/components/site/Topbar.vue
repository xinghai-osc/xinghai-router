<script setup lang="ts">
import { Menu, X } from 'lucide-vue-next'

const { settings } = useSiteSettings()
const { account, loadAccount } = useAccount()
const { t } = useI18n()
const route = useRoute()
const open = ref(false)
const scrolled = ref(false)

const accountName = computed(() => account.value?.name?.trim() || account.value?.email?.trim() || '')
const accountInitial = computed(() => accountName.value.slice(0, 1).toUpperCase() || '?')

const links = [
  { to: '/models', key: 'nav.models' },
  { to: '/activity', key: 'nav.activity' },
  { to: '/rankings', key: 'nav.rankings' },
  { to: '/pricing', key: 'nav.pricingPublic' },
]

function onScroll() {
  scrolled.value = window.scrollY > 8
}

onMounted(() => {
  void loadAccount()
  window.addEventListener('scroll', onScroll, { passive: true })
  onScroll()
})
onBeforeUnmount(() => window.removeEventListener('scroll', onScroll))

watch(() => route.fullPath, () => { open.value = false })
</script>

<template>
  <div class="sticky top-0 z-40">
    <div v-if="settings.announcement" class="border-b border-line bg-clay-soft px-4 py-2 text-center text-[13px] text-clay">
      {{ settings.announcement }}
    </div>
    <header class="bg-transparent transition-[padding] duration-150">
      <div class="shell py-2">
        <div
          class="flex h-14 items-center justify-between gap-3 rounded-full border px-3 transition-[background-color,border-color,box-shadow,backdrop-filter] duration-300 md:gap-6 md:px-4"
          :class="scrolled ? 'border-line-strong bg-surface/75 shadow-pop backdrop-blur-xl' : 'border-transparent bg-paper/20'"
        >
        <SiteLogo :name="settings.name" :icon-url="settings.icon_url" />

        <nav class="hidden items-center gap-1 md:flex" :aria-label="t('common.menu')">
          <NuxtLink
            v-for="link in links"
            :key="link.to"
            :to="link.to"
            class="rounded-control px-3 py-1.5 text-sm text-muted transition-colors duration-150 hover:bg-sunken hover:text-ink"
            active-class="bg-sunken text-ink"
          >{{ t(link.key) }}</NuxtLink>
        </nav>

        <div class="flex shrink-0 items-center gap-1.5">
          <div class="hidden md:block">
            <SiteLocaleToggle />
          </div>
          <div class="hidden md:block">
            <SiteThemeToggle />
          </div>
          <template v-if="account">
            <NuxtLink
              to="/console"
              class="hidden max-w-44 items-center gap-2 rounded-control px-2 py-1.5 text-sm text-ink transition-colors duration-150 hover:bg-sunken sm:flex"
              :aria-label="t('common.openConsole')"
            >
              <img
                v-if="account.avatar_url"
                :src="account.avatar_url"
                alt=""
                class="size-7 shrink-0 rounded-full object-cover"
              >
              <span
                v-else
                class="flex size-7 shrink-0 items-center justify-center rounded-full bg-clay-soft text-2xs font-semibold text-clay"
              >{{ accountInitial }}</span>
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
            type="button"
            class="inline-flex size-9 items-center justify-center rounded-control text-muted transition-colors hover:bg-sunken hover:text-ink md:hidden"
            :aria-expanded="open"
            :aria-label="t('common.menu')"
            @click="open = !open"
          >
            <X v-if="open" class="size-[18px]" />
            <Menu v-else class="size-[18px]" />
          </button>
        </div>
      </div>

      <div v-if="open" class="animate-fade border-t border-line bg-paper md:hidden">
        <nav class="shell flex flex-col py-2" :aria-label="t('common.menu')">
          <NuxtLink
            v-for="link in links"
            :key="link.to"
            :to="link.to"
            class="rounded-control px-3 py-2.5 text-sm text-muted transition-colors hover:bg-sunken hover:text-ink"
            active-class="bg-sunken text-ink"
          >{{ t(link.key) }}</NuxtLink>
          <NuxtLink
            v-if="account"
            to="/console"
            class="flex items-center gap-2 rounded-control px-3 py-2.5 text-sm text-muted hover:bg-sunken hover:text-ink sm:hidden"
            :aria-label="t('common.openConsole')"
          >
            <img
              v-if="account.avatar_url"
              :src="account.avatar_url"
              alt=""
              class="size-7 shrink-0 rounded-full object-cover"
            >
            <span
              v-else
              class="flex size-7 shrink-0 items-center justify-center rounded-full bg-clay-soft text-2xs font-semibold text-clay"
            >{{ accountInitial }}</span>
            <span class="truncate">{{ accountName }}</span>
          </NuxtLink>
          <NuxtLink v-else to="/auth" class="rounded-control px-3 py-2.5 text-sm text-muted hover:bg-sunken hover:text-ink sm:hidden">
            {{ t('common.signIn') }}
          </NuxtLink>
        </nav>
        <div class="shell flex items-center gap-1 border-t border-line py-2 md:hidden">
          <SiteLocaleToggle />
          <SiteThemeToggle />
        </div>
      </div>
      </div>
    </header>
  </div>
</template>

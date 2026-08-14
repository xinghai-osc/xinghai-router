<script setup lang="ts">
import { Menu, X } from 'lucide-vue-next'

const { settings } = useSiteSettings()
const { t } = useI18n()
const route = useRoute()
const open = ref(false)
const scrolled = ref(false)

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
  window.addEventListener('scroll', onScroll, { passive: true })
  onScroll()
})
onBeforeUnmount(() => window.removeEventListener('scroll', onScroll))

watch(() => route.fullPath, () => { open.value = false })
</script>

<template>
  <div class="sticky top-0 z-40">
    <div v-if="settings.announcement" class="border-b border-line bg-clay-soft px-4 py-2 text-center text-[13px] text-clay-ink">
      {{ settings.announcement }}
    </div>
    <header
      class="border-b border-line bg-paper/85 backdrop-blur-md transition-[box-shadow,border-color] duration-150"
      :class="scrolled ? 'shadow-[0_1px_3px_rgb(0_0_0/0.04)]' : 'border-transparent'"
    >
      <div class="shell flex h-16 items-center justify-between gap-3 md:gap-6">
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
          <div class="hidden sm:block">
            <UiButton to="/auth" variant="ghost" size="sm">{{ t('common.signIn') }}</UiButton>
          </div>
          <UiButton to="/auth?mode=register" size="sm">{{ t('common.getStarted') }}</UiButton>
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
          <NuxtLink to="/auth" class="rounded-control px-3 py-2.5 text-sm text-muted hover:bg-sunken hover:text-ink sm:hidden">
            {{ t('common.signIn') }}
          </NuxtLink>
        </nav>
        <div class="shell flex items-center gap-1 border-t border-line py-2 md:hidden">
          <SiteLocaleToggle />
          <SiteThemeToggle />
        </div>
      </div>
    </header>
  </div>
</template>

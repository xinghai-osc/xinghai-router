<script setup lang="ts">
import { Menu, X } from 'lucide-vue-next'

const { settings } = useSiteSettings()
const { t } = useI18n()
const route = useRoute()
const open = ref(false)

const links = [
  { to: '/models', key: 'nav.models' },
  { to: '/activity', key: 'nav.activity' },
  { to: '/rankings', key: 'nav.rankings' },
  { to: '/pricing', key: 'nav.pricingPublic' },
]

watch(() => route.fullPath, () => { open.value = false })
</script>

<template>
  <header class="sticky top-0 z-40 border-b border-line bg-paper/85 backdrop-blur-md">
    <div class="shell flex h-16 items-center justify-between gap-6">
      <SiteLogo :name="settings.name" :icon-url="settings.icon_url" />

      <nav class="hidden items-center gap-1 md:flex" :aria-label="t('common.menu')">
        <NuxtLink
          v-for="link in links"
          :key="link.to"
          :to="link.to"
          class="rounded-control px-3 py-1.5 text-sm text-muted transition-colors duration-150 hover:bg-sunken hover:text-ink"
          active-class="text-ink"
        >{{ t(link.key) }}</NuxtLink>
      </nav>

      <div class="flex items-center gap-1.5">
        <SiteLocaleToggle />
        <SiteThemeToggle />
        <UiButton to="/auth" variant="ghost" size="sm" class="hidden sm:inline-flex">{{ t('common.signIn') }}</UiButton>
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
          class="rounded-control px-2 py-2.5 text-sm text-muted transition-colors hover:bg-sunken hover:text-ink"
          active-class="text-ink"
        >{{ t(link.key) }}</NuxtLink>
        <NuxtLink to="/auth" class="rounded-control px-2 py-2.5 text-sm text-muted hover:bg-sunken hover:text-ink">
          {{ t('common.signIn') }}
        </NuxtLink>
      </nav>
    </div>
  </header>
</template>

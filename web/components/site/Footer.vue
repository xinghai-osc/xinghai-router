<script setup lang="ts">
const { settings } = useSiteSettings()
const { t } = useI18n()
const year = new Date().getFullYear()

const groups = [
  {
    titleKey: 'nav.product',
    links: [
      { to: '/models', key: 'nav.models' },
      { to: '/activity', key: 'nav.activity' },
      { to: '/pricing', key: 'nav.pricingPublic' },
      { to: '/rankings', key: 'nav.rankings' },
    ],
  },
  {
    titleKey: 'nav.developers',
    links: [
      { to: '/console/keys', key: 'nav.keys' },
      { to: '/console/usage', key: 'nav.usage' },
    ],
  },
  {
    titleKey: 'nav.legal',
    links: [
      { to: '/terms', key: 'nav.terms' },
      { to: '/privacy', key: 'nav.privacy' },
    ],
  },
]
</script>

<template>
  <footer class="border-t border-line bg-surface">
    <div class="shell grid grid-cols-2 gap-x-8 gap-y-10 py-12 sm:grid-cols-3 lg:grid-cols-[1.5fr_repeat(3,1fr)] lg:gap-12 lg:py-16">
      <div class="col-span-2 space-y-4 sm:col-span-3 lg:col-span-1">
        <SiteLogo :name="settings.name" :icon-url="settings.icon_url" class="rounded-control" />
        <p class="max-w-xs text-pretty text-sm leading-7 text-muted">{{ t('site.footerBlurb') }}</p>
      </div>

      <nav v-for="group in groups" :key="group.titleKey" class="space-y-3" :aria-label="t(group.titleKey)">
        <h2 class="text-xs font-semibold tracking-wide text-ink">{{ t(group.titleKey) }}</h2>
        <ul class="space-y-1">
          <li v-for="link in group.links" :key="link.to">
            <NuxtLink :to="link.to" class="inline-flex min-h-11 items-center rounded-control py-2 text-sm text-muted transition-colors duration-150 ease-out hover:text-clay">
              {{ t(link.key) }}
            </NuxtLink>
          </li>
        </ul>
      </nav>
    </div>

    <div class="shell">
      <div class="flex flex-col gap-3 border-t border-line py-5 text-xs text-muted sm:flex-row sm:items-center sm:justify-between">
        <p>© {{ year }} {{ settings.name }}</p>
        <div class="flex flex-wrap items-center gap-x-5 gap-y-2">
          <a href="https://github.com/xinghai-osc/xinghai-router" target="_blank" rel="noopener noreferrer" class="inline-flex min-h-11 items-center rounded-control transition-colors duration-150 ease-out hover:text-clay">
            {{ t('site.footerOpenSource') }}
          </a>
          <p class="text-2xs">{{ t('site.footerCompat') }}</p>
        </div>
      </div>
    </div>
  </footer>
</template>

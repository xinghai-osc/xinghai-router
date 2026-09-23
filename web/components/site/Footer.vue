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
  <footer class="border-t border-line bg-sunken/30">
    <div class="shell grid grid-cols-2 gap-x-6 gap-y-10 py-14 sm:grid-cols-3 md:grid-cols-[1.5fr_repeat(3,1fr)] md:gap-10">
      <div class="col-span-2 space-y-4 sm:col-span-3 md:col-span-1">
        <SiteLogo :name="settings.name" :icon-url="settings.icon_url" />
        <p class="max-w-xs text-[13px] leading-relaxed text-muted">{{ t('site.footerBlurb') }}</p>
      </div>

      <div v-for="group in groups" :key="group.titleKey" class="space-y-3">
        <p class="text-2xs font-medium tracking-wide text-faint uppercase">{{ t(group.titleKey) }}</p>
        <ul class="space-y-2">
          <li v-for="link in group.links" :key="link.to">
            <NuxtLink :to="link.to" class="inline-flex py-1 text-[13px] text-muted transition-colors duration-150 ease-out hover:text-clay">
              {{ t(link.key) }}
            </NuxtLink>
          </li>
        </ul>
      </div>
    </div>

    <div class="shell flex flex-col gap-2 border-t border-line py-5 text-2xs text-faint sm:flex-row sm:items-center sm:justify-between">
      <p>© {{ year }} {{ settings.name }}</p>
      <p class="numeric">{{ t('site.footerCompat') }}</p>
    </div>
  </footer>
</template>

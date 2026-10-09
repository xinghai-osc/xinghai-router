<script setup lang="ts">
const { loadSiteSettings } = useSiteSettings()
const { t } = useI18n()
onMounted(loadSiteSettings)
</script>

<template>
  <div class="flex min-h-dvh flex-col bg-paper">
    <a href="#site-content" class="sr-only z-50 rounded-control bg-clay px-4 py-2 text-sm text-clay-ink focus:not-sr-only focus:fixed focus:top-2 focus:left-2">{{ t('common.skipToContent') }}</a>
    <SiteTopbar />
    <main id="site-content" tabindex="-1" class="min-w-0 flex-1 outline-none">
      <slot />
    </main>
    <SiteFooter />
    <UiToaster />
  </div>
</template>

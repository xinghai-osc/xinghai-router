<script setup lang="ts">
const { loadSiteSettings } = useSiteSettings()
onMounted(loadSiteSettings)
</script>

<template>
  <div class="flex min-h-dvh flex-col bg-paper">
    <SiteTopbar />
    <main class="flex-1">
      <slot />
    </main>
    <SiteFooter />
    <UiToaster />
  </div>
</template>

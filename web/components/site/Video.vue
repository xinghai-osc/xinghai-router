<script setup lang="ts">
const { settings } = useSiteSettings()
const { t } = useI18n()

const videoURL = computed(() => settings.value.homepage_video_url?.trim() ?? '')
</script>

<template>
  <section v-if="videoURL" class="border-b border-line py-12 md:py-16">
    <div class="shell">
      <h2 class="sr-only">{{ t('site.homepageVideo') }}</h2>
      <video
        :src="videoURL"
        controls
        playsinline
        preload="metadata"
        :aria-label="t('site.homepageVideo')"
        class="mx-auto aspect-video max-h-[min(70vh,720px)] w-full rounded-control border border-line bg-sunken object-contain"
      />
    </div>
  </section>
</template>

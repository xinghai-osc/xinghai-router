<script setup lang="ts">
import { ArrowRight } from 'lucide-vue-next'

const { t } = useI18n()
</script>

<template>
  <section class="shell py-20 md:py-28">
    <div class="flex flex-col items-center gap-6 rounded-card border border-line bg-sunken px-6 py-16 text-center">
      <h2 class="display max-w-lg text-4xl text-ink md:text-5xl">{{ t('site.ctaTitle') }}</h2>
      <p class="max-w-md text-muted">{{ t('site.ctaLead') }}</p>
      <div class="flex flex-wrap items-center justify-center gap-3">
        <UiButton to="/auth?mode=register" size="lg">
          {{ t('site.ctaPrimary') }}
          <ArrowRight class="size-4" />
        </UiButton>
        <UiButton to="/pricing" variant="secondary" size="lg">{{ t('site.ctaSecondary') }}</UiButton>
      </div>
    </div>
  </section>
</template>

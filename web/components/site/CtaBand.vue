<script setup lang="ts">
import { ArrowRight } from 'lucide-vue-next'

const { t } = useI18n()
</script>

<template>
  <section class="shell py-20 md:py-28">
    <div class="relative flex flex-col items-center gap-6 overflow-hidden rounded-card border border-line bg-sunken px-6 py-16 text-center">
      <div
        class="pointer-events-none absolute inset-0 opacity-60"
        style="background: radial-gradient(50% 60% at 50% 0%, var(--clay-soft), transparent 70%)"
        aria-hidden="true"
      />
      <h2 class="display relative max-w-lg text-4xl text-ink md:text-5xl">{{ t('site.ctaTitle') }}</h2>
      <p class="relative max-w-md text-muted">{{ t('site.ctaLead') }}</p>
      <div class="relative flex flex-wrap items-center justify-center gap-3">
        <UiButton to="/auth?mode=register" size="lg" class="group">
          {{ t('site.ctaPrimary') }}
          <ArrowRight class="size-4 transition-transform duration-150 group-hover:translate-x-0.5" />
        </UiButton>
        <UiButton to="/pricing" variant="secondary" size="lg">{{ t('site.ctaSecondary') }}</UiButton>
      </div>
    </div>
  </section>
</template>

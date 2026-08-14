<script setup lang="ts">
import { ArrowRight } from 'lucide-vue-next'

const { t } = useI18n()
</script>

<template>
  <section class="shell py-20 md:py-28">
    <div class="relative grid items-center gap-8 overflow-hidden rounded-card border border-line-strong bg-sunken px-6 py-12 md:grid-cols-[1fr_auto] md:px-12 md:py-14 md:text-left">
      <div
        class="pointer-events-none absolute inset-0 opacity-60"
        style="background: radial-gradient(50% 60% at 50% 0%, var(--clay-soft), transparent 70%)"
        aria-hidden="true"
      />
      <div class="relative space-y-3">
        <h2 class="display max-w-lg text-4xl text-ink md:text-5xl">{{ t('site.ctaTitle') }}</h2>
        <p class="max-w-md text-muted">{{ t('site.ctaLead') }}</p>
      </div>
      <div class="relative flex flex-wrap items-center gap-3 md:justify-end">
        <UiButton to="/auth?mode=register" size="lg" class="group">
          {{ t('site.ctaPrimary') }}
          <ArrowRight class="size-4 transition-transform duration-150 group-hover:translate-x-0.5" />
        </UiButton>
        <UiButton to="/pricing" variant="secondary" size="lg">{{ t('site.ctaSecondary') }}</UiButton>
      </div>
    </div>
  </section>
</template>

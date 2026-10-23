<script setup lang="ts">
import { ArrowRight, KeyRound } from 'lucide-vue-next'

const { t } = useI18n()
</script>

<template>
  <section class="shell py-20 md:py-24" aria-labelledby="cta-title">
    <div class="grid items-center gap-8 rounded-card border border-line bg-surface p-6 sm:p-10 lg:grid-cols-[minmax(0,1fr)_auto] lg:gap-16 lg:p-12">
      <div>
        <span class="mb-6 inline-flex size-11 items-center justify-center rounded-control bg-clay-soft text-clay">
          <KeyRound class="size-5" aria-hidden="true" />
        </span>
        <h2 id="cta-title" class="max-w-lg text-balance font-sans text-3xl font-semibold leading-tight tracking-[-0.035em] text-ink sm:text-4xl lg:text-5xl">{{ t('site.ctaTitle') }}</h2>
        <p class="mt-4 max-w-lg text-pretty text-sm leading-7 text-muted">{{ t('site.ctaLead') }}</p>
      </div>
      <div class="flex flex-col gap-3 sm:flex-row lg:min-w-52 lg:flex-col">
        <UiButton to="/auth?mode=register" size="lg" class="group justify-center">
          {{ t('site.ctaPrimary') }}
          <ArrowRight class="size-4 transition-transform duration-150 ease-out group-hover:translate-x-0.5 motion-reduce:transform-none" aria-hidden="true" />
        </UiButton>
        <UiButton to="/pricing" variant="secondary" size="lg" class="justify-center">{{ t('site.ctaSecondary') }}</UiButton>
      </div>
    </div>
  </section>
</template>

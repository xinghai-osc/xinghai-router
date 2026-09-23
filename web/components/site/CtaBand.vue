<script setup lang="ts">
import { ArrowRight, KeyRound } from 'lucide-vue-next'

const { t } = useI18n()
</script>

<template>
  <section class="shell py-20 md:py-24">
    <div class="cta-shell relative overflow-hidden rounded-[1.5rem] border border-clay/25 bg-surface px-6 py-10 md:px-12 md:py-14">
      <div class="pointer-events-none absolute -top-24 -right-24 size-80 rounded-full border border-clay/10" aria-hidden="true" />
      <div class="pointer-events-none absolute -top-12 -right-12 size-56 rounded-full border border-clay/10" aria-hidden="true" />
      <div class="relative grid items-center gap-8 lg:grid-cols-[1fr_auto]">
        <div>
          <span class="mb-6 flex size-11 items-center justify-center rounded-xl border border-clay/20 bg-clay-soft text-clay">
            <KeyRound class="size-5" aria-hidden="true" />
          </span>
          <h2 class="display max-w-lg text-4xl text-ink md:text-5xl">{{ t('site.ctaTitle') }}</h2>
          <p class="mt-4 max-w-lg text-sm leading-7 text-muted">{{ t('site.ctaLead') }}</p>
        </div>
        <div class="flex flex-col gap-3 sm:flex-row lg:flex-col">
          <UiButton to="/auth?mode=register" size="lg" class="group justify-center">
            {{ t('site.ctaPrimary') }}
            <ArrowRight class="size-4 transition-transform duration-150 ease-out group-hover:translate-x-0.5" aria-hidden="true" />
          </UiButton>
          <UiButton to="/pricing" variant="secondary" size="lg" class="justify-center">{{ t('site.ctaSecondary') }}</UiButton>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.cta-shell {
  background-image: radial-gradient(ellipse at top right, var(--clay-soft), transparent 65%);
}
</style>

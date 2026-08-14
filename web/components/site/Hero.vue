<script setup lang="ts">
import { ArrowRight } from 'lucide-vue-next'

const props = defineProps<{ modelCount?: number }>()
const { t } = useI18n()

const countCopy = computed(() =>
  props.modelCount
    ? t('site.heroBodyCount', { count: props.modelCount })
    : t('site.heroBodyNoCount'))
</script>

<template>
  <section class="relative overflow-hidden">
    <div
      class="pointer-events-none absolute inset-x-0 -top-40 h-80 opacity-70"
      style="background: radial-gradient(60% 100% at 50% 100%, var(--clay-soft), transparent 70%)"
      aria-hidden="true"
    />
    <div
      class="pointer-events-none absolute inset-0 opacity-[0.035]"
      style="background-image: linear-gradient(var(--ink) 1px, transparent 1px), linear-gradient(90deg, var(--ink) 1px, transparent 1px); background-size: 56px 56px; mask-image: radial-gradient(60% 50% at 50% 30%, black, transparent 80%); -webkit-mask-image: radial-gradient(60% 50% at 50% 30%, black, transparent 80%)"
      aria-hidden="true"
    />

    <div class="shell relative flex flex-col items-center gap-7 py-20 text-center md:py-28">
      <UiBadge tone="clay" class="animate-rise">{{ t('site.heroBadge') }}</UiBadge>

      <h1 class="display animate-rise max-w-3xl text-[2.75rem] text-ink sm:text-6xl md:text-7xl" style="animation-delay: 60ms">
        {{ t('site.heroTitleLine1') }}<br>{{ t('site.heroTitleLine2') }}
      </h1>

      <p class="animate-rise max-w-xl text-base leading-relaxed text-muted md:text-lg" style="animation-delay: 120ms">
        {{ t('site.heroBodyLead') }}{{ countCopy }}{{ t('site.heroBodyTail') }}
      </p>

      <div class="animate-rise flex flex-wrap items-center justify-center gap-3" style="animation-delay: 180ms">
        <UiButton to="/auth?mode=register" size="lg" class="group">
          {{ t('site.heroPrimary') }}
          <ArrowRight class="size-4 transition-transform duration-150 group-hover:translate-x-0.5" />
        </UiButton>
        <UiButton to="/models" variant="secondary" size="lg">{{ t('site.heroSecondary') }}</UiButton>
      </div>

      <div class="animate-rise mt-6 w-full max-w-2xl" style="animation-delay: 240ms">
        <SiteCodeSample />
      </div>
    </div>
  </section>
</template>

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
      class="pointer-events-none absolute inset-x-0 -top-40 h-80 opacity-60"
      style="background: radial-gradient(60% 100% at 50% 100%, var(--clay-soft), transparent 70%)"
      aria-hidden="true"
    />

    <div class="shell relative flex flex-col items-center gap-7 py-20 text-center md:py-28">
      <UiBadge tone="clay">{{ t('site.heroBadge') }}</UiBadge>

      <h1 class="display max-w-3xl text-[2.75rem] text-ink sm:text-6xl md:text-7xl">
        {{ t('site.heroTitleLine1') }}<br>{{ t('site.heroTitleLine2') }}
      </h1>

      <p class="max-w-xl text-base leading-relaxed text-muted md:text-lg">
        {{ t('site.heroBodyLead') }}{{ countCopy }}{{ t('site.heroBodyTail') }}
      </p>

      <div class="flex flex-wrap items-center justify-center gap-3">
        <UiButton to="/auth?mode=register" size="lg">
          {{ t('site.heroPrimary') }}
          <ArrowRight class="size-4" />
        </UiButton>
        <UiButton to="/models" variant="secondary" size="lg">{{ t('site.heroSecondary') }}</UiButton>
      </div>

      <div class="mt-6 w-full max-w-2xl">
        <SiteCodeSample />
      </div>
    </div>
  </section>
</template>

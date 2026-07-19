<script setup lang="ts">
const props = defineProps<{ kind: 'terms' | 'privacy' }>()
const { locale, t, initializeLocale } = useI18n()

const isTerms = computed(() => props.kind === 'terms')
const title = computed(() => isTerms.value ? t('termsTitle') : t('privacyTitle'))
const updatedAt = '2026-07-16'

onMounted(() => {
  initializeLocale()
})
</script>

<template>
  <main class="min-h-screen bg-background text-foreground">
    <PublicTopbar />

    <article class="mx-auto w-full max-w-3xl px-4 py-12 sm:py-16">
      <header class="mb-10 border-b border-border pb-6">
        <p class="text-[10px] font-bold uppercase tracking-wider text-muted-foreground">XINGHAI ROUTER / {{ isTerms ? 'TERMS' : 'PRIVACY' }}</p>
        <h1 class="mt-3 text-3xl font-bold tracking-tight sm:text-4xl">{{ title }}</h1>
        <p class="mt-2 text-sm text-muted-foreground">{{ t('updatedAtLabel') }}{{ updatedAt }}</p>
      </header>

      <div class="flex flex-col gap-8 text-sm leading-relaxed text-foreground/90">
        <template v-if="locale === 'zh-CN' && isTerms">
          <p class="text-base text-muted-foreground">{{ t('termsServiceLead') }}</p>
          <section><h2 class="mb-2 text-lg font-semibold">{{ t('termsServiceTitle1') }}</h2><p>{{ t('termsServiceBody1') }}</p></section>
          <section><h2 class="mb-2 text-lg font-semibold">{{ t('termsServiceTitle2') }}</h2><p>{{ t('termsServiceBody2') }}</p></section>
          <section><h2 class="mb-2 text-lg font-semibold">{{ t('termsServiceTitle3') }}</h2><p>{{ t('termsServiceBody3') }}</p></section>
          <section><h2 class="mb-2 text-lg font-semibold">{{ t('termsServiceTitle4') }}</h2><p>{{ t('termsServiceBody4') }}</p></section>
          <section><h2 class="mb-2 text-lg font-semibold">{{ t('termsServiceTitle5') }}</h2><p>{{ t('termsServiceBody5') }}</p></section>
          <section><h2 class="mb-2 text-lg font-semibold">{{ t('termsServiceTitle6') }}</h2><p>{{ t('termsServiceBody6') }}</p></section>
          <section><h2 class="mb-2 text-lg font-semibold">{{ t('termsServiceTitle7') }}</h2><p>{{ t('termsServiceBody7') }}</p></section>
          <section><h2 class="mb-2 text-lg font-semibold">{{ t('termsServiceTitle8') }}</h2><p>{{ t('termsServiceBody8') }}</p></section>
        </template>

        <template v-else-if="locale === 'zh-CN'">
          <p class="text-base text-muted-foreground">{{ t('privacyPolicyLead') }}</p>
          <section><h2 class="mb-2 text-lg font-semibold">{{ t('privacyTitle1') }}</h2><p>{{ t('privacyBody1') }}</p></section>
          <section><h2 class="mb-2 text-lg font-semibold">{{ t('privacyTitle2') }}</h2><p>{{ t('privacyBody2') }}</p></section>
          <section><h2 class="mb-2 text-lg font-semibold">{{ t('privacyTitle3') }}</h2><p>{{ t('privacyBody3') }}</p></section>
          <section><h2 class="mb-2 text-lg font-semibold">{{ t('privacyTitle4') }}</h2><p>{{ t('privacyBody4') }}</p></section>
          <section><h2 class="mb-2 text-lg font-semibold">{{ t('privacyTitle5') }}</h2><p>{{ t('privacyBody5') }}</p></section>
          <section><h2 class="mb-2 text-lg font-semibold">{{ t('privacyTitle6') }}</h2><p>{{ t('privacyBody6') }}</p></section>
          <section><h2 class="mb-2 text-lg font-semibold">{{ t('privacyTitle7') }}</h2><p>{{ t('privacyBody7') }}</p></section>
        </template>

        <template v-else-if="isTerms">
          <p class="text-base text-muted-foreground">{{ t('termsServiceLead') }}</p>
          <section><h2 class="mb-2 text-lg font-semibold">{{ t('termsServiceTitle1') }}</h2><p>{{ t('termsServiceBody1') }}</p></section>
          <section><h2 class="mb-2 text-lg font-semibold">{{ t('termsServiceTitle2') }}</h2><p>{{ t('termsServiceBody2') }}</p></section>
          <section><h2 class="mb-2 text-lg font-semibold">{{ t('termsServiceTitle3') }}</h2><p>{{ t('termsServiceBody3') }}</p></section>
          <section><h2 class="mb-2 text-lg font-semibold">{{ t('termsServiceTitle4') }}</h2><p>{{ t('termsServiceBody4') }}</p></section>
          <section><h2 class="mb-2 text-lg font-semibold">{{ t('termsServiceTitle5') }}</h2><p>{{ t('termsServiceBody5') }}</p></section>
          <section><h2 class="mb-2 text-lg font-semibold">{{ t('termsServiceTitle6') }}</h2><p>{{ t('termsServiceBody6') }}</p></section>
          <section><h2 class="mb-2 text-lg font-semibold">{{ t('termsServiceTitle7') }}</h2><p>{{ t('termsServiceBody7') }}</p></section>
          <section><h2 class="mb-2 text-lg font-semibold">{{ t('termsServiceTitle8') }}</h2><p>{{ t('termsServiceBody8') }}</p></section>
        </template>

        <template v-else>
          <p class="text-base text-muted-foreground">{{ t('privacyPolicyLead') }}</p>
          <section><h2 class="mb-2 text-lg font-semibold">{{ t('privacyTitle1') }}</h2><p>{{ t('privacyBody1') }}</p></section>
          <section><h2 class="mb-2 text-lg font-semibold">{{ t('privacyTitle2') }}</h2><p>{{ t('privacyBody2') }}</p></section>
          <section><h2 class="mb-2 text-lg font-semibold">{{ t('privacyTitle3') }}</h2><p>{{ t('privacyBody3') }}</p></section>
          <section><h2 class="mb-2 text-lg font-semibold">{{ t('privacyTitle4') }}</h2><p>{{ t('privacyBody4') }}</p></section>
          <section><h2 class="mb-2 text-lg font-semibold">{{ t('privacyTitle5') }}</h2><p>{{ t('privacyBody5') }}</p></section>
          <section><h2 class="mb-2 text-lg font-semibold">{{ t('privacyTitle6') }}</h2><p>{{ t('privacyBody6') }}</p></section>
          <section><h2 class="mb-2 text-lg font-semibold">{{ t('privacyTitle7') }}</h2><p>{{ t('privacyBody7') }}</p></section>
        </template>
      </div>
    </article>
  </main>
</template>

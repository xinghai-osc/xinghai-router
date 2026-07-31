<script setup lang="ts">
export interface LegalTocEntry { id: string; label: string }

defineProps<{
  title: string
  updated: string
  intro: string
  toc: LegalTocEntry[]
}>()

const { t } = useI18n()

// No typography plugin in this project — prose rhythm is declared here once and
// inherited by whatever the slot renders.
const PROSE = [
  '[&_section]:scroll-mt-24',
  '[&_section+section]:mt-12',
  '[&_h2]:text-[15px] [&_h2]:font-semibold [&_h2]:text-ink',
  '[&_p]:mt-3 [&_p]:text-[14px] [&_p]:leading-7 [&_p]:text-muted',
  '[&_ul]:mt-3 [&_ul]:list-disc [&_ul]:space-y-1.5 [&_ul]:pl-5',
  '[&_li]:text-[14px] [&_li]:leading-7 [&_li]:text-muted [&_li]:marker:text-clay',
].join(' ')
</script>

<template>
  <article class="shell pt-16 pb-24 md:pt-20">
    <header class="max-w-2xl space-y-3 border-b border-line pb-8">
      <p class="text-2xs font-medium tracking-wide text-clay uppercase">{{ t('nav.legal') }}</p>
      <h1 class="text-3xl font-semibold tracking-tight text-ink md:text-4xl">{{ title }}</h1>
      <p class="numeric text-2xs text-faint">{{ t('site.legalUpdated', { date: updated }) }}</p>
      <p class="pt-2 text-[14px] leading-7 text-muted">{{ intro }}</p>
    </header>

    <div class="mt-10 gap-12 lg:grid lg:grid-cols-[15rem_minmax(0,1fr)]">
      <nav class="hidden lg:block" :aria-label="t('site.legalToc')">
        <div class="sticky top-24 space-y-3">
          <p class="text-2xs font-medium tracking-wide text-faint uppercase">{{ t('site.legalToc') }}</p>
          <ul class="space-y-1.5">
            <li v-for="entry in toc" :key="entry.id">
              <a
                :href="`#${entry.id}`"
                class="block rounded-control px-2 py-1 text-[13px] text-muted transition-colors duration-150 hover:bg-sunken hover:text-ink"
              >{{ entry.label }}</a>
            </li>
          </ul>
        </div>
      </nav>

      <div :class="['max-w-2xl', PROSE]">
        <slot />
      </div>
    </div>
  </article>
</template>

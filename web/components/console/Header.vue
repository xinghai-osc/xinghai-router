<script setup lang="ts">
import { PanelLeft } from 'lucide-vue-next'
import { NAV_TITLE_KEYS } from '~/src/nav'

defineProps<{ title?: string }>()
defineEmits<{ 'open-nav': [] }>()

const route = useRoute()
const { t } = useI18n()

const heading = computed(() => {
  const key = NAV_TITLE_KEYS[route.path]
  return key ? t(key) : t('common.console')
})
</script>

<template>
  <header class="sticky top-3 z-30 mx-4 mt-4 flex min-h-14 flex-wrap items-center gap-x-2 gap-y-2 rounded-card border border-line bg-surface/95 px-2 py-2 backdrop-blur-md sm:gap-x-3 sm:px-4 md:mx-8 xl:mx-10">
    <button
      type="button"
      class="inline-flex size-9 shrink-0 items-center justify-center rounded-control text-muted transition-colors duration-150 ease-out hover:bg-sunken hover:text-ink focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-clay lg:hidden"
      :aria-label="t('common.openNav')"
      @click="$emit('open-nav')"
    >
      <PanelLeft class="size-[18px]" aria-hidden="true" />
    </button>

    <h1 class="min-w-0 flex-1 truncate text-[15px] font-semibold tracking-tight text-ink">{{ title ?? heading }}</h1>

    <div class="ml-auto flex max-w-full flex-wrap items-center justify-end gap-0.5 sm:gap-1">
      <slot name="actions" />
      <SiteLocaleToggle />
      <SiteThemeToggle />
      <div v-if="$slots.account" class="ml-1 flex shrink-0 items-center border-l border-line pl-1.5 sm:ml-2 sm:pl-3">
        <slot name="account" />
      </div>
    </div>
  </header>
</template>

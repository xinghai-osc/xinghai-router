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
  <header class="sticky top-0 z-30 mx-3 mt-3 flex h-14 items-center gap-3 rounded-full border border-line-strong bg-surface/75 px-4 shadow-pop backdrop-blur-xl md:mx-6 md:px-6 xl:mx-8">
    <button
      type="button"
      class="inline-flex size-9 items-center justify-center rounded-full text-muted transition-colors hover:bg-sunken hover:text-ink lg:hidden"
      :aria-label="t('common.openNav')"
      @click="$emit('open-nav')"
    >
      <PanelLeft class="size-[18px]" />
    </button>

    <h1 class="min-w-0 flex-1 truncate text-[15px] font-semibold tracking-tight text-ink">{{ title ?? heading }}</h1>

    <div class="flex items-center gap-1.5">
      <slot name="actions" />
      <SiteLocaleToggle />
      <SiteThemeToggle />
      <slot name="account" />
    </div>
  </header>
</template>

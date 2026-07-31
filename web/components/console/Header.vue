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
  <header class="sticky top-0 z-30 flex h-16 items-center gap-3 border-b border-line bg-paper/85 px-4 backdrop-blur-md md:px-8">
    <button
      type="button"
      class="inline-flex size-9 items-center justify-center rounded-control text-muted transition-colors hover:bg-sunken hover:text-ink lg:hidden"
      :aria-label="t('common.openNav')"
      @click="$emit('open-nav')"
    >
      <PanelLeft class="size-[18px]" />
    </button>

    <h1 class="min-w-0 flex-1 truncate text-[15px] font-semibold text-ink">{{ title ?? heading }}</h1>

    <div class="flex items-center gap-1.5">
      <slot name="actions" />
      <SiteLocaleToggle />
      <SiteThemeToggle />
      <slot name="account" />
    </div>
  </header>
</template>

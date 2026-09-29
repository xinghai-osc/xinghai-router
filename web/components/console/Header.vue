<script setup lang="ts">
import { ChevronRight, LayoutGrid, PanelLeft } from 'lucide-vue-next'
import { NAV_TITLE_KEYS } from '~/src/nav'

defineProps<{ title?: string }>()
defineEmits<{ 'open-nav': [] }>()

const route = useRoute()
const { t } = useI18n()
const { current: workspace } = useWorkspace()

const heading = computed(() => {
  const key = NAV_TITLE_KEYS[route.path]
  return key ? t(key) : t('common.console')
})
</script>

<template>
  <header class="sticky top-0 z-30 flex min-h-16 items-center gap-2 border-b border-line bg-surface/95 px-3 py-2 backdrop-blur-md sm:gap-3 sm:px-6 lg:px-8">
    <button
      type="button"
      class="inline-flex size-9 shrink-0 items-center justify-center rounded-control text-muted transition-colors duration-150 ease-out hover:bg-sunken hover:text-ink lg:hidden"
      :aria-label="t('common.openNav')"
      aria-haspopup="dialog"
      @click="$emit('open-nav')"
    >
      <PanelLeft class="size-[18px]" aria-hidden="true" />
    </button>

    <div class="hidden items-center gap-2.5 text-[13px] text-muted md:flex">
      <NuxtLink to="/console" class="transition-colors duration-150 ease-out hover:text-clay">{{ t('common.console') }}</NuxtLink>
      <ChevronRight class="size-3.5 text-faint" aria-hidden="true" />
    </div>
    <div class="min-w-0 flex-1">
      <h1 class="truncate text-sm font-semibold text-ink">{{ title ?? heading }}</h1>
      <p v-if="workspace" class="truncate text-2xs text-muted">{{ workspace.is_personal ? t('console.workspacePersonal') : workspace.name }}</p>
    </div>

    <div class="ml-auto flex shrink-0 items-center gap-0.5 sm:gap-1">
      <slot name="actions" />
      <NuxtLink to="/models" class="mr-2 hidden items-center gap-2 rounded-control px-3 py-2 text-xs text-muted transition-colors duration-150 ease-out hover:bg-sunken hover:text-ink xl:inline-flex">
        <LayoutGrid class="size-4" aria-hidden="true" />
        {{ t('nav.models') }}
      </NuxtLink>
      <SiteLocaleToggle />
      <SiteThemeToggle />
      <div v-if="$slots.account" class="ml-1 flex shrink-0 items-center border-l border-line pl-1.5 sm:ml-2 sm:pl-3">
        <slot name="account" />
      </div>
    </div>
  </header>
</template>

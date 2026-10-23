<script setup lang="ts">
import { ChevronRight, LayoutGrid, PanelLeft } from 'lucide-vue-next'
import { NAV_TITLE_KEYS } from '~/src/nav'

defineProps<{ title?: string; navOpen?: boolean }>()
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
  <header class="console-header glass-control sticky top-2 z-30 mx-2 mt-2 rounded-card border border-line/70 [--glass-opacity:72%] sm:top-3 sm:mx-3 sm:mt-3">
    <div class="mx-auto flex min-h-18 w-full max-w-[var(--layout-console-width)] items-center gap-3 px-4 sm:gap-4 sm:px-6 lg:px-8 xl:px-10">
      <button
        type="button"
        class="glass-control inline-flex size-10 shrink-0 items-center justify-center rounded-full border border-line/50 text-muted transition-colors duration-150 ease-out hover:bg-sunken hover:text-ink focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-clay lg:hidden"
        :aria-label="t('common.openNav')"
        :aria-expanded="navOpen"
        aria-controls="console-mobile-nav"
        aria-haspopup="dialog"
        @click="$emit('open-nav')"
      >
        <PanelLeft class="size-5" aria-hidden="true" />
      </button>

      <div class="min-w-0 flex-1 py-3">
        <div class="flex min-w-0 items-center gap-2 text-sm">
          <template v-if="route.path !== '/console'">
            <NuxtLink to="/console" class="hidden shrink-0 text-muted transition-colors duration-150 ease-out hover:text-clay md:block">{{ t('common.console') }}</NuxtLink>
            <ChevronRight class="hidden size-3.5 shrink-0 text-faint md:block" aria-hidden="true" />
          </template>
          <h1 class="truncate font-semibold text-ink" :title="title ?? heading">{{ title ?? heading }}</h1>
        </div>
        <p v-if="workspace" class="mt-1 truncate text-xs text-muted" :title="workspace.is_personal ? t('console.workspacePersonal') : workspace.name">{{ workspace.is_personal ? t('console.workspacePersonal') : workspace.name }}</p>
      </div>

      <div class="console-header-actions ml-auto flex shrink-0 items-center gap-0.5 sm:gap-1">
        <slot name="actions" />
        <NuxtLink to="/models" class="mr-2 hidden items-center gap-2 rounded-control px-3 py-2 text-xs font-medium text-muted transition-colors duration-150 ease-out hover:bg-sunken hover:text-ink xl:inline-flex">
          <LayoutGrid class="size-4" aria-hidden="true" />
          {{ t('nav.models') }}
        </NuxtLink>
        <SiteLocaleToggle />
        <SiteThemeToggle />
        <div v-if="$slots.account" class="ml-1 flex shrink-0 items-center border-l border-line pl-1.5 sm:ml-2 sm:pl-3">
          <slot name="account" />
        </div>
      </div>
    </div>
  </header>
</template>

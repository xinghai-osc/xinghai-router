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
  <header class="sticky top-2 z-30 mx-2 mt-2 flex min-h-16 items-center gap-2 px-3 py-2 sm:top-3 sm:mx-3 sm:mt-3 sm:gap-3 sm:px-4 lg:top-6 lg:px-5">
    <button
      type="button"
      class="header-glass inline-flex size-9 shrink-0 items-center justify-center rounded-full border text-muted transition-colors duration-150 ease-out hover:bg-sunken hover:text-ink lg:hidden"
      :aria-label="t('common.openNav')"
      aria-haspopup="dialog"
      @click="$emit('open-nav')"
    >
      <PanelLeft class="size-[18px]" aria-hidden="true" />
    </button>

    <div class="header-glass flex min-w-0 items-center gap-2.5 rounded-full border px-3 py-2 sm:px-4">
      <div class="hidden shrink-0 items-center gap-2.5 text-[13px] text-muted md:flex">
        <NuxtLink to="/console" class="transition-colors duration-150 ease-out hover:text-clay">{{ t('common.console') }}</NuxtLink>
        <ChevronRight class="size-3.5 text-faint" aria-hidden="true" />
      </div>
      <div class="min-w-0">
        <h1 :key="route.path" class="header-heading truncate text-sm font-semibold text-ink">{{ title ?? heading }}</h1>
        <p v-if="workspace" class="truncate text-2xs text-muted">{{ workspace.is_personal ? t('console.workspacePersonal') : workspace.name }}</p>
      </div>
    </div>

    <div class="header-actions ml-auto flex shrink-0 items-center gap-0.5 p-1 sm:gap-1">
      <slot name="actions" />
      <NuxtLink to="/models" class="mr-2 hidden items-center gap-2 rounded-full px-3 py-2 text-xs text-muted transition-colors duration-150 ease-out hover:bg-sunken hover:text-ink xl:inline-flex">
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

<style scoped>
.header-glass {
  position: relative;
  isolation: isolate;
  border-color: color-mix(in srgb, var(--line) 65%, transparent);
  background:
    radial-gradient(ellipse at 0% 0%, color-mix(in srgb, var(--clay) 7%, transparent), transparent 65%),
    linear-gradient(125deg, color-mix(in srgb, var(--surface) 55%, transparent), color-mix(in srgb, var(--surface) 20%, transparent));
  -webkit-backdrop-filter: blur(24px) saturate(150%);
  backdrop-filter: blur(24px) saturate(150%);
}

.header-glass::before {
  position: absolute;
  inset: 0;
  z-index: -1;
  padding: 1px;
  border-radius: inherit;
  background: linear-gradient(135deg, var(--shimmer-highlight), transparent 40%, color-mix(in srgb, var(--surface) 35%, transparent));
  content: '';
  pointer-events: none;
  -webkit-mask: linear-gradient(var(--ink) 0 0) content-box, linear-gradient(var(--ink) 0 0);
  mask: linear-gradient(var(--ink) 0 0) content-box, linear-gradient(var(--ink) 0 0);
  -webkit-mask-composite: xor;
  mask-composite: exclude;
}

@supports not ((backdrop-filter: blur(1px)) or (-webkit-backdrop-filter: blur(1px))) {
  .header-glass {
    background: color-mix(in srgb, var(--surface) 95%, transparent);
  }
}

@media (prefers-reduced-transparency: reduce) {
  .header-glass {
    background: var(--surface);
    -webkit-backdrop-filter: none;
    backdrop-filter: none;
  }
}

@media (prefers-reduced-motion: no-preference) {
  .header-heading {
    animation: rise 150ms ease-out;
  }
}
</style>

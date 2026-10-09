<script setup lang="ts">
import { ArrowUpRight, Check, ChevronsUpDown, Home, Layers, PanelLeftClose, PanelLeftOpen } from 'lucide-vue-next'
import { NAV_SECTIONS } from '~/src/nav'

const props = withDefaults(defineProps<{
  can?: (permission: string) => boolean
  siteName?: string
  iconUrl?: string
  collapsed?: boolean
  collapsible?: boolean
}>(), { can: () => false })

defineEmits<{ toggle: [] }>()

const route = useRoute()
const { t } = useI18n()
const { current: workspace, workspaces, ready: workspaceReady, loading: workspaceLoading, switchWorkspace } = useWorkspace()

const sections = computed(() =>
  NAV_SECTIONS
    .map(section => ({ ...section, items: section.items.filter(item => !item.permission || props.can(item.permission)) }))
    .filter(section => section.items.length > 0),
)

function isActive(to: string) {
  if (to === '/console') return route.path === '/console'
  return route.path === to || route.path.startsWith(`${to}/`)
}
</script>

<template>
  <div class="flex h-full min-h-0 flex-col">
    <div :class="['flex h-[4.75rem] shrink-0 items-center', collapsed ? 'justify-center px-2' : 'px-5']">
      <SiteLogo :name="siteName" :icon-url="iconUrl" :compact="collapsed" class="max-w-full" />
    </div>

    <div class="mx-3 mt-1 shrink-0">
      <UiDropdownMenu align="start" :side="collapsed ? 'right' : 'bottom'">
        <template #trigger>
          <button
            type="button"
            :class="['flex w-full items-center gap-3 rounded-control border border-line bg-surface py-3 text-left transition-colors duration-150 ease-out hover:bg-sunken', collapsed ? 'justify-center px-1' : 'px-3']"
            :aria-label="t('console.workspaceSwitch')"
            :title="t('console.workspaceSwitch')"
            :disabled="!workspaceReady || workspaceLoading"
          >
            <Layers class="size-[18px] shrink-0 text-clay" aria-hidden="true" />
            <span v-if="!collapsed" class="min-w-0 flex-1">
              <span class="block truncate text-[13px] font-semibold text-ink">{{ workspace ? (workspace.is_personal ? t('console.workspacePersonal') : workspace.name) : t('console.workspaceLoading') }}</span>
              <span class="block truncate text-2xs text-muted">{{ t('console.workspaceSwitch') }}</span>
            </span>
            <ChevronsUpDown v-if="!collapsed" class="size-3.5 shrink-0 text-faint" aria-hidden="true" />
          </button>
        </template>
        <UiDropdownItem as="label">{{ t('console.workspaceSwitch') }}</UiDropdownItem>
        <div class="max-h-64 overflow-y-auto">
          <UiDropdownItem v-for="item in workspaces" :key="item.id" :disabled="workspaceLoading" @select="switchWorkspace(item.id)">
            <Check class="size-4 shrink-0" :class="item.id === workspace?.id ? 'text-clay' : 'invisible'" aria-hidden="true" />
            <span class="max-w-52 truncate">{{ item.is_personal ? t('console.workspacePersonal') : item.name }}</span>
          </UiDropdownItem>
        </div>
        <UiDropdownItem as="separator" />
        <UiDropdownItem @select="navigateTo('/console/workspaces')">{{ t('console.workspaceManage') }}</UiDropdownItem>
      </UiDropdownMenu>
    </div>

    <nav class="min-h-0 flex-1 space-y-6 overflow-y-auto overscroll-contain px-3 py-5" :aria-label="t('common.console')">
      <div v-for="section in sections" :key="section.titleKey" class="space-y-1">
        <p :class="collapsed ? 'sr-only' : 'px-3 pb-1.5 text-2xs font-semibold tracking-wider text-muted'">{{ t(section.titleKey) }}</p>
        <div v-if="collapsed" class="mx-2 mb-3 border-t border-line" aria-hidden="true" />
        <NuxtLink
          v-for="item in section.items"
          :key="item.to"
          :to="item.to"
          :class="[
            'console-nav-link group relative flex min-h-11 items-center gap-3 rounded-control text-[13px] leading-5 transition-colors duration-150 ease-out',
            collapsed ? 'justify-center px-2 py-2.5' : 'px-3 py-2.5',
            isActive(item.to) ? 'bg-clay-soft font-semibold text-clay' : 'text-muted hover:bg-sunken hover:text-ink',
          ]"
          :aria-current="isActive(item.to) ? 'page' : undefined"
          :aria-label="collapsed ? t(item.labelKey) : undefined"
          :title="collapsed ? t(item.labelKey) : undefined"
        >
          <component :is="item.icon" class="size-[18px] shrink-0" :stroke-width="1.75" aria-hidden="true" />
          <span v-if="!collapsed" class="truncate">{{ t(item.labelKey) }}</span>
        </NuxtLink>
      </div>
    </nav>

    <div class="shrink-0 space-y-1 border-t border-line p-3">
      <NuxtLink
        to="/"
        :class="['flex min-h-10 items-center gap-3 rounded-control text-[13px] text-muted transition-colors duration-150 ease-out hover:bg-sunken hover:text-ink', collapsed ? 'justify-center px-2' : 'px-3']"
        :aria-label="t('common.home')"
        :title="collapsed ? t('common.home') : undefined"
      >
        <Home class="size-[18px] shrink-0" :stroke-width="1.75" aria-hidden="true" />
        <template v-if="!collapsed">
          <span class="flex-1">{{ t('common.home') }}</span>
          <ArrowUpRight class="size-3.5 text-faint" aria-hidden="true" />
        </template>
      </NuxtLink>
      <button
        v-if="collapsible"
        type="button"
        :class="['flex min-h-10 w-full items-center gap-3 rounded-control text-[13px] text-muted transition-colors duration-150 ease-out hover:bg-sunken hover:text-ink', collapsed ? 'justify-center px-2' : 'px-3']"
        :aria-label="collapsed ? t('common.expandNav') : t('common.collapseNav')"
        :aria-expanded="!collapsed"
        :title="collapsed ? t('common.expandNav') : undefined"
        @click="$emit('toggle')"
      >
        <component :is="collapsed ? PanelLeftOpen : PanelLeftClose" class="size-[18px] shrink-0" :stroke-width="1.75" aria-hidden="true" />
        <span v-if="!collapsed">{{ t('common.collapseNav') }}</span>
      </button>
    </div>
  </div>
</template>

<style scoped>
.console-nav-link::before {
  content: '';
  position: absolute;
  inset-inline-start: 0;
  width: 3px;
  height: 16px;
  border-radius: 999px;
  background: var(--clay);
  opacity: 0;
  transform: scaleY(0.4);
  transition: opacity 150ms ease-out, transform 150ms ease-out;
}

.console-nav-link[aria-current='page']::before {
  opacity: 1;
  transform: scaleY(1);
}

.console-nav-link :deep(svg) {
  transition: transform 150ms ease-out;
}

@media (hover: hover) and (prefers-reduced-motion: no-preference) {
  .console-nav-link:hover :deep(svg) {
    transform: translateX(2px);
  }
}
</style>

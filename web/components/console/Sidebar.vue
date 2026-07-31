<script setup lang="ts">
import { NAV_SECTIONS } from '~/src/nav'

const props = withDefaults(defineProps<{
  can?: (permission: string) => boolean
  siteName?: string
  iconUrl?: string
}>(), { can: () => false })

const route = useRoute()
const { t } = useI18n()

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
  <div class="flex h-full flex-col gap-6 overflow-y-auto px-3 py-5">
    <div class="px-2">
      <SiteLogo :name="siteName" :icon-url="iconUrl" />
    </div>

    <nav class="flex flex-1 flex-col gap-6" :aria-label="t('common.console')">
      <div v-for="section in sections" :key="section.titleKey" class="space-y-1">
        <p class="px-2 pb-1 text-2xs font-medium tracking-wide text-faint uppercase">{{ t(section.titleKey) }}</p>
        <NuxtLink
          v-for="item in section.items"
          :key="item.to"
          :to="item.to"
          :class="[
            'flex items-center gap-2.5 rounded-control px-2 py-2 text-[13px] transition-colors duration-150',
            isActive(item.to)
              ? 'bg-clay-soft font-medium text-clay'
              : 'text-muted hover:bg-sunken hover:text-ink',
          ]"
          :aria-current="isActive(item.to) ? 'page' : undefined"
        >
          <component :is="item.icon" class="size-4 shrink-0" />
          <span class="truncate">{{ t(item.labelKey) }}</span>
        </NuxtLink>
      </div>
    </nav>
  </div>
</template>

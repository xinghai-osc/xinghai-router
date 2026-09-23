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
  <div class="flex h-full min-h-0 flex-col gap-6 overflow-y-auto overscroll-contain px-3 py-4">
    <div class="flex min-h-14 shrink-0 items-center rounded-card border border-line bg-surface px-3 py-3">
      <SiteLogo :name="siteName" :icon-url="iconUrl" class="w-full [&>span]:min-w-0" />
    </div>

    <nav class="flex flex-1 flex-col gap-6 pb-2" :aria-label="t('common.console')">
      <div v-for="section in sections" :key="section.titleKey" class="space-y-1">
        <p class="px-3 pb-1.5 text-2xs font-semibold tracking-wider text-faint uppercase">{{ t(section.titleKey) }}</p>
        <NuxtLink
          v-for="item in section.items"
          :key="item.to"
          :to="item.to"
          :class="[
            'group relative flex min-h-10 items-center gap-2.5 rounded-control border px-3 py-2 text-[13px] leading-5 transition-colors duration-150 ease-out focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-clay',
            isActive(item.to)
              ? 'border-clay/20 bg-clay-soft font-semibold text-clay before:absolute before:inset-y-2.5 before:left-0 before:w-0.5 before:rounded-full before:bg-clay'
              : 'border-line/0 text-muted hover:border-line hover:bg-surface hover:text-ink',
          ]"
          :aria-current="isActive(item.to) ? 'page' : undefined"
        >
          <component :is="item.icon" class="size-4 shrink-0" aria-hidden="true" />
          <span class="truncate">{{ t(item.labelKey) }}</span>
        </NuxtLink>
      </div>
    </nav>
  </div>
</template>

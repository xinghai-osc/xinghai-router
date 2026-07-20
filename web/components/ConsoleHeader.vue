<script setup lang="ts">
import { PanelLeftOpen, PanelLeftClose, Sparkles, RefreshCw, Languages } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useConsoleStore } from '~/composables/useConsoleStore'

const store = useConsoleStore()
const { t, view, busy, load, account, sidebarCollapsed, locale, generalNav, billingNav, personalNav, managementNav, localizedManagementNavItems, localizedAdminExtraNav } = store
</script>

<template>
  <header class="sticky top-0 z-10 flex h-14 items-center justify-between gap-3 border-b border-border bg-background/80 px-4 backdrop-blur">
    <div class="min-w-0">
      <p class="text-[10px] font-bold uppercase tracking-wider text-muted-foreground">{{ managementNav.some((item) => item[0] === view) ? t('management') : personalNav.some((item) => item[0] === view) ? t('personal') : billingNav.some((item) => item[0] === view) ? t('billing') : t('general') }}</p>
      <h1 class="truncate text-lg font-semibold">{{ [...localizedManagementNavItems, ...generalNav, ...billingNav, ...personalNav, ...localizedAdminExtraNav].find((item) => item[0] === view)?.[1] }}</h1>
    </div>
    <div class="flex items-center gap-2">
      <Button variant="ghost" size="icon-sm" class="hidden lg:inline-flex" :aria-label="sidebarCollapsed ? t('expandSidebar') : t('collapseSidebar')" :title="sidebarCollapsed ? t('expandSidebar') : t('collapseSidebar')" @click="sidebarCollapsed = !sidebarCollapsed"><PanelLeftOpen v-if="sidebarCollapsed" :size="16" /><PanelLeftClose v-else :size="16" /></Button>
      <Button variant="ghost" size="sm" as-child><a href="/models"><Sparkles :size="15" />{{ t('marketplace') }}</a></Button>
      <span class="hidden items-center gap-2 rounded-full border border-border px-3 py-1 text-sm font-medium sm:inline-flex"><i class="flex h-6 w-6 items-center justify-center rounded-full bg-primary text-[10px] font-bold text-primary-foreground">{{ account?.name?.slice(0, 1) || '?' }}</i>{{ account?.name || t('loadingLabel') }}</span>
      <ThemeCustomizer :locale="locale" />
      <Select v-model="locale">
        <SelectTrigger class="h-9 w-auto gap-1.5 border-input px-2.5 text-sm" :aria-label="t('switchLanguage')">
          <Languages :size="15" class="shrink-0 text-muted-foreground" />
          <SelectValue class="w-auto" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="zh-CN">{{ t('chinese') }}</SelectItem>
          <SelectItem value="en-US">{{ t('english') }}</SelectItem>
        </SelectContent>
      </Select>
      <Button variant="ghost" size="sm" :disabled="busy" @click="load"><RefreshCw :size="16" :class="{ 'animate-spin': busy }" />{{ t('refresh') }}</Button>
    </div>
  </header>
</template>
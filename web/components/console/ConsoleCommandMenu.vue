<script setup lang="ts">
import { ArrowUpRight, Command, CornerDownLeft, Search } from 'lucide-vue-next'
import { NAV_SECTIONS } from '~/src/nav'

const props = withDefaults(defineProps<{ can?: (permission: string) => boolean }>(), { can: () => false })

const open = ref(false)
const query = ref('')
const selectedIndex = ref(0)
const input = ref<HTMLInputElement | null>(null)
const trigger = ref<HTMLButtonElement | null>(null)
const listId = useId()
const { t } = useI18n()
const route = useRoute()

const items = computed(() => NAV_SECTIONS.flatMap(section => section.items
  .filter(item => !item.permission || props.can(item.permission))
  .map(item => ({ ...item, section: t(section.titleKey), label: t(item.labelKey) }))))

const filteredItems = computed(() => {
  const needle = query.value.trim().toLocaleLowerCase()
  if (!needle) return items.value
  return items.value.filter(item => `${item.label} ${item.section}`.toLocaleLowerCase().includes(needle))
})

watch(open, value => {
  if (!value) {
    void nextTick(() => trigger.value?.focus())
    return
  }
  query.value = ''
  selectedIndex.value = 0
  void nextTick(() => input.value?.focus())
})

watch(input, value => {
  if (value && open.value) void nextTick(() => value.focus())
})

watch(query, () => { selectedIndex.value = 0 })
watch(() => route.fullPath, () => { open.value = false })

watch(selectedIndex, () => {
  void nextTick(() => document.getElementById(`${listId}-${selectedIndex.value}`)?.scrollIntoView({ block: 'nearest' }))
})

watch(filteredItems, value => {
  selectedIndex.value = Math.min(selectedIndex.value, Math.max(value.length - 1, 0))
})

function isCurrent(to: string) {
  return route.path === to || (to !== '/console' && route.path.startsWith(`${to}/`))
}

function close() {
  open.value = false
}

function go(to: string) {
  close()
  void navigateTo(to)
}

function onInputKeydown(event: KeyboardEvent) {
  if (event.key === 'ArrowDown') {
    event.preventDefault()
    if (filteredItems.value.length) selectedIndex.value = (selectedIndex.value + 1) % filteredItems.value.length
  } else if (event.key === 'ArrowUp') {
    event.preventDefault()
    if (filteredItems.value.length) selectedIndex.value = (selectedIndex.value - 1 + filteredItems.value.length) % filteredItems.value.length
  } else if (event.key === 'Enter') {
    event.preventDefault()
    const item = filteredItems.value[selectedIndex.value]
    if (item) go(item.to)
  }
}

function onGlobalKeydown(event: KeyboardEvent) {
  if (event.defaultPrevented || event.isComposing) return
  const target = event.target as HTMLElement | null
  if (!open.value && target?.closest('[role="dialog"]')) return
  const isTyping = target?.tagName === 'INPUT' || target?.tagName === 'TEXTAREA' || target?.isContentEditable
  if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
    event.preventDefault()
    open.value = !open.value
  } else if (event.key === '/' && !isTyping && !event.metaKey && !event.ctrlKey && !event.altKey) {
    event.preventDefault()
    open.value = true
  }
}

onMounted(() => window.addEventListener('keydown', onGlobalKeydown))
onBeforeUnmount(() => window.removeEventListener('keydown', onGlobalKeydown))
</script>

<template>
  <button
    ref="trigger"
    type="button"
    class="glass-control inline-flex h-10 min-w-10 items-center justify-center gap-2 rounded-full border border-line/60 px-3 text-muted transition-colors duration-150 ease-out hover:bg-sunken hover:text-ink focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-clay sm:min-w-44 sm:justify-start"
    :aria-label="t('console.commandSearch')"
    :title="t('console.commandSearchHint')"
    aria-haspopup="dialog"
    :aria-expanded="open"
    @click="open = true"
  >
    <Search class="size-4 shrink-0" aria-hidden="true" />
    <span class="hidden flex-1 text-left text-xs sm:inline">{{ t('console.commandSearch') }}</span>
    <kbd class="hidden items-center gap-0.5 rounded border border-line bg-sunken px-1.5 py-0.5 font-mono text-2xs text-faint sm:inline-flex">
      <Command class="size-3" aria-hidden="true" />
      K
    </kbd>
  </button>

  <UiDialog v-model:open="open" :title="t('console.commandSearch')" :description="t('console.commandSearchDescription')" size="md">
    <div class="relative">
      <Search class="pointer-events-none absolute top-1/2 left-3.5 size-4 -translate-y-1/2 text-faint" aria-hidden="true" />
      <input
        ref="input"
        v-model="query"
        type="search"
        role="combobox"
        aria-expanded="true"
        aria-autocomplete="list"
        :aria-controls="listId"
        :aria-activedescendant="filteredItems.length ? `${listId}-${selectedIndex}` : undefined"
        class="h-12 w-full rounded-control border border-line bg-sunken/60 px-10 text-sm text-ink outline-none transition-colors duration-150 placeholder:text-faint focus:border-clay"
        :placeholder="t('console.commandSearchPlaceholder')"
        :aria-label="t('console.commandSearchPlaceholder')"
        @keydown="onInputKeydown"
      >
    </div>

    <div :id="listId" class="mt-4 max-h-[50vh] overflow-y-auto" role="listbox" :aria-label="t('console.commandSearch')">
      <div v-if="filteredItems.length" class="space-y-1">
        <button
          v-for="(item, index) in filteredItems"
          :id="`${listId}-${index}`"
          :key="item.to"
          type="button"
          class="group flex w-full items-center gap-3 rounded-control px-3 py-3 text-left transition-colors duration-150 ease-out focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-clay"
          :class="index === selectedIndex ? 'bg-clay-soft text-clay' : 'text-muted hover:bg-sunken hover:text-ink'"
          role="option"
          tabindex="-1"
          :aria-selected="index === selectedIndex"
          @focus="selectedIndex = index"
          @mouseenter="selectedIndex = index"
          @click="go(item.to)"
        >
          <span class="flex size-8 shrink-0 items-center justify-center rounded-control bg-surface text-clay">
            <component :is="item.icon" class="size-4" aria-hidden="true" />
          </span>
          <span class="min-w-0 flex-1">
            <span class="block truncate text-sm font-medium">{{ item.label }}</span>
            <span class="mt-0.5 block truncate text-xs text-faint">{{ item.section }}</span>
          </span>
          <ArrowUpRight v-if="isCurrent(item.to)" class="size-4 shrink-0 text-clay" aria-hidden="true" />
          <CornerDownLeft v-else class="size-4 shrink-0 text-faint opacity-0 transition-opacity duration-150 group-hover:opacity-100 group-focus-visible:opacity-100" aria-hidden="true" />
        </button>
      </div>
      <UiEmptyState v-else :icon="Search" :title="t('console.commandSearchEmpty')" />
    </div>

    <template #footer>
      <div class="flex w-full items-center justify-between gap-3 text-2xs text-faint">
        <span>{{ t('console.commandSearchHint') }}</span>
        <span class="font-mono">↑↓ {{ t('console.commandSearchSelect') }} · Enter</span>
      </div>
    </template>
  </UiDialog>
</template>

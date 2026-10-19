<script setup lang="ts">
import { Menu, X } from 'lucide-vue-next'

const { settings } = useSiteSettings()
const { account, loadAccount } = useAccount()
const { t } = useI18n()
const route = useRoute()
const open = ref(false)
const menuTrigger = ref<HTMLButtonElement | null>(null)
const menuId = useId()

const accountName = computed(() => account.value?.name?.trim() || account.value?.email?.trim() || '')
const accountInitial = computed(() => accountName.value.slice(0, 1).toUpperCase() || '?')

const links = [
  { to: '/models', key: 'nav.models' },
  { to: '/activity', key: 'nav.activity' },
  { to: '/rankings', key: 'nav.rankings' },
  { to: '/pricing', key: 'nav.pricingPublic' },
]

function closeMenu() {
  if (!open.value) return
  open.value = false
  menuTrigger.value?.focus()
}

onMounted(() => { void loadAccount() })
watch(() => route.fullPath, () => { open.value = false })
</script>

<template>
  <div class="site-topbar sticky top-2 z-40 mx-2 mt-2 shrink-0 rounded-[1.5rem] border sm:top-3 sm:mx-3 sm:mt-3 sm:rounded-[1.75rem]" @keydown.esc="closeMenu">
    <div v-if="settings.announcement" class="mx-3 mt-3 rounded-2xl border border-clay/15 bg-clay-soft/60 px-4 py-2 text-center text-xs break-words text-clay">
      {{ settings.announcement }}
    </div>
    <header class="bg-transparent">
      <div class="shell">
        <div class="flex min-h-16 items-center justify-between gap-3 py-2 sm:min-h-20 lg:gap-6">
          <SiteLogo :name="settings.name" :icon-url="settings.icon_url" class="min-w-0" />

          <nav class="hidden items-center gap-1 rounded-full border border-line/50 bg-surface/25 p-1 lg:flex" :aria-label="t('common.menu')">
            <NuxtLink
              v-for="link in links"
              :key="link.to"
              :to="link.to"
              class="rounded-full border border-transparent px-4 py-2 text-[13px] font-medium text-muted transition-colors duration-150 ease-out hover:bg-surface/60 hover:text-ink"
              active-class="!border-line/60 bg-surface/60 !text-ink"
            >{{ t(link.key) }}</NuxtLink>
          </nav>

          <div class="flex shrink-0 items-center gap-1.5">
            <div class="hidden items-center gap-1 rounded-full border border-line/50 bg-surface/25 p-1 lg:flex">
              <SiteLocaleToggle />
              <SiteThemeToggle />
            </div>
            <template v-if="account">
              <NuxtLink
                to="/console"
                class="hidden max-w-44 items-center gap-2 rounded-full border border-line/60 bg-surface/25 py-1.5 pr-3 pl-1.5 text-sm text-ink transition-colors duration-150 ease-out hover:border-line-strong hover:bg-sunken sm:flex"
                :aria-label="t('common.openConsole')"
              >
                <img v-if="account.avatar_url" :src="account.avatar_url" alt="" class="size-7 shrink-0 rounded-full object-cover">
                <span v-else class="flex size-7 shrink-0 items-center justify-center rounded-full bg-clay-soft text-2xs font-semibold text-clay">{{ accountInitial }}</span>
                <span class="truncate">{{ accountName }}</span>
              </NuxtLink>
            </template>
            <template v-else>
              <div class="hidden sm:block">
                <UiButton to="/auth" variant="ghost" size="sm">{{ t('common.signIn') }}</UiButton>
              </div>
              <UiButton to="/auth?mode=register" size="sm">{{ t('common.getStarted') }}</UiButton>
            </template>
            <button
              ref="menuTrigger"
              type="button"
              class="inline-flex size-9 items-center justify-center rounded-full border border-line text-muted transition-colors duration-150 ease-out hover:bg-sunken hover:text-ink lg:hidden"
              :aria-expanded="open"
              :aria-controls="menuId"
              :aria-label="open ? t('common.close') : t('common.menu')"
              @click="open = !open"
            >
              <X v-if="open" class="size-[18px]" aria-hidden="true" />
              <Menu v-else class="size-[18px]" aria-hidden="true" />
            </button>
          </div>
        </div>

        <div v-if="open" :id="menuId" class="animate-fade max-h-[calc(100dvh-8rem)] overflow-y-auto border-t border-line pb-3 lg:hidden">
          <nav class="flex flex-col gap-1 py-3" :aria-label="t('common.menu')">
            <NuxtLink
              v-for="link in links"
              :key="link.to"
              :to="link.to"
              class="rounded-full px-3 py-2.5 text-sm font-medium text-muted transition-colors duration-150 ease-out hover:bg-sunken hover:text-ink"
              active-class="bg-clay-soft !text-clay"
            >{{ t(link.key) }}</NuxtLink>
            <NuxtLink
              v-if="account"
              to="/console"
              class="flex items-center gap-2 rounded-full px-3 py-2.5 text-sm text-muted hover:bg-sunken hover:text-ink sm:hidden"
              :aria-label="t('common.openConsole')"
            >
              <img v-if="account.avatar_url" :src="account.avatar_url" alt="" class="size-7 shrink-0 rounded-full object-cover">
              <span v-else class="flex size-7 shrink-0 items-center justify-center rounded-full bg-clay-soft text-2xs font-semibold text-clay">{{ accountInitial }}</span>
              <span class="truncate">{{ accountName }}</span>
            </NuxtLink>
            <NuxtLink v-else to="/auth" class="rounded-full px-3 py-2.5 text-sm text-muted hover:bg-sunken hover:text-ink sm:hidden">
              {{ t('common.signIn') }}
            </NuxtLink>
          </nav>
          <div class="flex items-center gap-1 border-t border-line pt-3">
            <SiteLocaleToggle />
            <SiteThemeToggle />
          </div>
        </div>
      </div>
    </header>
  </div>
</template>

<style scoped>
.site-topbar {
  isolation: isolate;
  border-color: color-mix(in srgb, var(--line) 65%, transparent);
  background:
    radial-gradient(ellipse at 0% 0%, color-mix(in srgb, var(--clay) 7%, transparent), transparent 65%),
    linear-gradient(125deg, color-mix(in srgb, var(--surface) 55%, transparent), color-mix(in srgb, var(--surface) 20%, transparent));
  -webkit-backdrop-filter: blur(24px) saturate(150%);
  backdrop-filter: blur(24px) saturate(150%);
}

.site-topbar::before {
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

.site-topbar :deep(.ui-button) {
  border-radius: 999px;
}

@supports not ((backdrop-filter: blur(1px)) or (-webkit-backdrop-filter: blur(1px))) {
  .site-topbar {
    background: color-mix(in srgb, var(--surface) 95%, transparent);
  }
}

@media (prefers-reduced-transparency: reduce) {
  .site-topbar {
    background: var(--surface);
    -webkit-backdrop-filter: none;
    backdrop-filter: none;
  }
}
</style>

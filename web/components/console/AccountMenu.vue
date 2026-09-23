<script setup lang="ts">
import { ChevronDown, LogOut, UserCog } from 'lucide-vue-next'
import { formatMoney } from '~/src/format'

const { account, signOut } = useAccount()
const { t } = useI18n()

const initial = computed(() => {
  const source = account.value?.name?.trim() || account.value?.email?.trim() || ''
  return source ? source.slice(0, 1).toUpperCase() : '?'
})
</script>

<template>
  <UiDropdownMenu>
    <template #trigger>
      <button
        type="button"
        class="group flex h-9 min-w-9 shrink-0 items-center justify-center gap-2 rounded-control border border-line/0 px-1 text-[13px] font-medium text-ink transition-colors duration-150 ease-out hover:border-line hover:bg-sunken focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-clay data-[state=open]:border-line data-[state=open]:bg-sunken sm:pr-2"
        :aria-label="t('auth.accountMenu')"
      >
        <img
          v-if="account?.avatar_url"
          :src="account.avatar_url"
          alt=""
          class="size-7 shrink-0 rounded-full border border-line object-cover"
        >
        <span
          v-else
          class="flex size-7 shrink-0 items-center justify-center rounded-full border border-clay/15 bg-clay-soft text-2xs font-semibold text-clay"
          aria-hidden="true"
        >{{ initial }}</span>
        <span class="hidden max-w-28 truncate sm:block">{{ account?.name }}</span>
        <ChevronDown class="hidden size-3.5 shrink-0 text-faint transition-transform duration-150 ease-out group-data-[state=open]:rotate-180 sm:block" aria-hidden="true" />
      </button>
    </template>

    <div class="w-60 max-w-[calc(100vw-2rem)] space-y-1 px-2.5 py-2.5">
      <p class="truncate text-sm font-semibold text-ink">{{ account?.name }}</p>
      <p class="truncate text-xs text-muted">{{ account?.email }}</p>
    </div>

    <UiDropdownItem as="separator" />

    <div class="mx-1 my-2 flex flex-wrap items-center justify-between gap-x-3 gap-y-1 rounded-control border border-line bg-sunken/60 px-2.5 py-2 text-[13px]">
      <span class="text-muted">{{ t('auth.balance') }}</span>
      <span class="numeric font-medium text-ink">{{ formatMoney(account?.balance) }}</span>
    </div>

    <UiDropdownItem as="separator" />

    <UiDropdownItem @select="navigateTo('/console/account')">
      <UserCog class="size-4 shrink-0 text-faint" aria-hidden="true" />
      {{ t('auth.accountSettings') }}
    </UiDropdownItem>

    <UiDropdownItem danger @select="signOut()">
      <LogOut class="size-4 shrink-0" aria-hidden="true" />
      {{ t('common.signOut') }}
    </UiDropdownItem>
  </UiDropdownMenu>
</template>

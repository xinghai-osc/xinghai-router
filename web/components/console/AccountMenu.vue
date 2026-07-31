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
        class="flex h-9 items-center gap-2 rounded-control pr-1.5 pl-1 text-[13px] text-ink transition-colors duration-150 hover:bg-sunken"
        :aria-label="t('auth.accountMenu')"
      >
        <img
          v-if="account?.avatar_url"
          :src="account.avatar_url"
          alt=""
          class="size-7 shrink-0 rounded-full object-cover"
        >
        <span
          v-else
          class="flex size-7 shrink-0 items-center justify-center rounded-full bg-clay-soft text-2xs font-semibold text-clay"
        >{{ initial }}</span>
        <span class="hidden max-w-28 truncate sm:block">{{ account?.name }}</span>
        <ChevronDown class="size-3.5 shrink-0 text-faint" />
      </button>
    </template>

    <div class="w-52 px-2 py-1.5">
      <p class="truncate text-[13px] font-medium text-ink">{{ account?.name }}</p>
      <p class="truncate text-2xs text-muted">{{ account?.email }}</p>
    </div>

    <UiDropdownItem as="separator" />

    <div class="flex items-center justify-between gap-3 px-2 py-1.5 text-[13px]">
      <span class="text-muted">{{ t('auth.balance') }}</span>
      <span class="numeric font-medium text-ink">{{ formatMoney(account?.balance) }}</span>
    </div>

    <UiDropdownItem as="separator" />

    <UiDropdownItem @select="navigateTo('/console/account')">
      <UserCog class="size-4 text-faint" />
      {{ t('auth.accountSettings') }}
    </UiDropdownItem>

    <UiDropdownItem danger @select="signOut()">
      <LogOut class="size-4" />
      {{ t('common.signOut') }}
    </UiDropdownItem>
  </UiDropdownMenu>
</template>

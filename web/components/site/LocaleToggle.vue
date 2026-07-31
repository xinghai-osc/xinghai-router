<script setup lang="ts">
import { Check, Languages } from 'lucide-vue-next'

const { locale, locales, t, setLocale } = useI18n()
</script>

<template>
  <UiDropdownMenu align="end">
    <template #trigger>
      <button
        type="button"
        class="inline-flex size-9 items-center justify-center rounded-control text-muted transition-colors duration-150 hover:bg-sunken hover:text-ink"
        :aria-label="t('common.language')"
      >
        <Languages class="size-[18px]" />
      </button>
    </template>

    <UiDropdownItem as="label">{{ t('common.language') }}</UiDropdownItem>
    <UiDropdownItem v-for="item in locales" :key="item.value" @select="setLocale(item.value)">
      <span class="numeric w-6 shrink-0 text-2xs text-faint">{{ item.short }}</span>
      <span class="min-w-0 flex-1 truncate">{{ item.label }}</span>
      <Check v-if="locale === item.value" class="size-3.5 shrink-0 text-clay" />
    </UiDropdownItem>
  </UiDropdownMenu>
</template>

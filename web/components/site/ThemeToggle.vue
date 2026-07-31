<script setup lang="ts">
import { Check, Moon, Palette, Sun } from 'lucide-vue-next'

const { mode, preset, presets, toggleMode, setPreset } = useTheme()
const { t } = useI18n()
</script>

<template>
  <div class="flex items-center gap-0.5">
    <UiDropdownMenu align="end">
      <template #trigger>
        <button
          type="button"
          class="inline-flex size-9 items-center justify-center rounded-control text-muted transition-colors duration-150 hover:bg-sunken hover:text-ink"
          :aria-label="t('common.theme')"
        >
          <Palette class="size-[18px]" />
        </button>
      </template>

      <UiDropdownItem as="label">{{ t('common.theme') }}</UiDropdownItem>
      <UiDropdownItem
        v-for="item in presets"
        :key="item.value"
        @select="setPreset(item.value)"
      >
        <span class="flex shrink-0 items-center -space-x-1">
          <span
            v-for="(color, index) in item.swatch"
            :key="index"
            class="size-3 rounded-full border border-line"
            :style="{ backgroundColor: color }"
          />
        </span>
        <span class="min-w-0 flex-1">
          <span class="block truncate">{{ t(`theme.${item.value}Label`) }}</span>
          <span class="block truncate text-2xs text-faint">{{ t(`theme.${item.value}Hint`) }}</span>
        </span>
        <Check v-if="preset === item.value" class="size-3.5 shrink-0 text-clay" />
      </UiDropdownItem>
    </UiDropdownMenu>

    <button
      type="button"
      class="inline-flex size-9 items-center justify-center rounded-control text-muted transition-colors duration-150 hover:bg-sunken hover:text-ink"
      :aria-label="mode === 'dark' ? t('common.lightMode') : t('common.darkMode')"
      @click="toggleMode"
    >
      <ClientOnly>
        <Moon v-if="mode === 'dark'" class="size-[18px]" />
        <Sun v-else class="size-[18px]" />
        <template #fallback>
          <Sun class="size-[18px]" />
        </template>
      </ClientOnly>
    </button>
  </div>
</template>

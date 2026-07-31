<script setup lang="ts">
import { Check, Moon, Sun } from 'lucide-vue-next'

const { mode, preset, presets, setMode, setPreset } = useTheme()
const { t } = useI18n()
</script>

<template>
  <div class="space-y-4">
    <div class="grid gap-3 sm:grid-cols-3">
      <button
        v-for="item in presets"
        :key="item.value"
        type="button"
        :class="[
          'flex items-center gap-3 rounded-card border p-4 text-left transition-colors duration-150',
          preset === item.value ? 'border-clay bg-clay-soft' : 'border-line bg-surface hover:border-line-strong',
        ]"
        @click="setPreset(item.value)"
      >
        <span class="flex shrink-0 items-center -space-x-1.5">
          <span
            v-for="(color, index) in item.swatch"
            :key="index"
            class="size-6 rounded-full border border-line"
            :style="{ backgroundColor: color }"
          />
        </span>
        <span class="min-w-0 flex-1">
          <span class="block truncate text-sm font-medium text-ink">{{ t(`theme.${item.value}Label`) }}</span>
          <span class="block truncate text-2xs text-muted">{{ t(`theme.${item.value}Hint`) }}</span>
        </span>
        <Check v-if="preset === item.value" class="size-4 shrink-0 text-clay" />
      </button>
    </div>

    <div class="flex items-center gap-2">
      <UiButton
        :variant="mode === 'light' ? 'primary' : 'secondary'"
        size="sm"
        @click="setMode('light')"
      >
        <Sun class="size-4" />
        Light
      </UiButton>
      <UiButton
        :variant="mode === 'dark' ? 'primary' : 'secondary'"
        size="sm"
        @click="setMode('dark')"
      >
        <Moon class="size-4" />
        Dark
      </UiButton>
    </div>
  </div>
</template>

<script setup lang="ts">
import { Check, Monitor, Moon, Paintbrush, RotateCcw, Sun } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import type { Locale } from '~/composables/useI18n'
import type { ThemeColor, ThemeMode, ThemeRadius } from '~/composables/useTheme'

const props = defineProps<{ locale: Locale }>()
const { mode, color, radius, preset, setMode, setColor, setRadius, setPreset, resetTheme } = useTheme()

const modes: { value: ThemeMode; zh: string; en: string; icon: typeof Sun }[] = [
  { value: 'light', zh: '浅色', en: 'Light', icon: Sun },
  { value: 'dark', zh: '深色', en: 'Dark', icon: Moon },
  { value: 'system', zh: '跟随系统', en: 'System', icon: Monitor },
]
const colors: { value: ThemeColor; zh: string; en: string; swatch: string }[] = [
  { value: 'neutral', zh: '中性', en: 'Neutral', swatch: 'bg-zinc-900' },
  { value: 'blue', zh: '蓝色', en: 'Blue', swatch: 'bg-blue-600' },
  { value: 'green', zh: '绿色', en: 'Green', swatch: 'bg-green-600' },
  { value: 'orange', zh: '橙色', en: 'Orange', swatch: 'bg-orange-500' },
  { value: 'rose', zh: '玫红', en: 'Rose', swatch: 'bg-rose-500' },
  { value: 'violet', zh: '紫色', en: 'Violet', swatch: 'bg-violet-500' },
]
const radii: { value: ThemeRadius; zh: string; en: string }[] = [
  { value: 'none', zh: '无', en: 'None' },
  { value: 'small', zh: '小', en: 'Small' },
  { value: 'medium', zh: '中', en: 'Medium' },
  { value: 'large', zh: '大', en: 'Large' },
]
const label = (item: { zh: string; en: string }) => props.locale === 'zh-CN' ? item.zh : item.en
const isZh = computed(() => props.locale === 'zh-CN')
</script>

<template>
  <DropdownMenu>
    <DropdownMenuTrigger as-child>
      <Button variant="ghost" size="icon" :aria-label="isZh ? '配置主题' : 'Customize theme'" :title="isZh ? '配置主题' : 'Customize theme'">
        <Paintbrush />
      </Button>
    </DropdownMenuTrigger>
    <DropdownMenuContent align="end" :side-offset="8" class="w-64">
      <DropdownMenuLabel>{{ isZh ? '自定义主题' : 'Customize theme' }}</DropdownMenuLabel>
      <DropdownMenuSeparator />

      <div class="px-2 py-1 text-xs text-muted-foreground">{{ isZh ? '预设' : 'Preset' }}</div>
      <DropdownMenuItem :class="preset === 'a-site' ? 'bg-accent' : ''" @select="setPreset('a-site')">
        <span class="flex h-7 w-7 items-center justify-center rounded-md bg-orange-500 text-white text-[10px] font-bold">A</span>
        <span class="flex flex-1 flex-col">
          <span class="text-sm font-medium">Claude 官网</span>
          <span class="text-xs text-muted-foreground">{{ isZh ? 'Anthropic 奶油白与 Claude 橙' : 'Anthropic cream with Claude orange' }}</span>
        </span>
        <Check v-if="preset === 'a-site'" class="ml-auto" />
      </DropdownMenuItem>
      <DropdownMenuSeparator />

      <div class="px-2 py-1 text-xs text-muted-foreground">{{ isZh ? '模式' : 'Mode' }}</div>
      <DropdownMenuRadioGroup :model-value="mode" @update:model-value="(v) => setMode(v as ThemeMode)">
        <DropdownMenuRadioItem v-for="item in modes" :key="item.value" :value="item.value">
          <component :is="item.icon" class="mr-2" />
          {{ label(item) }}
        </DropdownMenuRadioItem>
      </DropdownMenuRadioGroup>
      <DropdownMenuSeparator />

      <div class="px-2 py-1 text-xs text-muted-foreground">{{ isZh ? '主色' : 'Color' }}</div>
      <DropdownMenuRadioGroup :model-value="color" @update:model-value="(v) => setColor(v as ThemeColor)">
        <DropdownMenuRadioItem v-for="item in colors" :key="item.value" :value="item.value">
          <span class="mr-2 inline-flex h-3 w-3 rounded-full" :class="item.swatch" />
          {{ label(item) }}
        </DropdownMenuRadioItem>
      </DropdownMenuRadioGroup>
      <DropdownMenuSeparator />

      <div class="px-2 py-1 text-xs text-muted-foreground">{{ isZh ? '圆角' : 'Radius' }}</div>
      <DropdownMenuRadioGroup :model-value="radius" @update:model-value="(v) => setRadius(v as ThemeRadius)">
        <DropdownMenuRadioItem v-for="item in radii" :key="item.value" :value="item.value">
          {{ label(item) }}
        </DropdownMenuRadioItem>
      </DropdownMenuRadioGroup>
      <DropdownMenuSeparator />

      <DropdownMenuItem @select="resetTheme">
        <RotateCcw />
        {{ isZh ? '恢复默认' : 'Reset theme' }}
      </DropdownMenuItem>
    </DropdownMenuContent>
  </DropdownMenu>
</template>

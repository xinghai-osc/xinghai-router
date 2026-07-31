<script setup lang="ts">
import { Check, ChevronDown } from 'lucide-vue-next'
import {
  SelectContent, SelectIcon, SelectItem, SelectItemIndicator, SelectItemText,
  SelectPortal, SelectRoot, SelectTrigger, SelectValue, SelectViewport,
} from 'reka-ui'
import { cn } from '~/lib/utils'

export interface SelectOption { value: string; label: string; disabled?: boolean }

const model = defineModel<string>({ default: '' })

const props = withDefaults(defineProps<{
  options: SelectOption[]
  placeholder?: string
  disabled?: boolean
  size?: 'sm' | 'md'
  id?: string
}>(), { size: 'md' })

// Resolved here rather than as a prop default: prop defaults are evaluated
// before the component has a Nuxt context, so t() would not be available.
const { t } = useI18n()
const placeholderText = computed(() => props.placeholder ?? t('common.selectPlaceholder'))
const EMPTY_OPTION_VALUE = '__ui-select-empty-option__'
const hasEmptyOption = computed(() => props.options.some(option => option.value === ''))
const normalizedOptions = computed(() => props.options.map(option => ({
  ...option,
  value: option.value === '' ? EMPTY_OPTION_VALUE : option.value,
})))
const selection = computed({
  get: () => model.value === '' && hasEmptyOption.value ? EMPTY_OPTION_VALUE : model.value,
  set: value => { model.value = value === EMPTY_OPTION_VALUE ? '' : value },
})
</script>

<template>
  <SelectRoot v-model="selection" :disabled="disabled">
    <SelectTrigger
      :id="id"
      :class="cn(
        'inline-flex w-full items-center justify-between gap-2 rounded-control border border-line-strong bg-surface px-3 text-sm text-ink',
        'transition-colors duration-150 hover:border-faint',
        'focus:border-clay focus:outline-none focus:ring-2 focus:ring-clay/20',
        'disabled:cursor-not-allowed disabled:bg-sunken disabled:text-muted',
        'data-[placeholder]:text-faint',
        size === 'sm' ? 'h-8 text-[13px]' : 'h-10',
      )"
    >
      <SelectValue :placeholder="placeholderText" class="truncate" />
      <SelectIcon as-child>
        <ChevronDown class="size-4 shrink-0 text-faint" />
      </SelectIcon>
    </SelectTrigger>

    <SelectPortal>
      <SelectContent
        position="popper"
        :side-offset="6"
        class="animate-pop z-50 max-h-72 min-w-[var(--reka-select-trigger-width)] overflow-hidden rounded-control border border-line bg-surface shadow-pop"
      >
        <SelectViewport class="p-1">
          <SelectItem
            v-for="option in normalizedOptions"
            :key="option.value"
            :value="option.value"
            :disabled="option.disabled"
            class="relative flex cursor-pointer items-center gap-2 rounded-[7px] py-1.5 pr-2 pl-8 text-sm text-ink select-none data-[disabled]:pointer-events-none data-[disabled]:opacity-45 data-[highlighted]:bg-sunken data-[highlighted]:outline-none"
          >
            <SelectItemIndicator class="absolute left-2 flex">
              <Check class="size-3.5 text-clay" />
            </SelectItemIndicator>
            <SelectItemText>{{ option.label }}</SelectItemText>
          </SelectItem>
        </SelectViewport>
      </SelectContent>
    </SelectPortal>
  </SelectRoot>
</template>

<script setup lang="ts">
import { computed, inject } from 'vue'
import { Check, ChevronDown } from 'lucide-vue-next'
import {
  SelectContent, SelectIcon, SelectItem, SelectItemIndicator, SelectItemText,
  SelectPortal, SelectRoot, SelectTrigger, SelectValue, SelectViewport,
} from 'reka-ui'
import { cn } from '~/lib/utils'
import { uiFieldContextKey } from './field-context'

export interface SelectOption { value: string; label: string; disabled?: boolean }

const model = defineModel<string>({ default: '' })

const props = withDefaults(defineProps<{
  options: SelectOption[]
  placeholder?: string
  disabled?: boolean
  size?: 'sm' | 'md'
  id?: string
}>(), { size: 'md' })

const field = inject(uiFieldContextKey, null)
const selectId = computed(() => props.id ?? field?.id.value)
const describedBy = computed(() => field?.describedBy.value)
const invalid = computed(() => Boolean(field?.invalid.value))

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
      :id="selectId"
      :aria-describedby="describedBy"
      :aria-invalid="invalid || undefined"
      :aria-required="field?.required.value || undefined"
      :class="cn(
        'inline-flex min-w-0 w-full items-center justify-between gap-2 rounded-control border border-line-strong bg-surface px-3 text-left text-sm text-ink',
        'transition-colors duration-150 ease-out enabled:hover:border-faint motion-reduce:transition-none',
        'focus-visible:border-clay focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-clay/60',
        'disabled:cursor-not-allowed disabled:bg-sunken disabled:text-muted',
        'aria-invalid:border-danger aria-invalid:focus-visible:outline-danger/60',
        'data-[state=open]:border-clay data-[placeholder]:text-faint',
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
        class="animate-pop z-50 max-h-[min(18rem,var(--reka-select-content-available-height))] min-w-[var(--reka-select-trigger-width)] max-w-[calc(100vw-2rem)] overflow-hidden rounded-control border border-line bg-surface shadow-pop motion-reduce:animate-none"
      >
        <SelectViewport class="p-1">
          <SelectItem
            v-for="option in normalizedOptions"
            :key="option.value"
            :value="option.value"
            :disabled="option.disabled"
            class="relative flex min-h-9 cursor-pointer items-center gap-2 rounded-control py-2 pr-3 pl-8 text-sm leading-5 text-ink transition-colors duration-150 ease-out select-none data-[disabled]:pointer-events-none data-[disabled]:opacity-45 data-[highlighted]:bg-sunken data-[highlighted]:outline-none data-[state=checked]:text-clay motion-reduce:transition-none"
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

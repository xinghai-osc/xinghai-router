<script setup lang="ts">
import { computed, inject, useAttrs } from 'vue'
import { cn } from '~/lib/utils'
import { uiFieldContextKey } from './field-context'

const model = defineModel<string | number>({ default: '' })

const props = withDefaults(defineProps<{
  type?: string
  placeholder?: string
  disabled?: boolean
  readonly?: boolean
  invalid?: boolean
  mono?: boolean
  autocomplete?: string
  id?: string
}>(), { type: 'text' })

defineOptions({ inheritAttrs: false })

const attrs = useAttrs()
const field = inject(uiFieldContextKey, null)
const inputId = computed(() => props.id ?? field?.id.value)
const describedBy = computed(() => [attrs['aria-describedby'], field?.describedBy.value].filter(value => typeof value === 'string' && value).join(' ') || undefined)
const invalid = computed(() => props.invalid || field?.invalid.value || attrs['aria-invalid'] === true || attrs['aria-invalid'] === 'true')
const required = computed(() => Boolean(field?.required.value || attrs.required))
const inputAttrs = computed(() => {
  const { class: _class, style: _style, ...nativeAttrs } = attrs
  return nativeAttrs
})
const rootClass = computed(() => cn('relative flex min-w-0 items-center', typeof attrs.class === 'string' ? attrs.class : undefined))
</script>

<template>
  <div :class="rootClass" :style="attrs.style">
    <span v-if="$slots.leading" class="pointer-events-none absolute left-3 flex text-faint">
      <slot name="leading" />
    </span>

    <input
      v-bind="inputAttrs"
      :id="inputId"
      v-model="model"
      :type="type"
      :placeholder="placeholder"
      :disabled="disabled"
      :readonly="readonly"
      :autocomplete="autocomplete"
      :aria-describedby="describedBy"
      :aria-invalid="invalid || undefined"
      :aria-required="required || undefined"
      :required="required || undefined"
      :class="cn(
        'h-10 min-w-0 w-full rounded-control border border-line-strong bg-surface px-3 text-sm leading-6 text-ink',
        'placeholder:text-faint transition-colors duration-150 ease-out motion-reduce:transition-none',
        'enabled:hover:border-faint focus:border-clay focus:outline-2 focus:outline-offset-2 focus:outline-clay/60',
        'disabled:cursor-not-allowed disabled:bg-sunken disabled:text-muted',
        'aria-invalid:border-danger aria-invalid:focus:border-danger aria-invalid:focus:outline-danger/60',
        mono && 'font-mono text-[13px]',
        $slots.leading && 'pl-9',
        $slots.trailing && 'pr-9',
      )"
    >

    <span v-if="$slots.trailing" class="absolute right-3 flex text-faint">
      <slot name="trailing" />
    </span>
  </div>
</template>

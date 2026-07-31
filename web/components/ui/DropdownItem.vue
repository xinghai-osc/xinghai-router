<script setup lang="ts">
import { DropdownMenuItem, DropdownMenuLabel, DropdownMenuSeparator } from 'reka-ui'
import { cn } from '~/lib/utils'

withDefaults(defineProps<{
  as?: 'item' | 'label' | 'separator'
  danger?: boolean
  disabled?: boolean
}>(), { as: 'item' })

defineEmits<{ select: [event: Event] }>()
</script>

<template>
  <DropdownMenuSeparator v-if="as === 'separator'" class="-mx-1 my-1 h-px bg-line" />

  <DropdownMenuLabel v-else-if="as === 'label'" class="px-2 py-1.5 text-2xs font-medium text-faint">
    <slot />
  </DropdownMenuLabel>

  <DropdownMenuItem
    v-else
    :disabled="disabled"
    :class="cn(
      'flex cursor-pointer items-center gap-2 rounded-[7px] px-2 py-1.5 text-sm select-none',
      'data-[disabled]:pointer-events-none data-[disabled]:opacity-45 data-[highlighted]:outline-none',
      danger
        ? 'text-danger data-[highlighted]:bg-danger-soft'
        : 'text-ink data-[highlighted]:bg-sunken',
    )"
    @select="$emit('select', $event)"
  >
    <slot />
  </DropdownMenuItem>
</template>

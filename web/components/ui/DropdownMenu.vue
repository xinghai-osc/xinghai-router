<script setup lang="ts">
import { DropdownMenuContent, DropdownMenuPortal, DropdownMenuRoot, DropdownMenuTrigger } from 'reka-ui'

withDefaults(defineProps<{
  align?: 'start' | 'center' | 'end'
  side?: 'top' | 'right' | 'bottom' | 'left'
}>(), { align: 'end', side: 'bottom' })
</script>

<template>
  <DropdownMenuRoot>
    <DropdownMenuTrigger as-child>
      <slot name="trigger" />
    </DropdownMenuTrigger>

    <DropdownMenuPortal>
      <DropdownMenuContent
        :align="align"
        :side="side"
        :side-offset="6"
        class="glass-control animate-pop z-50 max-h-[var(--reka-dropdown-menu-content-available-height)] min-w-44 max-w-[calc(100vw-2rem)] overflow-y-auto overscroll-contain rounded-2xl border border-line p-1 shadow-pop [--glass-opacity:90%] motion-reduce:animate-none"
      >
        <slot />
      </DropdownMenuContent>
    </DropdownMenuPortal>
  </DropdownMenuRoot>
</template>

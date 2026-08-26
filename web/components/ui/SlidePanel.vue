<script setup lang="ts">
import { X } from 'lucide-vue-next'
import {
  DialogClose, DialogContent, DialogDescription, DialogOverlay,
  DialogPortal, DialogRoot, DialogTitle,
} from 'reka-ui'
import { cn } from '~/lib/utils'

const open = defineModel<boolean>('open', { default: false })

withDefaults(defineProps<{
  title?: string
  description?: string
  size?: 'sm' | 'md' | 'lg'
}>(), { size: 'md' })

const WIDTHS = { sm: 'max-w-md', md: 'max-w-xl', lg: 'max-w-2xl' }

const { t } = useI18n()
</script>

<template>
  <DialogRoot v-model:open="open">
    <DialogPortal>
      <DialogOverlay class="animate-fade fixed inset-0 z-50 bg-[var(--overlay)] backdrop-blur-sm" />

      <DialogContent
        :class="cn(
          'animate-slide-in fixed inset-y-0 right-0 z-50 flex max-h-screen w-full flex-col',
          'border-l border-line bg-surface/95 shadow-pop backdrop-blur-xl focus:outline-none',
          WIDTHS[size],
        )"
      >
        <header class="flex items-start justify-between gap-4 border-b border-line px-5 py-4">
          <div class="min-w-0">
            <DialogTitle class="text-[15px] font-semibold text-ink">
              <slot name="title">{{ title }}</slot>
            </DialogTitle>
            <DialogDescription v-if="description" class="mt-0.5 text-[13px] text-muted">
              {{ description }}
            </DialogDescription>
          </div>
          <DialogClose
            class="-mt-1 -mr-1 rounded-control p-1.5 text-faint transition-colors hover:bg-sunken hover:text-ink focus-visible:outline-2 focus-visible:outline-clay"
            :aria-label="t('common.close')"
          >
            <X class="size-4" />
          </DialogClose>
        </header>

        <div class="min-h-0 flex-1 overflow-y-auto px-5 py-4">
          <slot />
        </div>

        <footer v-if="$slots.footer" class="flex items-center justify-end gap-2 border-t border-line px-5 py-3">
          <slot name="footer" />
        </footer>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>

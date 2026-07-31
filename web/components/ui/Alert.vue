<script setup lang="ts">
import { AlertTriangle, CheckCircle2, Info, X, XCircle } from 'lucide-vue-next'
import { cn } from '~/lib/utils'

type Tone = 'info' | 'success' | 'warn' | 'danger'

withDefaults(defineProps<{
  tone?: Tone
  title?: string
  dismissible?: boolean
}>(), { tone: 'info' })

defineEmits<{ dismiss: [] }>()

const TONES: Record<Tone, string> = {
  info: 'border-line bg-sunken text-ink',
  success: 'border-success/25 bg-success-soft text-success',
  warn: 'border-warn/25 bg-warn-soft text-warn',
  danger: 'border-danger/25 bg-danger-soft text-danger',
}

const ICONS = { info: Info, success: CheckCircle2, warn: AlertTriangle, danger: XCircle }

const { t } = useI18n()
</script>

<template>
  <div
    role="alert"
    :class="cn('animate-fade flex items-start gap-2.5 rounded-control border px-3.5 py-2.5 text-[13px]', TONES[tone])"
  >
    <component :is="ICONS[tone]" class="mt-0.5 size-4 shrink-0" />
    <div class="min-w-0 flex-1">
      <p v-if="title" class="font-medium">{{ title }}</p>
      <div :class="title && 'mt-0.5 opacity-90'">
        <slot />
      </div>
    </div>
    <button
      v-if="dismissible"
      type="button"
      class="-mt-0.5 -mr-1 shrink-0 rounded p-1 opacity-60 transition-opacity hover:opacity-100"
      :aria-label="t('common.close')"
      @click="$emit('dismiss')"
    >
      <X class="size-3.5" />
    </button>
  </div>
</template>

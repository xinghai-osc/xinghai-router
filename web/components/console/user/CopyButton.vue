<script setup lang="ts">
import { Check, Copy } from 'lucide-vue-next'

const props = withDefaults(defineProps<{
  value: string
  label?: string
  successMessage?: string
  size?: 'sm' | 'md' | 'icon'
  variant?: 'primary' | 'secondary' | 'ghost'
}>(), { size: 'sm', variant: 'secondary' })

const { t } = useI18n()
const { toast } = useToast()

const caption = computed(() => props.label ?? t('common.copy'))
const copied = ref(false)
let timer: ReturnType<typeof setTimeout> | undefined

async function copyText(value: string) {
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(value)
    return
  }

  const textarea = document.createElement('textarea')
  textarea.value = value
  textarea.setAttribute('readonly', '')
  textarea.style.position = 'fixed'
  textarea.style.top = '0'
  textarea.style.left = '-9999px'
  textarea.style.opacity = '0'
  document.body.appendChild(textarea)
  textarea.focus()
  textarea.select()
  textarea.setSelectionRange(0, textarea.value.length)
  const copiedByFallback = document.execCommand('copy')
  textarea.remove()
  if (!copiedByFallback) throw new Error('Clipboard copy failed')
}

async function handleCopy() {
  try {
    await copyText(props.value)
  } catch {
    toast.error(t('common.actionFailed'))
    return
  }

  copied.value = true
  if (props.successMessage) toast.success(props.successMessage)
  clearTimeout(timer)
  timer = setTimeout(() => { copied.value = false }, 1600)
}

onBeforeUnmount(() => clearTimeout(timer))
</script>

<template>
  <UiButton :variant="variant" :size="size" :aria-label="caption" @click="handleCopy">
    <component :is="copied ? Check : Copy" class="size-3.5" />
    <span v-if="size !== 'icon'">{{ copied ? t('common.copied') : caption }}</span>
  </UiButton>
</template>

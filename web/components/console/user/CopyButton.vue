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

const copied = ref(false)
let timer: ReturnType<typeof setTimeout> | undefined

const caption = computed(() => props.label ?? t('common.copy'))

async function copy() {
  try {
    await navigator.clipboard.writeText(props.value)
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
  <UiButton :variant="variant" :size="size" :aria-label="caption" @click="copy">
    <component :is="copied ? Check : Copy" class="size-3.5" />
    <span v-if="size !== 'icon'">{{ copied ? t('common.copied') : caption }}</span>
  </UiButton>
</template>

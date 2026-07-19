<script setup lang="ts">
import { computed, ref } from 'vue'
import { Check, ChevronRight, Copy } from 'lucide-vue-next'
import { effectivePrice, formatSquarePrice, getDisplayGroup, vendorIconUrl, type SquareModel, type TokenUnit } from '~/src/marketplace'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'

const props = defineProps<{
  model: SquareModel
  tokenUnit: TokenUnit
  selectedGroup: string
}>()
const emit = defineEmits<{ open: [model: SquareModel] }>()
const { t } = useI18n()

const iconError = ref(false)
const copied = ref(false)
const unitLabel = computed(() => (props.tokenUnit === 'K' ? '1K' : '1M'))
const primaryGroup = computed(() => getDisplayGroup(props.model, props.selectedGroup))

const inputPrice = computed(() => formatSquarePrice(effectivePrice(props.model, 'input', props.selectedGroup), props.tokenUnit))
const outputPrice = computed(() => formatSquarePrice(effectivePrice(props.model, 'output', props.selectedGroup), props.tokenUnit))
const cachePrice = computed(() => formatSquarePrice(effectivePrice(props.model, 'cache', props.selectedGroup), props.tokenUnit))

const hiddenCount = computed(() => Math.max(props.model.groups.length - 1, 0))

async function copyName(event: MouseEvent) {
  event.stopPropagation()
  try {
    await navigator.clipboard.writeText(props.model.model)
  } catch {
    const textarea = document.createElement('textarea')
    textarea.value = props.model.model
    document.body.append(textarea)
    textarea.select()
    document.execCommand('copy')
    textarea.remove()
  }
  copied.value = true
  window.setTimeout(() => { copied.value = false }, 1500)
}
</script>

<template>
  <div class="flex flex-col gap-3 rounded-lg border border-border bg-card p-4 transition-colors hover:bg-accent/30">
    <div class="flex items-start gap-3">
      <div class="flex flex-1 items-start gap-3">
        <span class="flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-lg bg-muted">
          <img v-if="props.model.vendor_slug && !iconError" :src="vendorIconUrl(props.model.vendor_slug)" :alt="props.model.vendor_name" loading="lazy" class="h-full w-full object-contain" @error="iconError = true">
          <span v-else class="text-base font-bold">{{ props.model.model.slice(0, 1).toUpperCase() }}</span>
        </span>
        <div class="min-w-0 flex-1">
          <h3 class="truncate font-semibold">{{ props.model.model }}</h3>
          <div class="mt-1 flex flex-wrap gap-x-3 gap-y-0.5 text-xs">
            <span class="flex items-center gap-1 text-muted-foreground">
              {{ t('inputLabel') }}
              <b v-if="inputPrice" class="font-mono font-medium text-foreground">{{ inputPrice }}</b>
              <b v-else class="text-muted-foreground">{{ t('pendingConfig') }}</b>
            </span>
            <span class="flex items-center gap-1 text-muted-foreground">
              {{ t('outputLabel') }}
              <b v-if="outputPrice" class="font-mono font-medium text-foreground">{{ outputPrice }}</b>
              <b v-else class="text-muted-foreground">{{ t('pendingConfig') }}</b>
            </span>
            <span v-if="cachePrice" class="flex items-center gap-1 text-muted-foreground">
              {{ t('msCached') }}
              <b class="font-mono font-medium text-foreground">{{ cachePrice }}</b>
            </span>
          </div>
        </div>
      </div>
      <div class="flex shrink-0 items-center gap-1">
        <Button variant="outline" size="sm" @click="emit('open', props.model)">
          {{ t('msDetails') }}<ChevronRight :size="13" />
        </Button>
        <Button variant="ghost" size="icon-sm" :title="copied ? t('msCopied') : t('msCopy')" @click="copyName">
          <Check v-if="copied" :size="13" />
          <Copy v-else :size="13" />
        </Button>
      </div>
    </div>

    <p class="text-sm text-muted-foreground">{{ props.model.vendor_name || t('msNoDesc') }}</p>

    <div class="flex items-center justify-between gap-2 border-t border-border pt-3">
      <div class="flex items-center gap-2">
        <span v-if="primaryGroup" class="text-xs font-medium">{{ primaryGroup.name }}</span>
        <Badge variant="secondary" class="text-[10px]">{{ t('msTokenBased') }}</Badge>
      </div>
      <div class="flex items-center gap-2">
        <span class="text-xs text-muted-foreground">{{ unitLabel }}</span>
        <Badge v-if="hiddenCount > 0" variant="outline" class="text-[10px]">+{{ hiddenCount }}</Badge>
      </div>
    </div>
  </div>
</template>

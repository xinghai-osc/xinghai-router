<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { Search, X } from 'lucide-vue-next'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'

const props = defineProps<{ modelValue: string; placeholder?: string }>()
const emit = defineEmits<{ 'update:modelValue': [value: string]; clear: [] }>()
const { t } = useI18n()
const inputRef = ref<HTMLInputElement | null>(null)

function handleKeydown(event: KeyboardEvent) {
  if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
    event.preventDefault()
    inputRef.value?.focus()
  }
  if (event.key === 'Escape' && document.activeElement === inputRef.value) inputRef.value?.blur()
}

onMounted(() => document.addEventListener('keydown', handleKeydown))
onBeforeUnmount(() => document.removeEventListener('keydown', handleKeydown))
</script>

<template>
  <div class="relative flex w-full max-w-md items-center">
    <Search :size="16" class="pointer-events-none absolute left-3 text-muted-foreground" />
    <Input
      ref="inputRef"
      type="text"
      :value="props.modelValue"
      :placeholder="props.placeholder || t('msSearchPlaceholder')"
      :aria-label="t('msSearchPlaceholder')"
      class="pl-9 pr-16"
      @input="emit('update:modelValue', ($event.target as HTMLInputElement).value)"
    />
    <div class="absolute right-1.5 flex items-center">
      <Button v-if="props.modelValue" variant="ghost" size="icon-sm" class="h-7 w-7" :aria-label="t('msClearSearch')" @click="emit('clear')">
        <X :size="15" />
      </Button>
      <kbd v-else class="rounded border border-border bg-muted px-1.5 py-0.5 font-mono text-[10px] text-muted-foreground">⌘K</kbd>
    </div>
  </div>
</template>

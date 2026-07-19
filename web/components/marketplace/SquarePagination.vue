<script setup lang="ts">
import { ChevronLeft, ChevronRight } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'

const props = defineProps<{ page: number; totalPages: number }>()
const emit = defineEmits<{ 'update:page': [value: number] }>()
const { t } = useI18n()
</script>

<template>
  <div v-if="props.totalPages > 1" class="flex items-center justify-between gap-3 py-4">
    <p class="text-xs text-muted-foreground">{{ t('msPageA') }} {{ props.page }} {{ t('msPageB') }} {{ props.totalPages }} {{ t('msPageC') }}</p>
    <div class="flex gap-2">
      <Button variant="outline" size="sm" :disabled="props.page <= 1" @click="emit('update:page', props.page - 1)">
        <ChevronLeft :size="15" />{{ t('msPrev') }}
      </Button>
      <Button variant="outline" size="sm" :disabled="props.page >= props.totalPages" @click="emit('update:page', props.page + 1)">
        {{ t('msNext') }}<ChevronRight :size="15" />
      </Button>
    </div>
  </div>
</template>

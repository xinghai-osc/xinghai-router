<script setup lang="ts">
import { Search as SearchIcon } from 'lucide-vue-next'

const model = defineModel<string>({ default: '' })

defineProps<{ placeholder?: string }>()
</script>

<template>
  <div class="w-full sm:max-w-xs">
    <UiInput v-model="model" type="search" :placeholder="placeholder">
      <template #leading>
        <SearchIcon class="size-4" />
      </template>
    </UiInput>
  </div>
</template>

<script setup lang="ts">
import { TabsIndicator, TabsList, TabsRoot, TabsTrigger } from 'reka-ui'

export interface TabItem { value: string; label: string; count?: number }

const model = defineModel<string>({ required: true })

defineProps<{ items: TabItem[] }>()
</script>

<template>
  <TabsRoot v-model="model">
    <TabsList class="relative flex items-center gap-1 border-b border-line">
      <TabsTrigger
        v-for="item in items"
        :key="item.value"
        :value="item.value"
        class="relative -mb-px flex items-center gap-1.5 border-b-2 border-transparent px-3 py-2 text-sm text-muted transition-colors duration-150 hover:text-ink focus-visible:outline-none data-[state=active]:border-clay data-[state=active]:text-ink"
      >
        {{ item.label }}
        <span
          v-if="item.count !== undefined"
          class="numeric rounded-full bg-sunken px-1.5 text-2xs text-muted"
        >{{ item.count }}</span>
      </TabsTrigger>
      <TabsIndicator class="hidden" />
    </TabsList>

    <slot />
  </TabsRoot>
</template>

<script setup lang="ts">
import { TabsIndicator, TabsList, TabsRoot, TabsTrigger } from 'reka-ui'

export interface TabItem { value: string; label: string; count?: number }

const model = defineModel<string>({ required: true })

defineProps<{ items: TabItem[] }>()
</script>

<template>
  <TabsRoot v-model="model">
    <TabsList class="relative flex max-w-full items-center gap-1 overflow-x-auto rounded-control border border-line bg-sunken/60 p-1">
      <TabsTrigger
        v-for="item in items"
        :key="item.value"
        :value="item.value"
        class="relative flex shrink-0 items-center gap-1.5 rounded-control border border-transparent px-3 py-1.5 text-sm whitespace-nowrap text-muted transition-colors duration-150 ease-out hover:text-ink data-[state=active]:border-line data-[state=active]:bg-surface data-[state=active]:font-medium data-[state=active]:text-clay"
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

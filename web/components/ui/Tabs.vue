<script setup lang="ts">
import { TabsIndicator, TabsList, TabsRoot, TabsTrigger } from 'reka-ui'

export interface TabItem { value: string; label: string; count?: number }

const model = defineModel<string>({ required: true })

defineProps<{ items: TabItem[] }>()
</script>

<template>
  <TabsRoot v-model="model" class="min-w-0">
    <TabsList class="relative flex max-w-full items-center gap-1 overflow-x-auto overscroll-x-contain rounded-control border border-line bg-sunken/60 p-1">
      <TabsTrigger
        v-for="item in items"
        :key="item.value"
        :value="item.value"
        class="relative flex min-h-9 shrink-0 items-center justify-center gap-2 rounded-control border border-transparent px-3 py-1.5 text-sm font-medium whitespace-nowrap text-muted transition-colors duration-150 ease-out hover:bg-surface/60 hover:text-ink focus-visible:z-10 focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-clay data-[state=active]:border-line data-[state=active]:bg-surface data-[state=active]:text-clay motion-reduce:transition-none"
      >
        {{ item.label }}
        <span
          v-if="item.count !== undefined"
          class="numeric inline-flex min-w-5 items-center justify-center rounded-full border border-line bg-sunken px-1.5 text-2xs leading-5 text-muted"
        >{{ item.count }}</span>
      </TabsTrigger>
      <TabsIndicator class="hidden" />
    </TabsList>

    <slot />
  </TabsRoot>
</template>

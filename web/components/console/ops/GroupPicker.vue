<script setup lang="ts">
import type { Group } from '~/src/api'

const model = defineModel<string[]>({ default: () => [] })

const props = defineProps<{ options: Group[] }>()

const { t } = useI18n()

function toggle(id: string, checked: boolean) {
  const next = new Set(model.value)
  if (checked) next.add(id)
  else next.delete(id)
  model.value = props.options.filter(group => next.has(group.id)).map(group => group.id)
}
</script>

<template>
  <p v-if="!options.length" class="text-[13px] text-muted">{{ t('admin.noGroups') }}</p>
  <div v-else class="flex flex-wrap gap-x-4 gap-y-2 rounded-control border border-line bg-sunken px-3 py-2.5">
    <UiCheckbox
      v-for="group in options"
      :key="group.id"
      :model-value="model.includes(group.id)"
      @update:model-value="toggle(group.id, $event)"
    >
      {{ group.display_name || group.name }}
    </UiCheckbox>
  </div>
</template>

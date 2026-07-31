<script setup lang="ts">
import { ShieldAlert } from 'lucide-vue-next'

const props = defineProps<{ permission: string }>()

const { can, loadAccount } = useAccount()
const { t } = useI18n()

// The account is fetched on mount, so gate the decision until it resolves —
// otherwise the "no access" state flashes before permissions are known.
const resolved = ref(false)
const allowed = computed(() => resolved.value && can(props.permission))

onMounted(async () => {
  await loadAccount()
  resolved.value = true
})
</script>

<template>
  <div v-if="!resolved" class="space-y-4">
    <UiSkeleton :rows="3" class="h-10" />
  </div>

  <slot v-else-if="allowed" />

  <UiCard v-else>
    <UiEmptyState
      :icon="ShieldAlert"
      :title="t('system.noAccessTitle')"
      :description="t('system.noAccessBody')"
    />
  </UiCard>
</template>

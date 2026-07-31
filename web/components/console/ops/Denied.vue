<script setup lang="ts">
import { ShieldOff } from 'lucide-vue-next'

const { t } = useI18n()
</script>

<template>
  <div class="rounded-card border border-line bg-surface">
    <UiEmptyState :icon="ShieldOff" :title="t('admin.noAccessTitle')" :description="t('admin.noAccessBody')" />
  </div>
</template>

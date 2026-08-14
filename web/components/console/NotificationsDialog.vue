<script setup lang="ts">
import { Bell } from 'lucide-vue-next'

const { notifications, open } = useNotifications()
const { t } = useI18n()
</script>

<template>
  <UiDialog v-model:open="open" size="sm" :title="t('system.notificationsPopupTitle')">
    <div class="space-y-3">
      <div v-for="n in notifications" :key="n.id" class="rounded-control border border-line bg-paper p-3.5">
        <p class="flex items-center gap-1.5 text-[13px] font-semibold text-ink">
          <Bell class="size-3.5 shrink-0 text-clay" />
          {{ n.title }}
        </p>
        <p v-if="n.content" class="mt-1 text-[13px] leading-relaxed whitespace-pre-wrap text-muted">
          {{ n.content }}
        </p>
      </div>
    </div>

    <template #footer>
      <UiButton variant="secondary" @click="open = false">{{ t('common.close') }}</UiButton>
    </template>
  </UiDialog>
</template>

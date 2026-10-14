<script setup lang="ts">
import { DialogContent, DialogDescription, DialogOverlay, DialogPortal, DialogRoot, DialogTitle } from 'reka-ui'

const { t } = useI18n()
const { ready, busy, failed, accept, retry } = useLegalConsent()
const { error: accountError } = useAccount()
const agreed = ref(false)
const declined = ref(false)
</script>

<template>
  <DialogRoot :open="true">
    <DialogPortal>
      <DialogOverlay class="fixed inset-0 z-[100] bg-[var(--overlay)] backdrop-blur-md" />
      <DialogContent
        class="fixed top-1/2 left-1/2 z-[100] flex max-h-[88vh] w-[calc(100vw-2rem)] max-w-lg -translate-x-1/2 -translate-y-1/2 flex-col gap-5 overflow-y-auto rounded-card border border-line bg-surface p-6 shadow-pop focus:outline-none"
        @escape-key-down.prevent
        @pointer-down-outside.prevent
        @interact-outside.prevent
      >
        <DialogTitle class="text-lg font-semibold text-ink">{{ t('site.consentTitle') }}</DialogTitle>
        <DialogDescription class="text-sm leading-relaxed text-muted">{{ t('site.consentDescription') }}</DialogDescription>
        <div class="flex flex-wrap gap-4">
          <NuxtLink to="/terms" class="text-clay underline underline-offset-4">{{ t('nav.terms') }}</NuxtLink>
          <NuxtLink to="/privacy" class="text-clay underline underline-offset-4">{{ t('nav.privacy') }}</NuxtLink>
        </div>
        <UiAlert v-if="failed || accountError" tone="danger">{{ t('site.consentError') }}</UiAlert>
        <template v-if="declined">
          <p class="text-sm text-muted">{{ t('site.consentDeclined') }}</p>
          <UiButton @click="declined = false">{{ t('site.consentReview') }}</UiButton>
        </template>
        <template v-else-if="ready">
          <UiCheckbox v-model="agreed" :disabled="busy" :label="t('site.consentCheckbox')" />
          <div class="flex flex-wrap justify-end gap-2">
            <UiButton variant="secondary" :disabled="busy" @click="declined = true; agreed = false">{{ t('site.consentDecline') }}</UiButton>
            <UiButton :disabled="!agreed || busy" :loading="busy" @click="accept">{{ t('site.consentAccept') }}</UiButton>
          </div>
        </template>
        <UiButton v-else-if="failed || accountError" :loading="busy" @click="retry">{{ t('site.consentRetry') }}</UiButton>
        <p v-else role="status" class="text-sm text-muted">{{ t('site.consentLoading') }}</p>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>

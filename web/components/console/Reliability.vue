<script setup lang="ts">
import { useConsoleStore } from '~/composables/useConsoleStore'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Checkbox } from '@/components/ui/checkbox'
import { Card, CardContent } from '@/components/ui/card'

const store = useConsoleStore()
const { t, busy, reliabilityForm, saveReliabilitySettings } = store
</script>

<template>
  <section class="flex flex-wrap items-center justify-between gap-4">
    <div>
      <h2 class="text-lg font-semibold">{{ t('reliability') }}</h2>
      <p class="text-sm text-muted-foreground">{{ t('reliabilityDesc') }}</p>
    </div>
  </section>
  <Card class="mt-4">
    <CardContent>
      <form class="flex flex-col gap-6" @submit.prevent="saveReliabilitySettings">
        <div class="flex flex-col gap-4">
          <h3 class="text-sm font-medium">{{ t('requestRetry') }}</h3>
          <div class="flex flex-col gap-2">
            <Label>{{ t('retryCount') }}</Label>
            <Input v-model.number="reliabilityForm.retry_count" type="number" min="0" max="10" step="1" required class="max-w-xs" />
            <p class="text-xs text-muted-foreground">{{ t('retryCountHint') }}</p>
          </div>
          <div class="flex flex-col gap-2">
            <Label>{{ t('retryStatusCodes') }}</Label>
            <Input v-model="reliabilityForm.retry_status_codes" required placeholder="100-199,300-407,409-503,505-523,525-599" />
            <p class="text-xs text-muted-foreground">{{ t('statusCodesHint') }}</p>
          </div>
        </div>

        <div class="flex flex-col gap-4">
          <h3 class="text-sm font-medium">{{ t('channelHealthCheck') }}</h3>
          <div class="flex flex-col gap-2">
            <Label>{{ t('healthCheckMode') }}</Label>
            <select v-model="reliabilityForm.health_check_mode" class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring">
              <option value="off">{{ t('healthCheckOff') }}</option>
              <option value="scheduled_all">{{ t('healthCheckScheduledAll') }}</option>
              <option value="passive_recovery">{{ t('healthCheckPassiveRecovery') }}</option>
            </select>
            <p class="text-xs text-muted-foreground">{{ t('healthCheckModeHint') }}</p>
          </div>
          <div class="flex flex-col gap-2">
            <Label>{{ t('healthCheckInterval') }}</Label>
            <Input v-model.number="reliabilityForm.health_check_interval_minutes" type="number" min="1" max="1440" step="1" required class="max-w-xs" />
            <p class="text-xs text-muted-foreground">{{ t('healthCheckIntervalHint') }}</p>
          </div>
          <div class="flex items-center gap-2">
            <Checkbox id="hc-auto-recover" :model-value="reliabilityForm.health_check_auto_recover" @update:model-value="v => reliabilityForm.health_check_auto_recover = !!v" />
            <Label for="hc-auto-recover">{{ t('healthCheckAutoRecover') }}</Label>
          </div>
          <div class="flex flex-col gap-2">
            <Label>{{ t('healthCheckChannelIds') }}</Label>
            <Textarea v-model="reliabilityForm.health_check_channel_ids" rows="3" :placeholder="t('healthCheckChannelIdsPlaceholder')" />
            <p class="text-xs text-muted-foreground">{{ t('healthCheckChannelIdsHint') }}</p>
          </div>
        </div>

        <div class="flex flex-col gap-4">
          <h3 class="text-sm font-medium">{{ t('autoDisableRules') }}</h3>
          <div class="flex items-center gap-2">
            <Checkbox id="ad-test-failure" :model-value="reliabilityForm.auto_disable_on_test_failure" @update:model-value="v => reliabilityForm.auto_disable_on_test_failure = !!v" />
            <Label for="ad-test-failure">{{ t('autoDisableOnTestFailure') }}</Label>
          </div>
          <div class="flex flex-col gap-2">
            <Label>{{ t('autoDisableSlowSeconds') }}</Label>
            <Input v-model.number="reliabilityForm.auto_disable_slow_seconds" type="number" min="0" max="600" step="1" class="max-w-xs" />
            <p class="text-xs text-muted-foreground">{{ t('autoDisableSlowSecondsHint') }}</p>
          </div>
          <div class="flex flex-col gap-2">
            <Label>{{ t('autoDisableStatusCodes') }}</Label>
            <Input v-model="reliabilityForm.auto_disable_status_codes" placeholder="401,429,503" />
            <p class="text-xs text-muted-foreground">{{ t('statusCodesHint') }}</p>
          </div>
          <div class="flex flex-col gap-2">
            <Label>{{ t('autoDisableKeywords') }}</Label>
            <Textarea v-model="reliabilityForm.auto_disable_keywords" rows="8" :placeholder="t('autoDisableKeywordsPlaceholder')" />
            <p class="text-xs text-muted-foreground">{{ t('autoDisableKeywordsHint') }}</p>
          </div>
        </div>

        <Button type="submit" :disabled="busy" class="w-fit">{{ t('saveSettings') }}</Button>
      </form>
    </CardContent>
  </Card>
</template>

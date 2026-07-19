<script setup lang="ts">
import { useConsoleStore } from '~/composables/useConsoleStore'

const store = useConsoleStore()
const { t, busy, reliabilityForm, saveReliabilitySettings } = store
</script>

<template>
  <section class="toolbar">
    <div>
      <h2>{{ t('reliability') }}</h2>
      <p>{{ t('reliabilityDesc') }}</p>
    </div>
  </section>
  <form class="panel pricing-form reliability-form" @submit.prevent="saveReliabilitySettings">
    <h3 class="reliability-section-title">{{ t('requestRetry') }}</h3>
    <label>{{ t('retryCount') }}<input v-model.number="reliabilityForm.retry_count" type="number" min="0" max="10" step="1" required /><small>{{ t('retryCountHint') }}</small></label>
    <label>{{ t('retryStatusCodes') }}<input v-model="reliabilityForm.retry_status_codes" required placeholder="100-199,300-407,409-503,505-523,525-599" /><small>{{ t('statusCodesHint') }}</small></label>
    <h3 class="reliability-section-title">{{ t('channelHealthCheck') }}</h3>
    <label>{{ t('healthCheckMode') }}
      <select v-model="reliabilityForm.health_check_mode">
        <option value="off">{{ t('healthCheckOff') }}</option>
        <option value="scheduled_all">{{ t('healthCheckScheduledAll') }}</option>
        <option value="passive_recovery">{{ t('healthCheckPassiveRecovery') }}</option>
      </select>
      <small>{{ t('healthCheckModeHint') }}</small>
    </label>
    <label>{{ t('healthCheckInterval') }}<input v-model.number="reliabilityForm.health_check_interval_minutes" type="number" min="1" max="1440" step="1" required /><small>{{ t('healthCheckIntervalHint') }}</small></label>
    <label class="payment-enabled"><input v-model="reliabilityForm.health_check_auto_recover" type="checkbox" />{{ t('healthCheckAutoRecover') }}</label>
    <label>{{ t('healthCheckChannelIds') }}<textarea v-model="reliabilityForm.health_check_channel_ids" rows="3" :placeholder="t('healthCheckChannelIdsPlaceholder')"></textarea><small>{{ t('healthCheckChannelIdsHint') }}</small></label>
    <h3 class="reliability-section-title">{{ t('autoDisableRules') }}</h3>
    <label class="payment-enabled"><input v-model="reliabilityForm.auto_disable_on_test_failure" type="checkbox" />{{ t('autoDisableOnTestFailure') }}</label>
    <label>{{ t('autoDisableSlowSeconds') }}<input v-model.number="reliabilityForm.auto_disable_slow_seconds" type="number" min="0" max="600" step="1" /><small>{{ t('autoDisableSlowSecondsHint') }}</small></label>
    <label>{{ t('autoDisableStatusCodes') }}<input v-model="reliabilityForm.auto_disable_status_codes" placeholder="401,429,503" /><small>{{ t('statusCodesHint') }}</small></label>
    <label>{{ t('autoDisableKeywords') }}<textarea v-model="reliabilityForm.auto_disable_keywords" rows="8" :placeholder="t('autoDisableKeywordsPlaceholder')"></textarea><small>{{ t('autoDisableKeywordsHint') }}</small></label>
    <button class="button primary" :disabled="busy">{{ t('saveSettings') }}</button>
  </form>
</template>

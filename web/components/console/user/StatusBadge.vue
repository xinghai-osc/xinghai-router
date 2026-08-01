<script setup lang="ts">
type Tone = 'neutral' | 'clay' | 'success' | 'warn' | 'danger'

const props = defineProps<{ status: string }>()

const { t } = useI18n()

/** Order and subscription states share the wire vocabulary, so one map covers both. */
const TONES: Record<string, Tone> = {
  pending: 'warn',
  paid: 'success',
  failed: 'danger',
  expired: 'neutral',
  active: 'success',
  cancelled: 'neutral',
  settled: 'success',
  success: 'success',
  error: 'danger',
}

const LABEL_KEYS: Record<string, string> = {
  pending: 'console.statusPending',
  paid: 'console.statusPaid',
  failed: 'console.statusFailed',
  expired: 'console.statusExpired',
  active: 'console.subStatusActive',
  cancelled: 'console.subStatusCancelled',
  settled: 'console.statusSettled',
  success: 'console.statusSuccess',
}

const tone = computed<Tone>(() => TONES[props.status] ?? 'neutral')
// Unmapped states come straight from the API and are shown verbatim.
const label = computed(() => (LABEL_KEYS[props.status] ? t(LABEL_KEYS[props.status]) : props.status))
</script>

<template>
  <UiBadge :tone="tone" dot>{{ label }}</UiBadge>
</template>

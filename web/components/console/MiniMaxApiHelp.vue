<script setup lang="ts">
const props = defineProps<{ baseUrl: string }>()
const { t } = useI18n()
const selected = ref('create')

function shellQuote(value: string): string {
  return "'" + value.replaceAll("'", "'\\''") + "'"
}

const samples = computed(() => {
  const base = props.baseUrl.replace(/\/+$/, '').replace(/\/v1$/, '')
  const auth = '  -H "Authorization: Bearer $XINGHAI_API_KEY"'
  const command = (lines: string[]) => lines.join(' \\\n')
  const request = (path: string) => `curl --fail --show-error ${shellQuote(`${base}${path}`)}`
  const body = {
    model: 'MiniMax-H3',
    content: [{ type: 'text', text: t('console.mediaVideoPrompt') }],
    resolution: '768P',
    duration: 5,
    ratio: '16:9',
  }
  return [
    {
      value: 'create', label: t('console.minimaxVideoCreate'), hint: t('console.minimaxVideoCreateHint'),
      code: command([request('/v2/video_generation'), auth,
        '  -H "Content-Type: application/json"', `  -d ${shellQuote(JSON.stringify(body, null, 2))}`]),
    },
    {
      value: 'query', label: t('console.minimaxVideoQuery'), hint: t('console.minimaxVideoQueryHint'),
      code: `TASK_ID="YOUR_TASK_ID"\n${command([`${request('/v2/query/video_generation')}/"$TASK_ID"`, auth])}`,
    },
    {
      value: 'delete', label: t('console.minimaxVideoDelete'), hint: t('console.minimaxVideoDeleteHint'),
      code: `TASK_ID="YOUR_TASK_ID"\n${command([`${request('/v2/video_generation')}/"$TASK_ID"`, '  -X DELETE', auth])}`,
    },
  ]
})
const current = computed(() => samples.value.find(sample => sample.value === selected.value) ?? samples.value[0]!)
</script>

<template>
  <details class="min-w-0 rounded-control border border-line bg-surface p-3">
    <summary class="cursor-pointer text-sm font-medium text-ink">{{ t('console.minimaxHelpTitle') }}</summary>
    <div class="mt-4 min-w-0 space-y-3">
      <p class="text-xs leading-5 text-muted">{{ t('console.minimaxHelpIntro') }}</p>
      <UiField :label="t('console.mediaOperation')">
        <UiSelect v-model="selected" :options="samples" />
      </UiField>
      <p class="text-xs leading-5 text-muted">{{ current.hint }}</p>
      <pre class="max-w-full overflow-x-auto rounded-control bg-sunken p-3 font-mono text-[11px] leading-5 text-ink"><code>{{ current.code }}</code></pre>
      <ConsoleUserCopyButton :value="current.code" :label="t('console.mediaCopyExample')" />
      <p class="text-xs leading-5 text-muted">{{ t('console.minimaxVideoBillingHint') }}</p>
      <p class="text-xs leading-5 text-muted">{{ t('console.minimaxHelpAdmin') }}</p>
      <a href="https://platform.minimax.cn/docs/m-plan/intro" target="_blank" rel="noopener noreferrer" class="text-xs text-clay underline">{{ t('console.minimaxHelpDocs') }}</a>
    </div>
  </details>
</template>

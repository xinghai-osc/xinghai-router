<script setup lang="ts">
const props = defineProps<{ baseUrl: string }>()
const { t } = useI18n()
const selected = ref('speech')

function shellQuote(value: string): string {
  return "'" + value.replaceAll("'", "'\\''") + "'"
}

const samples = computed(() => {
  const base = props.baseUrl.replace(/\/+$/, '')
  const auth = '  -H "Authorization: Bearer $XINGHAI_API_KEY"'
  const request = (path: string) => `curl --fail --show-error ${shellQuote(`${base}${path}`)}`
  const json = (body: Record<string, string>) => [
    '  -H "Content-Type: application/json"',
    `  -d ${shellQuote(JSON.stringify(body))}`,
  ]
  const command = (lines: string[]) => lines.join(' \\\n')
  return [
    {
      value: 'speech', label: t('console.mediaSpeech'), hint: t('console.mediaSpeechHint'),
      code: command([request('/audio/speech'), auth,
        ...json({ model: 'YOUR_SPEECH_MODEL', input: t('console.mediaSpeechInput'), voice: 'alloy', response_format: 'mp3' }),
        '  --output speech.mp3']),
    },
    ...['transcriptions', 'translations'].map(operation => ({
      value: operation,
      label: t(operation === 'transcriptions' ? 'console.mediaTranscription' : 'console.mediaTranslation'),
      hint: t('console.mediaTranscriptionHint'),
      code: command([request(`/audio/${operation}`), auth,
        "  -F 'model=YOUR_AUDIO_MODEL'", "  -F 'file=@input.mp3'", "  -F 'max_audio_seconds=60'", "  -F 'response_format=verbose_json'"]),
    })),
    {
      value: 'video-json', label: t('console.mediaVideoJson'), hint: t('console.mediaVideoCreateHint'),
      code: command([request('/videos'), auth,
        ...json({ model: 'YOUR_VIDEO_MODEL', prompt: t('console.mediaVideoPrompt'), seconds: '4' })]),
    },
    {
      value: 'video-multipart', label: t('console.mediaVideoMultipart'), hint: t('console.mediaVideoMultipartHint'),
      code: command([request('/videos'), auth,
        "  -F 'model=YOUR_VIDEO_MODEL'", `  --form-string ${shellQuote(`prompt=${t('console.mediaVideoPrompt')}`)}`,
        "  -F 'seconds=4'", "  -F 'input_reference=@reference.png'"]),
    },
    {
      value: 'video-status', label: t('console.mediaVideoStatus'), hint: t('console.mediaVideoStatusHint'),
      code: `VIDEO_ID="YOUR_VIDEO_ID"\n${command([`${request('/videos')}/"$VIDEO_ID"`, auth])}`,
    },
    {
      value: 'video-content', label: t('console.mediaVideoContent'), hint: t('console.mediaVideoContentHint'),
      code: `VIDEO_ID="YOUR_VIDEO_ID"\n${command([`${request('/videos')}/"$VIDEO_ID"/content`, auth, '  --output video.mp4'])}`,
    },
  ]
})
const current = computed(() => samples.value.find(sample => sample.value === selected.value) ?? samples.value[0]!)
</script>

<template>
  <details id="media-api-help" class="min-w-0 rounded-control border border-line bg-surface p-3">
    <summary class="cursor-pointer text-sm font-medium text-ink">{{ t('console.mediaHelpTitle') }}</summary>
    <div class="mt-4 min-w-0 space-y-3">
      <p class="text-xs leading-5 text-muted">{{ t('console.mediaHelpIntro') }}</p>
      <UiField :label="t('console.mediaOperation')">
        <UiSelect v-model="selected" :options="samples" />
      </UiField>
      <p class="text-xs leading-5 text-muted">{{ current.hint }}</p>
      <p v-if="selected === 'transcriptions' || selected === 'translations'" class="text-xs leading-5 text-muted">{{ t('console.mediaAudioReservationHint') }}</p>
      <pre class="max-w-full overflow-x-auto rounded-control bg-sunken p-3 font-mono text-[11px] leading-5 text-ink"><code>{{ current.code }}</code></pre>
      <ConsoleUserCopyButton :value="current.code" :label="t('console.mediaCopyExample')" />
      <p class="text-xs leading-5 text-muted">{{ t('console.mediaBillingHint') }}</p>
    </div>
  </details>
</template>

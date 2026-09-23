<script setup lang="ts">
import { useClipboard } from '@vueuse/core'
import { Check, Copy, Terminal } from 'lucide-vue-next'
import { TabsContent, TabsList, TabsRoot, TabsTrigger } from 'reka-ui'

const { t } = useI18n()
const origin = ref('https://your-gateway.example.com')
onMounted(() => { origin.value = window.location.origin })

const active = ref('curl')
const sampleId = useId()

const samples = computed(() => [
  {
    value: 'curl',
    label: t('site.codeCurl'),
    code: `curl ${origin.value}/api/v1/chat/completions \\
  -H "Authorization: Bearer $XINGHAI_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "claude-sonnet-4",
    "messages": [{"role": "user", "content": "${t('site.codeGreeting')}"}]
  }'`,
  },
  {
    value: 'python',
    label: t('site.codePython'),
    code: `import os
from openai import OpenAI

client = OpenAI(
    base_url="${origin.value}/api/v1",
    api_key=os.environ["XINGHAI_API_KEY"],
)

response = client.chat.completions.create(
    model="claude-sonnet-4",
    messages=[{"role": "user", "content": "${t('site.codeGreeting')}"}],
)`,
  },
  {
    value: 'anthropic',
    label: t('site.codeAnthropic'),
    code: `curl ${origin.value}/api/v1/messages \\
  -H "x-api-key: $XINGHAI_API_KEY" \\
  -H "anthropic-version: 2023-06-01" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "claude-sonnet-4",
    "max_tokens": 1024,
    "messages": [{"role": "user", "content": "${t('site.codeGreeting')}"}]
  }'`,
  },
])

const current = computed(() => samples.value.find(sample => sample.value === active.value) ?? samples.value[0]!)
const { copy, copied } = useClipboard({ copiedDuring: 1600 })
</script>

<template>
  <TabsRoot v-model="active" class="min-w-0 overflow-hidden rounded-card border border-line bg-surface text-left">
    <div class="flex items-center justify-between gap-3 border-b border-line px-4 py-3">
      <span class="inline-flex items-center gap-2 text-2xs font-medium text-muted">
        <Terminal class="size-3.5 text-clay" aria-hidden="true" />
        {{ t('site.codeTerminal') }}
      </span>
      <button
        type="button"
        class="inline-flex items-center gap-1.5 rounded-control px-2 py-1 text-2xs text-muted transition-colors duration-150 ease-out hover:bg-sunken hover:text-ink"
        :aria-label="copied ? t('common.copied') : t('site.codeCopy')"
        @click="copy(current.code)"
      >
        <Check v-if="copied" class="size-3.5 text-success" aria-hidden="true" />
        <Copy v-else class="size-3.5" aria-hidden="true" />
        <span aria-live="polite">{{ copied ? t('common.copied') : t('common.copy') }}</span>
      </button>
    </div>

    <TabsList class="flex gap-5 overflow-x-auto border-b border-line bg-sunken/40 px-4" :aria-label="t('site.codeLanguage')">
      <TabsTrigger
        v-for="sample in samples"
        :id="`${sampleId}-${sample.value}-tab`"
        :key="sample.value"
        :value="sample.value"
        :aria-controls="`${sampleId}-${sample.value}-panel`"
        class="shrink-0 border-b-2 border-transparent py-3 text-xs font-medium text-muted transition-colors duration-150 ease-out hover:text-ink data-[state=active]:border-clay data-[state=active]:text-clay"
      >{{ sample.label }}</TabsTrigger>
    </TabsList>

    <TabsContent
      v-for="sample in samples"
      :id="`${sampleId}-${sample.value}-panel`"
      :key="sample.value"
      :value="sample.value"
      :aria-labelledby="`${sampleId}-${sample.value}-tab`"
      class="min-w-0"
    >
      <pre class="min-h-[17rem] overflow-x-auto py-5 pr-5 font-mono text-[11px] leading-[1.85] text-ink sm:text-xs"><code><span v-for="(line, index) in sample.code.split('\n')" :key="index" class="block min-h-[1.85em]"><span class="inline-block w-10 select-none pr-3 text-right text-faint/60" aria-hidden="true">{{ index + 1 }}</span>{{ line }}</span></code></pre>
    </TabsContent>
  </TabsRoot>
</template>

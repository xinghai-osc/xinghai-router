<script setup lang="ts">
import { useClipboard } from '@vueuse/core'
import { Check, Copy } from 'lucide-vue-next'

const origin = ref('https://your-gateway.example.com')
onMounted(() => { origin.value = window.location.origin })

const active = ref('curl')

const samples = computed(() => [
  {
    value: 'curl',
    label: 'cURL',
    code: `curl ${origin.value}/api/v1/chat/completions \\
  -H "Authorization: Bearer $XINGHAI_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "claude-sonnet-4",
    "messages": [{"role": "user", "content": "你好"}]
  }'`,
  },
  {
    value: 'python',
    label: 'Python',
    code: `from openai import OpenAI

client = OpenAI(
    base_url="${origin.value}/api/v1",
    api_key=os.environ["XINGHAI_API_KEY"],
)

response = client.chat.completions.create(
    model="claude-sonnet-4",
    messages=[{"role": "user", "content": "你好"}],
)`,
  },
  {
    value: 'anthropic',
    label: 'Anthropic',
    code: `curl ${origin.value}/api/v1/messages \\
  -H "x-api-key: $XINGHAI_API_KEY" \\
  -H "anthropic-version: 2023-06-01" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "claude-sonnet-4",
    "max_tokens": 1024,
    "messages": [{"role": "user", "content": "你好"}]
  }'`,
  },
])

const current = computed(() => samples.value.find(sample => sample.value === active.value) ?? samples.value[0]!)
const { copy, copied } = useClipboard({ copiedDuring: 1600 })
const { t } = useI18n()
</script>

<template>
  <div class="overflow-hidden rounded-card border border-line bg-surface text-left">
    <div class="flex items-center justify-between gap-2 border-b border-line bg-sunken px-2 py-1.5">
      <div class="flex items-center gap-1">
        <button
          v-for="sample in samples"
          :key="sample.value"
          type="button"
          :class="[
            'rounded-[7px] px-2.5 py-1 text-[13px] transition-colors duration-150',
            active === sample.value ? 'bg-surface font-medium text-ink' : 'text-muted hover:text-ink',
          ]"
          @click="active = sample.value"
        >{{ sample.label }}</button>
      </div>

      <button
        type="button"
        class="inline-flex items-center gap-1.5 rounded-[7px] px-2 py-1 text-2xs text-muted transition-colors hover:text-ink"
        :aria-label="copied ? t('common.copied') : t('site.codeCopy')"
        @click="copy(current.code)"
      >
        <Check v-if="copied" class="size-3.5 text-success" />
        <Copy v-else class="size-3.5" />
        {{ copied ? t('common.copied') : t('common.copy') }}
      </button>
    </div>

    <pre class="overflow-x-auto px-4 py-4 font-mono text-[12.5px] leading-relaxed text-ink"><code>{{ current.code }}</code></pre>
  </div>
</template>

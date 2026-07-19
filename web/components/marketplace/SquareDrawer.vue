<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Check, Code2, Copy, Info } from 'lucide-vue-next'
import { formatRatio, formatSquarePrice, groupPrice, vendorColor, vendorIconUrl, type SquareModel, type TokenUnit } from '~/src/marketplace'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Dialog, DialogContent, DialogTitle } from '@/components/ui/dialog'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

const props = defineProps<{
  model: SquareModel
  tokenUnit: TokenUnit
  origin: string
}>()
const emit = defineEmits<{ close: [] }>()
const { t, locale } = useI18n()

const tab = ref<'overview' | 'api'>('overview')
const iconError = ref(false)
const copiedName = ref(false)
const copiedCurl = ref(false)
const unitLabel = computed(() => (props.tokenUnit === 'K' ? '1K' : '1M'))
const hasCache = computed(() => props.model.cached_input_per_million != null && Number(props.model.cached_input_per_million) !== 0)

function price(kind: 'input' | 'output' | 'cache', group?: (props.model.groups)[number]) {
  const value = group
    ? groupPrice(props.model, kind, group)
    : groupPrice(props.model, kind, props.model.groups[0] ?? { id: '', name: '', multiplier: 1 })
  return formatSquarePrice(value, props.tokenUnit)
}

const baseGroupFree = computed(() => ({
  input: formatSquarePrice(props.model.input_per_million == null ? null : Number(props.model.input_per_million) * Number(props.model.multiplier ?? 1), props.tokenUnit),
  output: formatSquarePrice(props.model.output_per_million == null ? null : Number(props.model.output_per_million) * Number(props.model.multiplier ?? 1), props.tokenUnit),
  cache: formatSquarePrice(props.model.cached_input_per_million == null || Number(props.model.cached_input_per_million) === 0 ? null : Number(props.model.cached_input_per_million) * Number(props.model.multiplier ?? 1), props.tokenUnit),
}))

const curlExample = computed(() => `curl ${props.origin}/v1/chat/completions \\
  -H "Authorization: Bearer sk-xh-your-key" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"${props.model.model}","messages":[{"role":"user","content":"${locale.value === 'en-US' ? 'Hello' : '你好'}"}]}'`)

const endpoints = computed(() => [
  { method: 'POST', path: '/v1/chat/completions', label: 'OpenAI Chat Completions' },
  { method: 'GET', path: '/v1/models', label: 'OpenAI Models' },
  { method: 'POST', path: '/v1/messages', label: 'Anthropic Messages' },
])

async function copyText(value: string, flag: 'name' | 'curl') {
  try {
    await navigator.clipboard.writeText(value)
  } catch {
    const textarea = document.createElement('textarea')
    textarea.value = value
    document.body.append(textarea)
    textarea.select()
    document.execCommand('copy')
    textarea.remove()
  }
  if (flag === 'name') { copiedName.value = true; window.setTimeout(() => { copiedName.value = false }, 1500) }
  else { copiedCurl.value = true; window.setTimeout(() => { copiedCurl.value = false }, 1500) }
}

const open = computed({
  get: () => true,
  set: (v: boolean) => { if (!v) emit('close') },
})

watch(() => props.model.model, () => { tab.value = 'overview' })
</script>

<template>
  <Dialog :open="open" @update:open="v => !v && emit('close')">
    <DialogContent class="max-w-2xl">
      <DialogTitle class="sr-only">{{ props.model.model }}</DialogTitle>
      <header class="flex items-start gap-3">
        <span class="flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-lg bg-muted">
          <img v-if="props.model.vendor_slug && !iconError" :src="vendorIconUrl(props.model.vendor_slug)" :alt="props.model.vendor_name" @error="iconError = true">
          <span v-else class="text-base font-bold">{{ props.model.model.slice(0, 1).toUpperCase() }}</span>
        </span>
        <div class="min-w-0 flex-1">
          <div class="flex items-center gap-2">
            <h1 class="truncate text-lg font-semibold">{{ props.model.model }}</h1>
            <Button variant="ghost" size="icon-sm" class="shrink-0" :title="copiedName ? t('msCopied') : t('msCopyModelName')" @click="copyText(props.model.model, 'name')">
              <Check v-if="copiedName" :size="12" />
              <Copy v-else :size="12" />
            </Button>
          </div>
          <div class="mt-1 flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
            <span v-if="props.model.vendor_name">{{ props.model.vendor_name }}</span>
            <span>·</span>
            <Badge variant="outline">{{ t('msTokenBased') }}</Badge>
          </div>
          <p v-if="props.model.description" class="mt-2 text-sm text-muted-foreground">{{ props.model.description }}</p>
        </div>
      </header>

      <Tabs :default-value="tab" class="mt-4">
        <TabsList>
          <TabsTrigger value="overview" class="gap-1.5" @click="tab = 'overview'"><Info :size="13" />{{ t('msOverview') }}</TabsTrigger>
          <TabsTrigger value="api" class="gap-1.5" @click="tab = 'api'"><Code2 :size="13" />API</TabsTrigger>
        </TabsList>

        <TabsContent value="overview" class="flex flex-col gap-4">
          <section class="rounded-lg border border-border p-4">
            <h2 class="mb-3 text-sm font-semibold">{{ t('msPricing') }}</h2>

            <h3 class="mb-2 text-xs font-medium text-muted-foreground">{{ t('msBasePrice') }}</h3>
            <div class="mb-3 grid grid-cols-2 gap-2">
              <div class="rounded-md border border-border p-3">
                <span class="text-xs text-muted-foreground">{{ t('inputLabel') }}</span>
                <b v-if="baseGroupFree.input" class="mt-1 block text-lg font-semibold">{{ baseGroupFree.input }}<small class="ml-1 text-xs font-normal text-muted-foreground">/ {{ unitLabel }}</small></b>
                <b v-else class="mt-1 block text-sm text-muted-foreground">{{ t('pendingConfig') }}</b>
              </div>
              <div class="rounded-md border border-border p-3">
                <span class="text-xs text-muted-foreground">{{ t('outputLabel') }}</span>
                <b v-if="baseGroupFree.output" class="mt-1 block text-lg font-semibold">{{ baseGroupFree.output }}<small class="ml-1 text-xs font-normal text-muted-foreground">/ {{ unitLabel }}</small></b>
                <b v-else class="mt-1 block text-sm text-muted-foreground">{{ t('pendingConfig') }}</b>
              </div>
            </div>
            <div v-if="baseGroupFree.cache" class="mb-4">
              <div class="rounded-md border border-border p-3">
                <span class="text-xs text-muted-foreground">{{ t('cachedInputLabel') }}</span>
                <b class="mt-1 block font-mono text-base font-semibold">{{ baseGroupFree.cache }}<small class="ml-1 text-xs font-normal text-muted-foreground">/ {{ unitLabel }}</small></b>
              </div>
            </div>

            <h3 class="mb-2 text-xs font-medium text-muted-foreground">{{ t('msGroupPricing') }}</h3>
            <div v-if="props.model.groups.length" class="overflow-hidden rounded-md border border-border">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{{ t('msGroup') }}</TableHead>
                    <TableHead>{{ t('msRatio') }}</TableHead>
                    <TableHead class="text-right">{{ t('inputLabel') }}</TableHead>
                    <TableHead class="text-right">{{ t('outputLabel') }}</TableHead>
                    <TableHead v-if="hasCache" class="text-right">{{ t('msCached') }}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  <TableRow v-for="group in props.model.groups" :key="group.id">
                    <TableCell><Badge variant="secondary">{{ group.name }}</Badge></TableCell>
                    <TableCell class="font-mono text-muted-foreground">{{ formatRatio(Number(group.multiplier)) }}</TableCell>
                    <TableCell class="text-right font-mono">{{ price('input', group) || '—' }}</TableCell>
                    <TableCell class="text-right font-mono">{{ price('output', group) || '—' }}</TableCell>
                    <TableCell v-if="hasCache" class="text-right font-mono">{{ price('cache', group) || '—' }}</TableCell>
                  </TableRow>
                </TableBody>
              </Table>
            </div>
            <p v-else class="text-sm text-muted-foreground">{{ t('msNoGroups') }}</p>
            <p class="mt-2 text-xs text-muted-foreground">{{ t('msPriceNoteA') }} {{ unitLabel }} {{ t('msPriceNoteB') }}</p>
          </section>

          <section class="rounded-lg border border-border p-4">
            <h2 class="mb-3 text-sm font-semibold">{{ t('msModel') }}</h2>
            <div class="grid grid-cols-2 gap-3">
              <div>
                <span class="text-xs text-muted-foreground">{{ t('msVendor') }}</span>
                <div class="mt-1"><span class="inline-flex rounded-full px-2 py-0.5 text-xs font-medium" :style="{ background: vendorColor(props.model.vendor_name).bg, color: vendorColor(props.model.vendor_name).fg }">{{ props.model.vendor_name }}</span></div>
              </div>
              <div>
                <span class="text-xs text-muted-foreground">{{ t('msType') }}</span>
                <div class="mt-1"><Badge variant="outline">{{ t('msTokenBased') }}</Badge></div>
              </div>
              <div>
                <span class="text-xs text-muted-foreground">{{ t('msGroups') }}</span>
                <div class="mt-1 flex flex-wrap gap-1"><Badge v-for="group in props.model.groups" :key="group.id" variant="secondary" class="text-[10px]">{{ group.name }}</Badge></div>
              </div>
              <div>
                <span class="text-xs text-muted-foreground">{{ t('msEndpoints') }}</span>
                <div class="mt-1 flex flex-wrap gap-1"><Badge variant="secondary" class="text-[10px]">openai</Badge><Badge variant="secondary" class="text-[10px]">anthropic</Badge></div>
              </div>
            </div>
          </section>
        </TabsContent>

        <TabsContent value="api" class="flex flex-col gap-4">
          <section class="rounded-lg border border-border p-4">
            <h2 class="mb-3 text-sm font-semibold">{{ t('msAvailableEndpoints') }}</h2>
            <div class="flex flex-col gap-2">
              <div v-for="endpoint in endpoints" :key="endpoint.path" class="flex items-center gap-2 rounded-md border border-border p-2">
                <Badge variant="secondary" class="font-mono text-[10px]">{{ endpoint.method }}</Badge>
                <code class="font-mono text-xs">{{ endpoint.path }}</code>
                <small class="ml-auto text-xs text-muted-foreground">{{ endpoint.label }}</small>
              </div>
            </div>
          </section>
          <section class="rounded-lg border border-border p-4">
            <div class="mb-3 flex items-center justify-between">
              <h2 class="text-sm font-semibold">{{ t('msRequestExample') }}</h2>
              <Button variant="outline" size="sm" @click="copyText(curlExample, 'curl')">
                <Check v-if="copiedCurl" :size="12" />
                <Copy v-else :size="12" />
                {{ copiedCurl ? t('msCopied') : t('msCopy') }}
              </Button>
            </div>
            <pre class="overflow-x-auto rounded-md bg-muted p-3 font-mono text-xs leading-relaxed"><code>{{ curlExample }}</code></pre>
          </section>
        </TabsContent>
      </Tabs>
    </DialogContent>
  </Dialog>
</template>

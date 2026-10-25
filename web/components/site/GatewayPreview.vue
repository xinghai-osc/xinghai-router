<script setup lang="ts">
import { Check, Code2, Layers, Route } from 'lucide-vue-next'
import { squareModelKey, type SquareModel } from '~/src/marketplace'

type ProtocolId = 'openai' | 'anthropic'

const props = withDefaults(defineProps<{
  models?: SquareModel[]
  loading?: boolean
}>(), { models: () => [], loading: false })

const { t } = useI18n()
const protocolId = ref<ProtocolId>('openai')
const selectedModelKey = ref('')

const protocols = computed(() => [
  { id: 'openai' as const, label: t('site.previewProtocolOpenAI'), path: '/api/v1/chat/completions' },
  { id: 'anthropic' as const, label: t('site.previewProtocolAnthropic'), path: '/api/v1/messages' },
])
const activeProtocol = computed(() => protocols.value.find(protocol => protocol.id === protocolId.value) ?? protocols.value[0]!)
const visibleModels = computed(() => props.models.slice(0, 3))
const activeModel = computed(() => props.models.find(model => squareModelKey(model) === selectedModelKey.value) ?? props.models[0])
const activeModelName = computed(() => activeModel.value?.model || t('site.previewFallbackModel'))
const activeVendorName = computed(() => activeModel.value?.vendor_name || t('site.previewProvider'))

function selectModel(model: SquareModel) {
  selectedModelKey.value = squareModelKey(model)
}
</script>

<template>
  <div class="gateway-preview relative isolate overflow-hidden rounded-card border border-line-strong bg-surface">
    <div class="preview-grid pointer-events-none absolute inset-0 -z-10" aria-hidden="true" />

    <header class="flex items-center justify-between gap-3 border-b border-line bg-surface/80 px-4 py-4 sm:px-5">
      <div class="flex min-w-0 items-center gap-3">
        <span class="flex size-9 shrink-0 items-center justify-center rounded-control bg-clay text-clay-ink">
          <Route class="size-4" aria-hidden="true" />
        </span>
        <div class="min-w-0">
          <p class="truncate text-sm font-semibold tracking-tight text-ink">{{ t('site.previewTitle') }}</p>
          <p class="mt-0.5 truncate text-2xs text-muted">{{ t('site.previewFooter') }}</p>
        </div>
      </div>
      <span class="inline-flex shrink-0 items-center gap-1.5 rounded-full border border-clay/20 bg-clay-soft px-2.5 py-1 text-2xs font-medium text-clay">
        <span class="size-1.5 rounded-full bg-clay" aria-hidden="true" />
        {{ t('site.previewBadge') }}
      </span>
    </header>

    <div class="p-4 sm:p-5">
      <div class="grid grid-cols-2 gap-1 rounded-control border border-line bg-sunken/80 p-1" role="group" :aria-label="t('site.previewProtocolLabel')">
        <button
          v-for="protocol in protocols"
          :key="protocol.id"
          type="button"
          :class="[
            'inline-flex min-h-10 items-center justify-center gap-2 rounded-[8px] px-3 text-xs font-medium transition-colors duration-150 ease-out',
            protocolId === protocol.id ? 'bg-surface text-ink' : 'text-muted hover:text-ink',
          ]"
          :aria-pressed="protocolId === protocol.id"
          @click="protocolId = protocol.id"
        >
          <Check v-if="protocolId === protocol.id" class="size-3.5 text-clay" aria-hidden="true" />
          {{ protocol.label }}
        </button>
      </div>

      <div class="mt-3 flex min-w-0 items-center gap-2 overflow-hidden rounded-control border border-line bg-sunken/60 px-3 py-2.5 font-mono text-[11px] sm:text-xs">
        <span class="shrink-0 font-semibold text-clay">POST</span>
        <code class="truncate text-muted" :title="activeProtocol.path">{{ activeProtocol.path }}</code>
      </div>

      <div class="mt-6 grid grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)_auto_minmax(0,1fr)] items-center" :aria-label="t('site.previewRouteLabel')">
        <div class="flex min-w-0 flex-col items-center text-center">
          <span class="flex size-11 items-center justify-center rounded-[14px] border border-line bg-surface text-muted">
            <Code2 class="size-5" aria-hidden="true" />
          </span>
          <span class="mt-2 w-full truncate text-2xs font-medium text-ink">{{ t('site.previewClient') }}</span>
          <span class="mt-0.5 w-full truncate font-mono text-[10px] text-faint">SDK</span>
        </div>

        <span class="route-connector relative mx-1 h-px w-4 bg-line-strong sm:mx-2 sm:w-8" aria-hidden="true">
          <span class="route-packet absolute -top-[2px] size-[5px] rounded-full bg-clay motion-reduce:hidden" />
        </span>

        <div class="relative flex min-w-0 flex-col items-center text-center">
          <span class="absolute inset-0 -z-10 m-auto size-16 animate-pulse rounded-full bg-clay-soft motion-reduce:animate-none" aria-hidden="true" />
          <span class="flex size-12 items-center justify-center rounded-control border border-clay/30 bg-clay text-clay-ink">
            <Route class="size-5" aria-hidden="true" />
          </span>
          <span class="mt-2 w-full truncate text-2xs font-semibold text-clay">{{ t('site.previewGateway') }}</span>
          <span class="mt-0.5 w-full truncate font-mono text-[10px] text-faint">Router</span>
        </div>

        <span class="route-connector relative mx-1 h-px w-4 bg-line-strong sm:mx-2 sm:w-8" aria-hidden="true">
          <span class="route-packet route-packet-late absolute -top-[2px] size-[5px] rounded-full bg-clay motion-reduce:hidden" />
        </span>

        <div class="flex min-w-0 flex-col items-center text-center">
          <SiteVendorMark v-if="activeModel" :name="activeModel.vendor_name" :slug="activeModel.vendor_slug" size="lg" />
          <span v-else class="flex size-10 items-center justify-center rounded-[9px] border border-line bg-clay-soft text-clay">
            <Layers class="size-5" aria-hidden="true" />
          </span>
          <span class="mt-2 w-full truncate text-2xs font-medium text-ink" :title="activeVendorName">{{ activeVendorName }}</span>
          <span class="mt-0.5 w-full truncate font-mono text-[10px] text-faint" :title="activeModelName">{{ activeModelName }}</span>
        </div>
      </div>

      <div class="mt-6 border-t border-line pt-4">
        <div class="mb-2.5 flex items-center justify-between gap-3">
          <p class="text-2xs font-medium text-muted">{{ t('site.previewModelLabel') }}</p>
          <p class="numeric text-[10px] text-faint">
            {{ loading ? t('site.dataLoading') : models.length ? t('site.previewModelCount', { count: models.length }) : t('site.previewModelEmpty') }}
          </p>
        </div>
        <div v-if="loading && !models.length" class="grid grid-cols-3 gap-2" aria-busy="true">
          <UiSkeleton v-for="item in 3" :key="item" class="h-11" />
        </div>
        <div v-else-if="visibleModels.length" class="grid grid-cols-3 gap-2">
          <button
            v-for="model in visibleModels"
            :key="squareModelKey(model)"
            type="button"
            :class="[
              'flex min-w-0 min-h-11 items-center gap-2 rounded-control border px-2.5 py-2 text-left transition-colors duration-150 ease-out',
              activeModel && squareModelKey(activeModel) === squareModelKey(model)
                ? 'border-clay/35 bg-clay-soft text-clay'
                : 'border-line bg-surface/70 text-muted hover:border-line-strong hover:text-ink',
            ]"
            :aria-pressed="Boolean(activeModel && squareModelKey(activeModel) === squareModelKey(model))"
            :title="model.model"
            @click="selectModel(model)"
          >
            <SiteVendorMark :name="model.vendor_name" :slug="model.vendor_slug" size="sm" />
            <span class="min-w-0 flex-1 truncate font-mono text-[10px]">{{ model.model }}</span>
          </button>
        </div>
        <div v-else class="flex min-h-11 items-center gap-2 rounded-control border border-dashed border-line-strong px-3 text-xs text-muted">
          <Layers class="size-4 shrink-0 text-clay" aria-hidden="true" />
          <span class="truncate font-mono">{{ activeModelName }}</span>
        </div>
      </div>
    </div>

    <footer class="flex items-center justify-between gap-3 border-t border-line bg-sunken/40 px-4 py-3 sm:px-5">
      <span class="inline-flex items-center gap-2 text-2xs font-medium text-success">
        <span class="size-1.5 rounded-full bg-success" aria-hidden="true" />
        {{ t('site.previewReady') }}
      </span>
      <span class="font-mono text-[10px] text-faint">base_url → /api/v1</span>
    </footer>
  </div>
</template>

<style scoped>
.preview-grid {
  background-image:
    linear-gradient(color-mix(in srgb, var(--line) 42%, transparent) 1px, transparent 1px),
    linear-gradient(90deg, color-mix(in srgb, var(--line) 42%, transparent) 1px, transparent 1px),
    radial-gradient(circle at 50% 38%, var(--clay-soft), transparent 56%);
  background-size: 28px 28px, 28px 28px, 100% 100%;
}

.route-packet {
  left: 0;
  animation: route-packet 2.4s ease-in-out infinite;
}

.route-packet-late {
  animation-delay: 700ms;
}

@keyframes route-packet {
  0%, 15% {
    left: 0;
    opacity: 0;
  }
  30% {
    opacity: 1;
  }
  70% {
    opacity: 1;
  }
  85%, 100% {
    left: calc(100% - 5px);
    opacity: 0;
  }
}
</style>

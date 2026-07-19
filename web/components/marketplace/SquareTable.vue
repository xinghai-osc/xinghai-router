<script setup lang="ts">
import { computed, ref } from 'vue'
import { effectivePrice, formatSquarePrice, vendorColor, vendorIconUrl, type SquareModel, type TokenUnit } from '~/src/marketplace'
import SquarePagination from './SquarePagination.vue'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'

const props = defineProps<{
  models: SquareModel[]
  tokenUnit: TokenUnit
  selectedGroup: string
  page: number
  totalPages: number
}>()
const emit = defineEmits<{ open: [model: SquareModel]; 'update:page': [value: number] }>()
const { t } = useI18n()

const unitLabel = computed(() => (props.tokenUnit === 'K' ? '1K' : '1M'))
const iconErrors = ref(new Set<string>())
function iconFailed(slug: string) {
  iconErrors.value = new Set(iconErrors.value).add(slug)
}

function price(model: SquareModel, kind: 'input' | 'output' | 'cache') {
  return formatSquarePrice(effectivePrice(model, kind, props.selectedGroup), props.tokenUnit)
}
</script>

<template>
  <div class="overflow-hidden rounded-lg border border-border bg-card">
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{{ t('msModel') }}</TableHead>
          <TableHead>{{ t('msType') }}</TableHead>
          <TableHead>{{ t('msPrice') }}</TableHead>
          <TableHead>{{ t('msCached') }}</TableHead>
          <TableHead>{{ t('msVendor') }}</TableHead>
          <TableHead>{{ t('msGroups') }}</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        <TableRow v-for="model in props.models" :key="model.model" class="cursor-pointer hover:bg-accent/50" @click="emit('open', model)">
          <TableCell>
            <div class="flex items-center gap-2">
              <img v-if="model.vendor_slug && !iconErrors.has(model.vendor_slug)" class="h-5 w-5 rounded-full" :src="vendorIconUrl(model.vendor_slug)" :alt="model.vendor_name" loading="lazy" @error="iconFailed(model.vendor_slug)">
              <span class="font-mono text-sm">{{ model.model }}</span>
            </div>
          </TableCell>
          <TableCell><Badge variant="outline">{{ t('msTokenBased') }}</Badge></TableCell>
          <TableCell>
            <div class="flex flex-col">
              <span class="font-mono text-sm">
                <template v-if="price(model, 'input')">{{ price(model, 'input') }} / {{ price(model, 'output') }}</template>
                <template v-else>{{ t('pendingConfig') }}</template>
              </span>
              <small class="text-xs text-muted-foreground">/ {{ unitLabel }} tokens</small>
            </div>
          </TableCell>
          <TableCell>
            <div v-if="price(model, 'cache')" class="flex flex-col">
              <span class="font-mono text-sm">{{ price(model, 'cache') }}</span>
              <small class="text-xs text-muted-foreground">/ {{ unitLabel }}</small>
            </div>
            <span v-else class="text-muted-foreground">—</span>
          </TableCell>
          <TableCell>
            <span class="inline-flex rounded-full px-2 py-0.5 text-xs font-medium" :style="{ background: vendorColor(model.vendor_name).bg, color: vendorColor(model.vendor_name).fg }">{{ model.vendor_name }}</span>
          </TableCell>
          <TableCell>
            <div class="flex flex-wrap gap-1">
              <Badge v-for="group in model.groups" :key="group.id" variant="secondary" class="text-[10px]">{{ group.name }}</Badge>
            </div>
          </TableCell>
        </TableRow>
      </TableBody>
    </Table>
    <SquarePagination :page="props.page" :total-pages="props.totalPages" @update:page="emit('update:page', $event)" />
  </div>
</template>

<script setup lang="ts">
import { Plus } from 'lucide-vue-next'
import { useConsoleStore } from '~/composables/useConsoleStore'
import Empty from '~/components/console/Empty.vue'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

const store = useConsoleStore()
const { t, channels, can, showChannel, toggleChannel, editChannel } = store
</script>

<template>
  <section class="flex flex-wrap items-center justify-between gap-4">
    <div>
      <h2 class="text-lg font-semibold">{{ t('upstreamChannels') }}</h2>
      <p class="text-sm text-muted-foreground">{{ t('channelsDesc') }}</p>
    </div>
    <Button v-if="can('channels.manage')" @click="showChannel = true"><Plus :size="16" />{{ t('addChannel') }}</Button>
  </section>
  <section class="mt-4 overflow-hidden rounded-lg border border-border bg-card">
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Status</TableHead>
          <TableHead>Channel name</TableHead>
          <TableHead>Upstream URL</TableHead>
          <TableHead>{{ t('modelLabel') }}</TableHead>
          <TableHead>{{ t('priorityLabel') }}</TableHead>
          <TableHead v-if="can('channels.manage')">{{ t('enableChannelLabel') }}</TableHead>
          <TableHead v-if="can('channels.manage')" />
        </TableRow>
      </TableHeader>
      <TableBody>
        <TableRow v-for="channel in channels" :key="channel.id">
          <TableCell>
            <Badge :variant="channel.enabled ? 'secondary' : 'destructive'">{{ channel.enabled ? t('enabled') : channel.auto_disabled ? t('autoDisabled') : t('disabled') }}</Badge>
            <div v-if="channel.auto_disabled && channel.disabled_reason" class="mt-1 text-xs text-muted-foreground" :title="channel.disabled_reason">{{ channel.disabled_reason }}</div>
          </TableCell>
          <TableCell>
            <div class="font-medium">{{ channel.name }}</div>
            <div class="text-xs text-muted-foreground">{{ channel.provider }}</div>
          </TableCell>
          <TableCell><code class="font-mono text-xs" :title="channel.base_url">{{ channel.base_url }}</code></TableCell>
          <TableCell>
            <div class="flex flex-wrap gap-1">
              <Badge v-for="model in channel.models" :key="model" variant="outline" class="font-mono">{{ model }}</Badge>
            </div>
          </TableCell>
          <TableCell>{{ channel.priority }}</TableCell>
          <TableCell v-if="can('channels.manage')">
            <Switch :model-value="channel.enabled" :aria-label="channel.enabled ? t('disableChannelLabel') : t('enableChannelLabel')" @update:model-value="toggleChannel(channel)" />
          </TableCell>
          <TableCell v-if="can('channels.manage')" class="text-right">
            <Button variant="link" size="sm" @click="editChannel(channel)">{{ t('edit') }}</Button>
          </TableCell>
        </TableRow>
      </TableBody>
    </Table>
    <Empty v-if="!channels.length" :text="t('addOpenAICompatibleUpstream')" />
  </section>
</template>

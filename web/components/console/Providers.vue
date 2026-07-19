<script setup lang="ts">
import { Plus } from 'lucide-vue-next'
import { useConsoleStore } from '~/composables/useConsoleStore'
import Empty from '~/components/console/Empty.vue'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

const store = useConsoleStore()
const { t, providers, can, openProvider, editProvider, removeProvider } = store
</script>

<template>
  <section class="flex flex-wrap items-center justify-between gap-4">
    <div>
      <h2 class="text-lg font-semibold">{{ t('modelProviders') }}</h2>
      <p class="text-sm text-muted-foreground">{{ t('providersDesc') }}</p>
    </div>
    <Button v-if="can('system.manage')" @click="openProvider"><Plus :size="16" />{{ t('addProvider') }}</Button>
  </section>
  <section class="mt-4 overflow-hidden rounded-lg border border-border bg-card">
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{{ t('supplier') }}</TableHead>
          <TableHead>{{ t('modelPrefix') }}</TableHead>
          <TableHead>{{ t('iconSlug') }}</TableHead>
          <TableHead>{{ t('matchPriority') }}</TableHead>
          <TableHead v-if="can('system.manage')">{{ t('actions') }}</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        <TableRow v-for="provider in providers" :key="provider.id">
          <TableCell class="font-medium">{{ provider.name }}</TableCell>
          <TableCell>
            <div class="flex flex-wrap gap-1">
              <Badge v-for="prefix in provider.prefixes" :key="prefix" variant="outline" class="font-mono">{{ prefix }}</Badge>
            </div>
          </TableCell>
          <TableCell><code class="font-mono text-xs">{{ provider.slug }}</code></TableCell>
          <TableCell>{{ provider.priority }}</TableCell>
          <TableCell v-if="can('system.manage')" class="text-right">
            <Button variant="link" size="sm" type="button" @click="editProvider(provider)">{{ t('edit') }}</Button>
            <Button variant="link" size="sm" type="button" class="text-destructive" @click="removeProvider(provider)">{{ t('remove') }}</Button>
          </TableCell>
        </TableRow>
      </TableBody>
    </Table>
    <Empty v-if="!providers.length" :text="t('noProvidersYet')" />
  </section>
</template>

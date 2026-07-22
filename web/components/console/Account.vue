<script setup lang="ts">
import { Plus } from 'lucide-vue-next'
import { useConsoleStore } from '~/composables/useConsoleStore'
import Empty from '~/components/console/Empty.vue'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

const store = useConsoleStore()
const { t, accountKeys, showAccountKey, formatDate, editAccountKey, revokeAccountKey } = store
</script>

<template>
  <section class="flex flex-wrap items-center justify-between gap-4">
    <div>
      <h2 class="text-lg font-semibold">{{ t('account') }}</h2>
      <p class="text-sm text-muted-foreground">{{ t('keyBelongsToAccount') }}</p>
    </div>
    <Button @click="showAccountKey = true"><Plus :size="16" />{{ t('createKeyButton') }}</Button>
  </section>
  <section class="mt-4 overflow-hidden rounded-lg border border-border bg-card">
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{{ t('keyName') }}</TableHead>
          <TableHead>{{ t('keyPrefix') }}</TableHead>
          <TableHead>{{ t('createdAt') }}</TableHead>
          <TableHead>{{ t('lastUsed') }}</TableHead>
          <TableHead>{{ t('accountStatus') }}</TableHead>
          <TableHead />
        </TableRow>
      </TableHeader>
      <TableBody>
        <TableRow v-for="key in accountKeys" :key="key.id">
          <TableCell class="font-medium">{{ key.name }}</TableCell>
          <TableCell><code class="font-mono text-xs">{{ key.key_prefix }}...</code></TableCell>
          <TableCell>{{ formatDate(key.created_at) }}</TableCell>
          <TableCell>{{ formatDate(key.last_used_at) }}</TableCell>
          <TableCell>
            <Badge :variant="key.revoked_at ? 'destructive' : 'secondary'">{{ key.revoked_at ? t('revoked') : t('valid') }}</Badge>
          </TableCell>
          <TableCell class="text-right">
            <Button v-if="!key.revoked_at" variant="link" size="sm" @click="editAccountKey(key)">{{ t('edit') }}</Button>
            <Button v-if="!key.revoked_at" variant="link" size="sm" class="text-destructive" @click="revokeAccountKey(key)">{{ t('revokeLabel') }}</Button>
          </TableCell>
        </TableRow>
      </TableBody>
    </Table>
    <Empty v-if="!accountKeys.length" :text="t('noApiKeysYet')" />
  </section>
</template>
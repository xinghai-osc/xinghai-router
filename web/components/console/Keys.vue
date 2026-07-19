<script setup lang="ts">
import { Plus } from 'lucide-vue-next'
import { useConsoleStore } from '~/composables/useConsoleStore'
import Empty from '~/components/console/Empty.vue'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

const store = useConsoleStore()
const { t, keys, users, showKey, userName, formatDate, revokeKey } = store
</script>

<template>
  <section class="flex flex-wrap items-center justify-between gap-4">
    <div>
      <h2 class="text-lg font-semibold">{{ t('keys') }}</h2>
      <p class="text-sm text-muted-foreground">{{ t('showOnceAfterCreation') }}</p>
    </div>
    <Button :disabled="!users.length" @click="showKey = true"><Plus :size="16" />{{ t('createKeyButton') }}</Button>
  </section>
  <section class="mt-4 overflow-hidden rounded-lg border border-border bg-card">
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{{ t('keyName') }}</TableHead>
          <TableHead>{{ t('userLabel') }}</TableHead>
          <TableHead>{{ t('keyPrefix') }}</TableHead>
          <TableHead>{{ t('lastUsed') }}</TableHead>
          <TableHead>{{ t('accountStatus') }}</TableHead>
          <TableHead />
        </TableRow>
      </TableHeader>
      <TableBody>
        <TableRow v-for="key in keys" :key="key.id">
          <TableCell class="font-medium">{{ key.name }}</TableCell>
          <TableCell>{{ userName(key.user_id) }}</TableCell>
          <TableCell><code class="font-mono text-xs">{{ key.key_prefix }}...</code></TableCell>
          <TableCell>{{ formatDate(key.last_used_at) }}</TableCell>
          <TableCell>
            <Badge :variant="key.revoked_at ? 'destructive' : 'secondary'">{{ key.revoked_at ? t('revoked') : t('valid') }}</Badge>
          </TableCell>
          <TableCell class="text-right">
            <Button v-if="!key.revoked_at" variant="link" size="sm" class="text-destructive" @click="revokeKey(key)">{{ t('revokeLabel') }}</Button>
          </TableCell>
        </TableRow>
      </TableBody>
    </Table>
    <Empty v-if="!keys.length" :text="t('createUserThenIssueKey')" />
  </section>
</template>

<script setup lang="ts">
import { useConsoleStore } from '~/composables/useConsoleStore'
import Empty from '~/components/console/Empty.vue'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

const store = useConsoleStore()
const { t, users, can, manageUser } = store
</script>

<template>
  <section class="flex flex-wrap items-center justify-between gap-4">
    <div>
      <h2 class="text-lg font-semibold">{{ t('usersAndPermissions') }}</h2>
      <p class="text-sm text-muted-foreground">{{ t('usersPermissionDesc') }}</p>
    </div>
  </section>
  <section class="mt-4 overflow-hidden rounded-lg border border-border bg-card">
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{{ t('userLabel') }}</TableHead>
          <TableHead>{{ t('roleLabel') }}</TableHead>
          <TableHead>{{ t('groupLabel') }}</TableHead>
          <TableHead>{{ t('balanceLabel') }}</TableHead>
          <TableHead>{{ t('permissionScope') }}</TableHead>
          <TableHead>{{ t('accountStatus') }}</TableHead>
          <TableHead />
        </TableRow>
      </TableHeader>
      <TableBody>
        <TableRow v-for="user in users" :key="user.id">
          <TableCell>
            <div class="font-medium">{{ user.name }}</div>
            <div class="text-xs text-muted-foreground">{{ user.email }}</div>
          </TableCell>
          <TableCell><Badge variant="outline">{{ user.role }}</Badge></TableCell>
          <TableCell>{{ user.groups.length || t('none') }}</TableCell>
          <TableCell>
            {{ Number(user.balance ?? 0).toFixed(4) }}
            <span v-if="Number(user.reserved ?? 0)" class="text-xs text-muted-foreground">{{ t('reserved') }} {{ Number(user.reserved).toFixed(4) }}</span>
          </TableCell>
          <TableCell class="text-xs">{{ user.role === 'admin' ? t('allPermissions') : user.permissions.join(', ') || t('none') }}</TableCell>
          <TableCell>
            <Badge :variant="user.enabled ? 'secondary' : 'destructive'">{{ user.enabled ? t('enabled') : t('disabled') }}</Badge>
          </TableCell>
          <TableCell class="text-right">
            <Button v-if="can('system.manage')" variant="link" size="sm" @click="manageUser(user)">{{ t('edit') }}</Button>
          </TableCell>
        </TableRow>
      </TableBody>
    </Table>
    <Empty v-if="!users.length" :text="t('noUsersYet')" />
  </section>
</template>

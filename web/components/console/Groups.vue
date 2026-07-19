<script setup lang="ts">
import { Plus } from 'lucide-vue-next'
import { useConsoleStore } from '~/composables/useConsoleStore'
import Empty from '~/components/console/Empty.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

const store = useConsoleStore()
const { t, groups, groupForm, groupImportText, busy, createGroup, editGroupMultiplier, importGroups, formatDate } = store
</script>

<template>
  <section class="flex flex-wrap items-center justify-between gap-4">
    <div>
      <h2 class="text-lg font-semibold">{{ t('accessGroups') }}</h2>
      <p class="text-sm text-muted-foreground">{{ t('groupsDesc') }}</p>
    </div>
  </section>
  <div class="mt-4 grid gap-4 lg:grid-cols-2">
    <Card>
      <CardHeader>
        <CardTitle>{{ t('batchImportGroups') }}</CardTitle>
        <CardDescription>{{ t('importGroupsDesc') }}</CardDescription>
      </CardHeader>
      <CardContent class="flex flex-col gap-4">
        <Textarea v-model="groupImportText" required :placeholder="t('importJsonPlaceholder')" rows="6" />
        <Button :disabled="busy" @click="importGroups"><Plus :size="16" />{{ t('importOneClick') }}</Button>
      </CardContent>
    </Card>
    <Card>
      <CardHeader>
        <CardTitle>{{ t('createGroupTitle') }}</CardTitle>
        <CardDescription>{{ t('createGroupDesc') }}</CardDescription>
      </CardHeader>
      <CardContent class="flex flex-col gap-4">
        <form class="flex flex-col gap-4" @submit.prevent="createGroup">
          <div class="flex flex-col gap-2">
            <Label for="group-name">{{ t('groupNameLabel') }}</Label>
            <Input id="group-name" v-model="groupForm.name" required maxlength="100" :placeholder="t('groupNamePlaceholder')" />
          </div>
          <div class="flex flex-col gap-2">
            <Label for="group-mult">{{ t('multiplierLabel') }}</Label>
            <Input id="group-mult" v-model.number="groupForm.multiplier" required type="number" min="0" step="0.01" />
          </div>
          <Button type="submit" :disabled="busy"><Plus :size="16" />{{ t('createGroupButton') }}</Button>
        </form>
      </CardContent>
    </Card>
  </div>
  <section class="mt-4 overflow-hidden rounded-lg border border-border bg-card">
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{{ t('groupNameLabel') }}</TableHead>
          <TableHead>{{ t('createdAt') }}</TableHead>
          <TableHead>{{ t('multiplierLabel') }}</TableHead>
          <TableHead />
        </TableRow>
      </TableHeader>
      <TableBody>
        <TableRow v-for="group in groups" :key="group.id">
          <TableCell>
            <div class="font-medium">{{ group.name }}</div>
            <div class="text-xs text-muted-foreground font-mono">{{ group.id }}</div>
          </TableCell>
          <TableCell>{{ formatDate(group.created_at) }}</TableCell>
          <TableCell>
            <form class="flex items-center gap-2" @submit.prevent="editGroupMultiplier(group, $event)">
              <Input name="multiplier" :value="Number(group.multiplier)" :aria-label="t('multiplierLabel')" required type="number" min="0" step="0.01" class="w-24" />
              <span class="text-sm text-muted-foreground">×</span>
              <Button type="submit" variant="ghost" size="sm" :disabled="busy">{{ t('saveLabel') }}</Button>
            </form>
          </TableCell>
          <TableCell />
        </TableRow>
      </TableBody>
    </Table>
    <Empty v-if="!groups.length" :text="t('noGroupsYet')" />
  </section>
</template>

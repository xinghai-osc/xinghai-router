<script setup lang="ts">
import { Copy } from 'lucide-vue-next'
import { useConsoleStore } from '~/composables/useConsoleStore'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Sheet, SheetContent, SheetFooter, SheetHeader, SheetTitle } from '@/components/ui/sheet'

const store = useConsoleStore()
const {
  t, busy, users, groups, ownGroups, permissions,
  selectedUser, originalUser, showKey, showAccountKey, editingAccountKey,
  showChannel, editingChannel, showProvider, createdKey,
  keyForm, accountKeyForm, channelForm, providerForm,
  selectedPermissions, selectedGroups, userPassword, userBalance, userBalanceNote,
  saveUserAccess, createKey, createAccountKey, updateAccountKey,
  createChannel, updateChannel, fetchChannelModels,
  saveProvider, copyKey,
} = store

const selectClass = 'flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring'
const selectMultiClass = 'flex min-h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring'
</script>

<template>
  <Dialog :open="Boolean(selectedUser)" @update:open="v => !v && ((selectedUser = null) || (originalUser = null))">
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>{{ t('editUser') }}</DialogTitle>
        <DialogDescription class="font-mono">{{ selectedUser?.id }}</DialogDescription>
      </DialogHeader>
      <form v-if="selectedUser" class="grid gap-4" @submit.prevent="saveUserAccess">
        <div class="flex flex-col gap-2">
          <Label>{{ t('nameLabel') }}</Label>
          <Input v-model="selectedUser.name" required maxlength="100" />
        </div>
        <div class="flex flex-col gap-2">
          <Label>{{ t('emailLabel') }}</Label>
          <Input v-model="selectedUser.email" required type="email" />
        </div>
        <div class="flex flex-col gap-2">
          <Label>{{ t('newPassword') }} <span class="text-xs text-muted-foreground">{{ t('leaveEmptyToKeep') }}</span></Label>
          <Input v-model="userPassword" type="password" minlength="8" autocomplete="new-password" />
        </div>
        <div class="grid grid-cols-2 gap-4">
          <div class="flex flex-col gap-2">
            <Label>{{ t('accountStatus') }}</Label>
            <select v-model="selectedUser.enabled" :class="selectClass">
              <option :value="true">{{ t('enabled') }}</option>
              <option :value="false">{{ t('disabled') }}</option>
            </select>
          </div>
          <div class="flex flex-col gap-2">
            <Label>{{ t('roleLabel') }}</Label>
            <select v-model="selectedUser.role" :class="selectClass">
              <option value="user">{{ t('userRole') }}</option>
              <option value="operator">{{ t('operatorRole') }}</option>
              <option value="admin">{{ t('adminRoleFull') }}</option>
            </select>
          </div>
        </div>
        <div class="flex flex-col gap-2">
          <Label>{{ t('balanceLabel') }}</Label>
          <Input v-model.number="userBalance" required type="number" min="0" step="0.00000001" />
        </div>
        <div class="flex flex-col gap-2">
          <Label>{{ t('balanceChangeNote') }}</Label>
          <Input v-model="userBalanceNote" maxlength="200" :placeholder="t('balanceNotePlaceholder')" />
        </div>
        <div class="flex flex-col gap-2">
          <Label>{{ t('userGroupsLabel') }}</Label>
          <select v-model="selectedGroups" multiple size="5" :class="selectMultiClass">
            <option v-for="group in groups" :key="group.id" :value="group.id">{{ group.name }} · {{ Number(group.multiplier).toFixed(2) }}x</option>
          </select>
        </div>
        <div v-if="selectedUser.role !== 'admin'" class="flex flex-col gap-2">
          <Label>{{ t('granularPermissions') }}</Label>
          <select v-model="selectedPermissions" multiple size="8" :class="selectMultiClass">
            <option v-for="permission in permissions" :key="permission" :value="permission">{{ permission }}</option>
          </select>
        </div>
        <DialogFooter>
          <Button type="submit" :disabled="busy" class="w-full">{{ t('saveChanges') }}</Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>

  <Dialog :open="showKey" @update:open="v => !v && (showKey = false)">
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>{{ t('createApiKeyTitle') }}</DialogTitle>
      </DialogHeader>
      <form class="grid gap-4" @submit.prevent="createKey">
        <div class="flex flex-col gap-2">
          <Label>{{ t('userLabel') }}</Label>
          <select v-model="keyForm.user_id" required :class="selectClass">
            <option disabled value="">{{ t('selectUser') }}</option>
            <option v-for="user in users" :key="user.id" :value="user.id">{{ user.name }} · {{ user.email }}</option>
          </select>
        </div>
        <div class="flex flex-col gap-2">
          <Label>{{ t('useGroup') }}</Label>
          <select v-model="keyForm.group_id" :class="selectClass">
            <option value="">{{ t('autoMatch') }}</option>
            <option v-for="group in groups.filter((item) => users.find((user) => user.id === keyForm.user_id)?.groups.includes(item.id))" :key="group.id" :value="group.id">{{ group.name }} · {{ Number(group.multiplier).toFixed(2) }}x</option>
          </select>
        </div>
        <div class="flex flex-col gap-2">
          <Label>{{ t('keyName') }}</Label>
          <Input v-model="keyForm.name" required :placeholder="t('keyNamePlaceholder')" />
        </div>
        <div class="flex flex-col gap-2">
          <Label>{{ t('expiresAt') }} <span class="text-xs text-muted-foreground">{{ t('optional') }}</span></Label>
          <Input v-model="keyForm.expires_at" type="datetime-local" />
        </div>
        <DialogFooter>
          <Button type="submit" :disabled="busy" class="w-full">{{ t('issueKey') }}</Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>

  <Dialog :open="showAccountKey || Boolean(editingAccountKey)" @update:open="v => !v && ((showAccountKey = false) || (editingAccountKey = null))">
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>{{ editingAccountKey ? t('editApiKey') : t('createApiKeyTitle') }}</DialogTitle>
        <DialogDescription v-if="!editingAccountKey">{{ t('keyBelongsToAccount') }}</DialogDescription>
      </DialogHeader>
      <form class="grid gap-4" @submit.prevent="editingAccountKey ? updateAccountKey() : createAccountKey()">
        <div class="flex flex-col gap-2">
          <Label>{{ t('useGroup') }}</Label>
          <select v-model="accountKeyForm.group_id" :class="selectClass">
            <option value="">{{ t('autoMatch') }}</option>
            <option v-for="group in groups.filter((item) => ownGroups.includes(item.name))" :key="group.id" :value="group.id">{{ group.name }} · {{ Number(group.multiplier).toFixed(2) }}x</option>
          </select>
        </div>
        <div class="flex flex-col gap-2">
          <Label>{{ t('keyName') }}</Label>
          <Input v-model="accountKeyForm.name" required maxlength="100" :placeholder="t('keyNamePlaceholder2')" />
        </div>
        <div class="flex flex-col gap-2">
          <Label>{{ t('expiresAt') }} <span class="text-xs text-muted-foreground">{{ t('optional') }}</span></Label>
          <Input v-model="accountKeyForm.expires_at" type="datetime-local" />
        </div>
        <DialogFooter>
          <Button type="submit" :disabled="busy" class="w-full">{{ editingAccountKey ? t('saveChanges') : t('createKeyButton') }}</Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>

  <Sheet :open="showChannel || Boolean(editingChannel)" @update:open="v => !v && ((showChannel = false) || (editingChannel = null))">
    <SheetContent side="right" class="w-full sm:max-w-lg">
      <SheetHeader>
        <SheetTitle>{{ editingChannel ? t('editChannel') : t('addChannel') }}</SheetTitle>
      </SheetHeader>
      <form class="flex flex-1 flex-col overflow-y-auto px-6" @submit.prevent="editingChannel ? updateChannel() : createChannel()">
        <div class="flex flex-col gap-5">
          <div class="flex flex-col gap-2">
            <Label>{{ t('channelNameLabel') }}</Label>
            <Input v-model="channelForm.name" required maxlength="100" />
          </div>
          <div class="flex flex-col gap-2">
            <Label>{{ t('upstreamProtocol') }}</Label>
            <select v-model="channelForm.provider" :class="selectClass">
              <option value="openai">OpenAI</option>
              <option value="anthropic">Anthropic</option>
              <option value="ollama">Ollama</option>
              <option value="kimi">Kimi</option>
              <option value="opencode_go">OpenCode Go</option>
            </select>
          </div>
          <div class="flex flex-col gap-2">
            <Label>{{ t('upstreamURL') }}</Label>
            <Input v-model="channelForm.base_url" required type="url" />
          </div>
          <div class="flex flex-col gap-2">
            <Label>{{ t('apiKeyLabel') }} <span class="text-xs text-muted-foreground">{{ editingChannel ? t('leaveBlankUnchanged') : t('requiredField') }}</span></Label>
            <Input v-model="channelForm.api_key" :required="!editingChannel" type="password" autocomplete="new-password" />
          </div>
          <div class="flex flex-col gap-2">
            <Label>{{ t('modelLabel') }} <span class="text-xs text-muted-foreground">{{ t('modelsCommaSeparated') }}</span></Label>
            <Input v-model="channelForm.models" required />
          </div>
          <Button variant="link" type="button" class="w-fit px-0" :disabled="busy || !channelForm.api_key" @click="fetchChannelModels">{{ t('fetchUpstreamModels') }}</Button>
          <div class="flex flex-col gap-2">
            <Label>{{ t('priorityLabel') }}</Label>
            <Input v-model.number="channelForm.priority" required type="number" min="0" />
          </div>
          <div class="flex flex-col gap-2">
            <Label>{{ t('availableGroups') }}</Label>
            <select v-model="channelForm.groups" multiple size="5" :class="selectMultiClass">
              <option v-for="group in groups" :key="group.id" :value="group.id">{{ group.name }} · {{ Number(group.multiplier).toFixed(2) }}x</option>
            </select>
          </div>
        </div>
        <SheetFooter class="mt-auto px-0 pb-6">
          <Button type="submit" :disabled="busy" class="w-full">{{ editingChannel ? t('saveChanges') : t('addChannel') }}</Button>
        </SheetFooter>
      </form>
    </SheetContent>
  </Sheet>

  <Dialog :open="showProvider" @update:open="v => !v && (showProvider = false)">
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>{{ editingProviderID ? t('editProvider') : t('configureProvider') }}</DialogTitle>
        <DialogDescription>{{ t('providerDesc') }}</DialogDescription>
      </DialogHeader>
      <form class="grid gap-4" @submit.prevent="saveProvider">
        <div class="flex flex-col gap-2">
          <Label>{{ t('providerName') }}</Label>
          <Input v-model="providerForm.name" required :placeholder="t('providerNamePlaceholder')" />
        </div>
        <div class="flex flex-col gap-2">
          <Label>{{ t('iconSlug') }}</Label>
          <Input v-model="providerForm.slug" required :placeholder="t('iconSlugPlaceholder')" />
        </div>
        <div class="flex flex-col gap-2">
          <Label>{{ t('modelPrefix') }}</Label>
          <Input v-model="providerForm.prefixes" required :placeholder="t('modelPrefixPlaceholder')" />
        </div>
        <div class="flex flex-col gap-2">
          <Label>{{ t('matchPriority') }}</Label>
          <Input v-model.number="providerForm.priority" required type="number" min="0" />
        </div>
        <DialogFooter>
          <Button type="submit" :disabled="busy" class="w-full">{{ t('saveProviderButton') }}</Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>

  <Dialog :open="Boolean(createdKey)" @update:open="v => !v && (createdKey = '')">
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>{{ t('saveApiKey') }}</DialogTitle>
        <DialogDescription>{{ t('saveKeyWarning') }}</DialogDescription>
      </DialogHeader>
      <div class="flex flex-col gap-4">
        <code class="break-all rounded-md bg-muted p-3 font-mono text-sm">{{ createdKey }}</code>
        <Button class="w-full" @click="copyKey"><Copy :size="16" />{{ t('copyKeyButton') }}</Button>
        <Button variant="outline" class="w-full" @click="createdKey = ''">{{ t('iHaveSaved') }}</Button>
      </div>
    </DialogContent>
  </Dialog>
</template>

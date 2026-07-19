<script setup lang="ts">
import { Copy } from 'lucide-vue-next'
import { useConsoleStore } from '~/composables/useConsoleStore'

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

function closeAll() {
  selectedUser.value = null
  originalUser.value = null
  editingAccountKey.value = null
  editingChannel.value = null
  showKey.value = false
  showAccountKey.value = false
  showChannel.value = false
  showProvider.value = false
}

const open = computed(() => Boolean(selectedUser.value || showKey.value || showAccountKey.value || editingAccountKey.value || showChannel.value || editingChannel.value || showProvider.value || createdKey.value))
</script>

<template>
  <div v-if="open" class="modal-backdrop" @click.self="closeAll">
    <form v-if="selectedUser" class="modal" @submit.prevent="saveUserAccess">
      <div class="modal-title">
        <h2>{{ t('editUser') }}</h2>
        <button type="button" @click="selectedUser = null; originalUser = null">×</button>
      </div>
      <p class="muted">{{ selectedUser.id }}</p>
      <label>{{ t('nameLabel') }}<input v-model="selectedUser.name" required maxlength="100" /></label>
      <label>{{ t('emailLabel') }}<input v-model="selectedUser.email" required type="email" /></label>
      <label>{{ t('newPassword') }} <small>{{ t('leaveEmptyToKeep') }}</small><input v-model="userPassword" type="password" minlength="8" autocomplete="new-password" /></label>
      <label>{{ t('accountStatus') }}
        <select v-model="selectedUser.enabled">
          <option :value="true">{{ t('enabled') }}</option>
          <option :value="false">{{ t('disabled') }}</option>
        </select>
      </label>
      <label>{{ t('roleLabel') }}
        <select v-model="selectedUser.role">
          <option value="user">{{ t('userRole') }}</option>
          <option value="operator">{{ t('operatorRole') }}</option>
          <option value="admin">{{ t('adminRoleFull') }}</option>
        </select>
      </label>
      <label>{{ t('balanceLabel') }}<input v-model.number="userBalance" required type="number" min="0" step="0.00000001" /></label>
      <label>{{ t('balanceChangeNote') }}<input v-model="userBalanceNote" maxlength="200" :placeholder="t('balanceNotePlaceholder')" /></label>
      <label>{{ t('userGroupsLabel') }}
        <select v-model="selectedGroups" multiple size="5">
          <option v-for="group in groups" :key="group.id" :value="group.id">{{ group.name }} · {{ Number(group.multiplier).toFixed(2) }}x</option>
        </select>
      </label>
      <label v-if="selectedUser.role !== 'admin'">{{ t('granularPermissions') }}
        <select v-model="selectedPermissions" multiple size="8">
          <option v-for="permission in permissions" :key="permission" :value="permission">{{ permission }}</option>
        </select>
      </label>
      <button class="button primary full" :disabled="busy">{{ t('saveChanges') }}</button>
    </form>

    <form v-if="showKey" class="modal" @submit.prevent="createKey">
      <div class="modal-title">
        <h2>{{ t('createApiKeyTitle') }}</h2>
        <button type="button" @click="showKey = false">×</button>
      </div>
      <label>{{ t('userLabel') }}
        <select v-model="keyForm.user_id" required>
          <option disabled value="">{{ t('selectUser') }}</option>
          <option v-for="user in users" :key="user.id" :value="user.id">{{ user.name }} · {{ user.email }}</option>
        </select>
      </label>
      <label>{{ t('useGroup') }}
        <select v-model="keyForm.group_id">
          <option value="">{{ t('autoMatch') }}</option>
          <option v-for="group in groups.filter((item) => users.find((user) => user.id === keyForm.user_id)?.groups.includes(item.id))" :key="group.id" :value="group.id">{{ group.name }} · {{ Number(group.multiplier).toFixed(2) }}x</option>
        </select>
      </label>
      <label>{{ t('keyName') }}<input v-model="keyForm.name" required :placeholder="t('keyNamePlaceholder')" /></label>
      <label>{{ t('expiresAt') }} <small>{{ t('optional') }}</small><input v-model="keyForm.expires_at" type="datetime-local" /></label>
      <button class="button primary full" :disabled="busy">{{ t('issueKey') }}</button>
    </form>

    <form v-if="showAccountKey || editingAccountKey" class="modal" @submit.prevent="editingAccountKey ? updateAccountKey() : createAccountKey()">
      <div class="modal-title">
        <h2>{{ editingAccountKey ? t('editApiKey') : t('createApiKeyTitle') }}</h2>
        <button type="button" @click="showAccountKey = false; editingAccountKey = null">×</button>
      </div>
      <p v-if="!editingAccountKey" class="muted">{{ t('keyBelongsToAccount') }}</p>
      <label>{{ t('useGroup') }}
        <select v-model="accountKeyForm.group_id">
          <option value="">{{ t('autoMatch') }}</option>
          <option v-for="group in groups.filter((item) => ownGroups.includes(item.name))" :key="group.id" :value="group.id">{{ group.name }} · {{ Number(group.multiplier).toFixed(2) }}x</option>
        </select>
      </label>
      <label>{{ t('keyName') }}<input v-model="accountKeyForm.name" required maxlength="100" :placeholder="t('keyNamePlaceholder2')" /></label>
      <label>{{ t('expiresAt') }} <small>{{ t('optional') }}</small><input v-model="accountKeyForm.expires_at" type="datetime-local" /></label>
      <button class="button primary full" :disabled="busy">{{ editingAccountKey ? t('saveChanges') : t('createKeyButton') }}</button>
    </form>

    <form v-if="showChannel || editingChannel" class="modal" @submit.prevent="editingChannel ? updateChannel() : createChannel()">
      <div class="modal-title">
        <h2>{{ editingChannel ? t('editChannel') : t('addChannel') }}</h2>
        <button type="button" @click="showChannel = false; editingChannel = null">×</button>
      </div>
      <label>{{ t('channelNameLabel') }}<input v-model="channelForm.name" required maxlength="100" /></label>
      <label>{{ t('upstreamProtocol') }}
        <select v-model="channelForm.provider">
          <option value="openai">OpenAI</option>
          <option value="anthropic">Anthropic</option>
          <option value="ollama">Ollama</option>
          <option value="kimi">Kimi</option>
          <option value="opencode_go">OpenCode Go</option>
        </select>
      </label>
      <label>{{ t('upstreamURL') }}<input v-model="channelForm.base_url" required type="url" /></label>
      <label>{{ t('apiKeyLabel') }} <small>{{ editingChannel ? t('leaveBlankUnchanged') : t('requiredField') }}</small><input v-model="channelForm.api_key" :required="!editingChannel" type="password" autocomplete="new-password" /></label>
      <label>{{ t('modelLabel') }} <small>{{ t('modelsCommaSeparated') }}</small><input v-model="channelForm.models" required /></label>
      <button class="text-button" type="button" :disabled="busy || !channelForm.api_key" @click="fetchChannelModels">{{ t('fetchUpstreamModels') }}</button>
      <label>{{ t('priorityLabel') }}<input v-model.number="channelForm.priority" required type="number" min="0" /></label>
      <label>{{ t('availableGroups') }}
        <select v-model="channelForm.groups" multiple size="5">
          <option v-for="group in groups" :key="group.id" :value="group.id">{{ group.name }} · {{ Number(group.multiplier).toFixed(2) }}x</option>
        </select>
      </label>
      <button class="button primary full" :disabled="busy">{{ editingChannel ? t('saveChanges') : t('addChannel') }}</button>
    </form>

    <form v-if="showProvider" class="modal" @submit.prevent="saveProvider">
      <div class="modal-title">
        <h2>{{ editingProviderID ? t('editProvider') : t('configureProvider') }}</h2>
        <button type="button" @click="showProvider = false">×</button>
      </div>
      <p class="muted">{{ t('providerDesc') }}</p>
      <label>{{ t('providerName') }}<input v-model="providerForm.name" required :placeholder="t('providerNamePlaceholder')" /></label>
      <label>{{ t('iconSlug') }}<input v-model="providerForm.slug" required :placeholder="t('iconSlugPlaceholder')" /></label>
      <label>{{ t('modelPrefix') }}<input v-model="providerForm.prefixes" required :placeholder="t('modelPrefixPlaceholder')" /></label>
      <label>{{ t('matchPriority') }}<input v-model.number="providerForm.priority" required type="number" min="0" /></label>
      <button class="button primary full" :disabled="busy">{{ t('saveProviderButton') }}</button>
    </form>

    <section v-if="createdKey" class="modal secret">
      <div class="modal-title">
        <h2>{{ t('saveApiKey') }}</h2>
        <button @click="createdKey = ''">×</button>
      </div>
      <p>{{ t('saveKeyWarning') }}</p>
      <code>{{ createdKey }}</code>
      <button class="button primary full" @click="copyKey"><Copy :size="16" />{{ t('copyKeyButton') }}</button>
      <button class="button ghost full" @click="createdKey = ''">{{ t('iHaveSaved') }}</button>
    </section>
  </div>
</template>

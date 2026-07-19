<script setup lang="ts">
import { Plus } from 'lucide-vue-next'
import { useConsoleStore } from '~/composables/useConsoleStore'
import Empty from '~/components/console/Empty.vue'

const store = useConsoleStore()
const { t, keys, users, showKey, busy, userName, formatDate, revokeKey } = store
</script>

<template>
  <section class="toolbar">
    <div>
      <h2>{{ t('keys') }}</h2>
      <p>{{ t('showOnceAfterCreation') }}</p>
    </div>
    <button class="button primary" :disabled="!users.length" @click="showKey = true"><Plus :size="16" />{{ t('createKeyButton') }}</button>
  </section>
  <section class="panel table-panel">
    <table>
      <thead>
        <tr>
          <th>{{ t('keyName') }}</th>
          <th>{{ t('userLabel') }}</th>
          <th>{{ t('keyPrefix') }}</th>
          <th>{{ t('lastUsed') }}</th>
          <th>{{ t('accountStatus') }}</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="key in keys" :key="key.id">
          <td><b>{{ key.name }}</b></td>
          <td>{{ userName(key.user_id) }}</td>
          <td><code>{{ key.key_prefix }}...</code></td>
          <td>{{ formatDate(key.last_used_at) }}</td>
          <td><span :class="['state', key.revoked_at ? 'bad' : 'good']">{{ key.revoked_at ? t('revoked') : t('valid') }}</span></td>
          <td><button v-if="!key.revoked_at" class="text-button danger" @click="revokeKey(key)">{{ t('revokeLabel') }}</button></td>
        </tr>
      </tbody>
    </table>
    <Empty v-if="!keys.length" :text="t('createUserThenIssueKey')" />
  </section>
</template>

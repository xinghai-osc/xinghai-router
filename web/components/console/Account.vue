<script setup lang="ts">
import { Plus } from 'lucide-vue-next'
import { useConsoleStore } from '~/composables/useConsoleStore'
import Empty from '~/components/console/Empty.vue'

const store = useConsoleStore()
const { t, accountKeys, showAccountKey, formatDate, editAccountKey } = store
</script>

<template>
  <section class="toolbar">
    <div>
      <h2>{{ t('account') }}</h2>
      <p>{{ t('keyBelongsToAccount') }}</p>
    </div>
    <button class="button primary" @click="showAccountKey = true"><Plus :size="16" />{{ t('createKeyButton') }}</button>
  </section>
  <section class="panel table-panel">
    <table>
      <thead>
        <tr>
          <th>{{ t('keyName') }}</th>
          <th>{{ t('keyPrefix') }}</th>
          <th>{{ t('createdAt') }}</th>
          <th>{{ t('lastUsed') }}</th>
          <th>{{ t('accountStatus') }}</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="key in accountKeys" :key="key.id">
          <td><b>{{ key.name }}</b></td>
          <td><code>{{ key.key_prefix }}...</code></td>
          <td>{{ formatDate(key.created_at) }}</td>
          <td>{{ formatDate(key.last_used_at) }}</td>
          <td><span :class="['state', key.revoked_at ? 'bad' : 'good']">{{ key.revoked_at ? t('revoked') : t('valid') }}</span></td>
          <td><button v-if="!key.revoked_at" class="text-button" @click="editAccountKey(key)">{{ t('edit') }}</button></td>
        </tr>
      </tbody>
    </table>
    <Empty v-if="!accountKeys.length" :text="t('noApiKeysYet')" />
  </section>
</template>

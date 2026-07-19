<script setup lang="ts">
import { Plus } from 'lucide-vue-next'
import { useConsoleStore } from '~/composables/useConsoleStore'
import Empty from '~/components/console/Empty.vue'

const store = useConsoleStore()
const { t, providers, can, openProvider, editProvider, removeProvider } = store
</script>

<template>
  <section class="toolbar">
    <div>
      <h2>{{ t('modelProviders') }}</h2>
      <p>{{ t('providersDesc') }}</p>
    </div>
    <button v-if="can('system.manage')" class="button primary" @click="openProvider"><Plus :size="16" />{{ t('addProvider') }}</button>
  </section>
  <section class="panel table-panel">
    <table>
      <thead>
        <tr>
          <th>{{ t('supplier') }}</th>
          <th>{{ t('modelPrefix') }}</th>
          <th>{{ t('iconSlug') }}</th>
          <th>{{ t('matchPriority') }}</th>
          <th v-if="can('system.manage')">{{ t('actions') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="provider in providers" :key="provider.id">
          <td><b>{{ provider.name }}</b></td>
          <td><span v-for="prefix in provider.prefixes" :key="prefix" class="pill">{{ prefix }}</span></td>
          <td><code>{{ provider.slug }}</code></td>
          <td>{{ provider.priority }}</td>
          <td v-if="can('system.manage')">
            <button class="text-button" type="button" @click="editProvider(provider)">{{ t('edit') }}</button>
            <button class="text-button danger" type="button" @click="removeProvider(provider)">{{ t('remove') }}</button>
          </td>
        </tr>
      </tbody>
    </table>
    <Empty v-if="!providers.length" :text="t('noProvidersYet')" />
  </section>
</template>

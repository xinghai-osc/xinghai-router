<script setup lang="ts">
import { Plus } from 'lucide-vue-next'
import { useConsoleStore } from '~/composables/useConsoleStore'
import Empty from '~/components/console/Empty.vue'

const store = useConsoleStore()
const { t, groups, groupForm, groupImportText, busy, createGroup, editGroupMultiplier, importGroups, formatDate } = store
</script>

<template>
  <section class="toolbar">
    <div>
      <h2>{{ t('accessGroups') }}</h2>
      <p>{{ t('groupsDesc') }}</p>
    </div>
  </section>
  <div class="group-page">
    <form class="panel group-import-form" @submit.prevent="importGroups">
      <div>
        <h3>{{ t('batchImportGroups') }}</h3>
        <p>{{ t('importGroupsDesc') }}</p>
      </div>
      <textarea v-model="groupImportText" required :placeholder="t('importJsonPlaceholder')"></textarea>
      <button class="button primary" :disabled="busy"><Plus :size="16" />{{ t('importOneClick') }}</button>
    </form>
    <form class="panel group-create-form" @submit.prevent="createGroup">
      <div>
        <h3>{{ t('createGroupTitle') }}</h3>
        <p>{{ t('createGroupDesc') }}</p>
      </div>
      <label>{{ t('groupNameLabel') }}<input v-model="groupForm.name" required maxlength="100" :placeholder="t('groupNamePlaceholder')" /></label>
      <label>{{ t('multiplierLabel') }}<input v-model.number="groupForm.multiplier" required type="number" min="0" step="0.01" /></label>
      <button class="button primary" :disabled="busy"><Plus :size="16" />{{ t('createGroupButton') }}</button>
    </form>
    <section class="panel table-panel">
      <table>
        <thead>
          <tr>
            <th>{{ t('groupNameLabel') }}</th>
            <th>{{ t('createdAt') }}</th>
            <th>{{ t('multiplierLabel') }}</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="group in groups" :key="group.id">
            <td><b>{{ group.name }}</b><small>{{ group.id }}</small></td>
            <td>{{ formatDate(group.created_at) }}</td>
            <td>
              <form class="group-rate-form" @submit.prevent="editGroupMultiplier(group, $event)">
                <input name="multiplier" :value="Number(group.multiplier)" :aria-label="t('multiplierLabel')" required type="number" min="0" step="0.01" />
                <span>x</span>
                <button class="button ghost" :disabled="busy">{{ t('saveLabel') }}</button>
              </form>
            </td>
            <td></td>
          </tr>
        </tbody>
      </table>
      <Empty v-if="!groups.length" :text="t('noGroupsYet')" />
    </section>
  </div>
</template>

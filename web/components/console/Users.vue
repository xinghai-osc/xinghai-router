<script setup lang="ts">
import { useConsoleStore } from '~/composables/useConsoleStore'
import Empty from '~/components/console/Empty.vue'

const store = useConsoleStore()
const { t, users, can, manageUser } = store
</script>

<template>
  <section class="toolbar">
    <div>
      <h2>{{ t('usersAndPermissions') }}</h2>
      <p>{{ t('usersPermissionDesc') }}</p>
    </div>
  </section>
  <section class="panel table-panel">
    <table>
      <thead>
        <tr>
          <th>{{ t('userLabel') }}</th>
          <th>{{ t('roleLabel') }}</th>
          <th>{{ t('groupLabel') }}</th>
          <th>{{ t('balanceLabel') }}</th>
          <th>{{ t('permissionScope') }}</th>
          <th>{{ t('accountStatus') }}</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="user in users" :key="user.id">
          <td><b>{{ user.name }}</b><small>{{ user.email }}</small></td>
          <td><span class="pill">{{ user.role }}</span></td>
          <td>{{ user.groups.length || t('none') }}</td>
          <td>{{ Number(user.balance ?? 0).toFixed(4) }}<small v-if="Number(user.reserved ?? 0)">{{ t('reserved') }} {{ Number(user.reserved).toFixed(4) }}</small></td>
          <td>{{ user.role === 'admin' ? t('allPermissions') : user.permissions.join(', ') || t('none') }}</td>
          <td><span :class="['state', user.enabled ? 'good' : 'bad']">{{ user.enabled ? t('enabled') : t('disabled') }}</span></td>
          <td><button v-if="can('system.manage')" class="text-button" @click="manageUser(user)">{{ t('edit') }}</button></td>
        </tr>
      </tbody>
    </table>
    <Empty v-if="!users.length" :text="t('noUsersYet')" />
  </section>
</template>

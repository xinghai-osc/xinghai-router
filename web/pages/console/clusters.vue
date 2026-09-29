<script setup lang="ts">
import { Server } from 'lucide-vue-next'
import { endpoints, type Cluster, type Instance } from '~/src/api'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { can } = useAccount()
const { toast } = useToast()
const { busy, error: actionError, run } = useAction()

const allowed = computed(() => can('system.manage'))
const tab = ref<'clusters' | 'instances'>('clusters')
const { data: clusters, pending: clustersPending, error: clustersError, refresh: refreshClusters } = useResource(
  () => endpoints.getAdminClusters(),
  { data: [] as Cluster[] },
  { immediate: false },
)
const { data: instances, pending: instancesPending, error: instancesError, refresh: refreshInstances } = useResource(
  () => endpoints.getAdminInstances(),
  { data: [] as Instance[] },
  { immediate: false },
)

const clusterRows = computed(() => clusters.value.data)
const instanceRows = computed(() => instances.value.data)
const clusterOptions = computed(() => clusterRows.value.map(cluster => ({ value: cluster.id, label: cluster.name })))
const clusterNames = computed(() => new Map(clusterRows.value.map(cluster => [cluster.id, cluster.name])))
const activePending = computed(() => tab.value === 'clusters' ? clustersPending.value : instancesPending.value)
const activeError = computed(() => tab.value === 'clusters' ? clustersError.value : instancesError.value)
const activeEmpty = computed(() => tab.value === 'clusters' ? clusterRows.value.length === 0 : instanceRows.value.length === 0)

async function refresh() {
  if (allowed.value) await Promise.all([refreshClusters(), refreshInstances()])
}

onMounted(refresh)

const open = ref(false)
const formTab = ref<'clusters' | 'instances'>('clusters')
const editing = ref<Cluster | Instance | null>(null)
const form = reactive({ name: '', endpoint: '', cluster_id: '' })

function edit(item: Cluster | Instance) {
  if (busy.value) return
  formTab.value = 'cluster_id' in item ? 'instances' : 'clusters'
  editing.value = item
  form.name = item.name
  form.endpoint = 'cluster_id' in item ? item.address : item.endpoint
  form.cluster_id = 'cluster_id' in item ? item.cluster_id : ''
  actionError.value = ''
  open.value = true
}

function create() {
  if (busy.value) return
  formTab.value = tab.value
  editing.value = null
  form.name = ''
  form.endpoint = ''
  form.cluster_id = clusterRows.value[0]?.id ?? ''
  actionError.value = ''
  open.value = true
}

async function save() {
  if (busy.value) return
  const name = form.name.trim()
  const endpoint = form.endpoint.trim()
  if (!name) {
    toast.error(t('admin.nameRequired'))
    return
  }
  if (formTab.value === 'instances' && !form.cluster_id) {
    toast.error(t('admin.clusterRequired'))
    return
  }
  if (formTab.value === 'instances' && !endpoint) {
    toast.error(t('admin.instanceAddressRequired'))
    return
  }

  const ok = await run(() => {
    const item = editing.value
    if (formTab.value === 'clusters') {
      const body = { name, endpoint }
      return item && !('cluster_id' in item)
        ? endpoints.updateAdminCluster(item.id, { ...body, description: item.description, enabled: item.enabled })
        : endpoints.createAdminCluster(body)
    }

    const body = { name, address: endpoint }
    return item && 'cluster_id' in item
      ? endpoints.updateAdminInstance(item.cluster_id, item.id, body)
      : endpoints.createAdminInstance(form.cluster_id, body)
  })
  if (!ok) {
    toast.error(actionError.value || t('common.actionFailed'))
    return
  }

  open.value = false
  toast.success(t('admin.saved'))
  await refresh()
}

async function remove(item: Cluster | Instance) {
  if (busy.value || !confirm(t('admin.confirmDelete'))) return
  const ok = await run(() => 'cluster_id' in item
    ? endpoints.deleteAdminInstance(item.cluster_id, item.id)
    : endpoints.deleteAdminCluster(item.id))
  if (!ok) {
    toast.error(actionError.value || t('common.actionFailed'))
    return
  }

  toast.success(t('admin.deleted'))
  await refresh()
}

async function sync(item: Cluster) {
  if (busy.value) return
  const ok = await run(() => endpoints.syncAdminCluster(item.id))
  if (!ok) {
    toast.error(actionError.value || t('common.actionFailed'))
    return
  }
  toast.success(t('admin.clusterSyncQueued'))
  await refreshClusters()
}
</script>

<template>
  <div v-if="allowed" class="shell py-8">
    <div class="mb-6 flex items-start justify-between gap-4">
      <div>
        <div class="flex items-center gap-3">
          <Server class="size-6 text-clay" />
          <h1 class="display text-3xl text-ink">{{ t('nav.clusters') }}</h1>
        </div>
        <p class="mt-2 text-muted">{{ t('admin.clustersLead') }}</p>
      </div>
      <UiButton :disabled="busy" @click="create">{{ t('admin.create') }}</UiButton>
    </div>

    <UiTabs v-model="tab" :items="[{ value: 'clusters', label: t('admin.clusters') }, { value: 'instances', label: t('admin.instances') }]" />

    <UiAlert v-if="activeError" class="mt-4" tone="danger" :title="activeError">
      <UiButton size="sm" variant="secondary" @click="refresh">{{ t('common.retry') }}</UiButton>
    </UiAlert>
    <UiSkeleton v-else-if="activePending" class="mt-4" :rows="4" />
    <UiCard v-else class="mt-4" flush>
      <UiEmptyState v-if="activeEmpty" :title="t('admin.empty')" />
      <UiTable v-else>
        <thead>
          <tr>
            <th>{{ t('admin.id') }}</th>
            <th>{{ t(tab === 'clusters' ? 'admin.clusterName' : 'admin.instanceName') }}</th>
            <th v-if="tab === 'instances'">{{ t('admin.clusterName') }}</th>
            <th>{{ t('admin.endpoint') }}</th>
            <th>{{ t('common.actions') }}</th>
          </tr>
        </thead>
        <tbody>
          <template v-if="tab === 'clusters'">
            <tr v-for="item in clusterRows" :key="item.id">
              <td class="num">{{ item.id }}</td>
              <td>{{ item.name }}</td>
              <td>{{ item.endpoint || t('common.none') }}</td>
              <td class="text-right">
                <UiButton size="sm" variant="ghost" :disabled="busy" @click="edit(item)">{{ t('admin.edit') }}</UiButton>
                <UiButton size="sm" variant="ghost" :disabled="busy" @click="sync(item)">{{ t('admin.sync') }}</UiButton>
                <UiButton size="sm" variant="danger" :disabled="busy" @click="remove(item)">{{ t('admin.delete') }}</UiButton>
              </td>
            </tr>
          </template>
          <template v-else>
            <tr v-for="item in instanceRows" :key="item.id">
              <td class="num">{{ item.id }}</td>
              <td>{{ item.name }}</td>
              <td>{{ clusterNames.get(item.cluster_id) || item.cluster_id }}</td>
              <td>{{ item.address || t('common.none') }}</td>
              <td class="text-right">
                <UiButton size="sm" variant="ghost" :disabled="busy" @click="edit(item)">{{ t('admin.edit') }}</UiButton>
                <UiButton size="sm" variant="danger" :disabled="busy" @click="remove(item)">{{ t('admin.delete') }}</UiButton>
              </td>
            </tr>
          </template>
        </tbody>
      </UiTable>
    </UiCard>

    <UiDialog v-model:open="open" :title="editing ? t('admin.edit') : t('admin.create')">
      <div class="space-y-4">
        <UiAlert v-if="actionError" tone="danger" :title="actionError" />
        <UiField :label="t(formTab === 'clusters' ? 'admin.clusterName' : 'admin.instanceName')" required>
          <UiInput v-model="form.name" :disabled="busy" />
        </UiField>
        <UiField v-if="formTab === 'instances'" :label="t('admin.clusterName')" required>
          <UiAlert v-if="clustersError" tone="danger" :title="clustersError" />
          <UiSelect v-model="form.cluster_id" :options="clusterOptions" :disabled="busy || !!editing || clustersPending" />
        </UiField>
        <UiField :label="t('admin.endpoint')" :required="formTab === 'instances'">
          <UiInput v-model="form.endpoint" :disabled="busy" />
        </UiField>
      </div>
      <template #footer>
        <UiButton variant="secondary" :disabled="busy" @click="open = false">{{ t('common.cancel') }}</UiButton>
        <UiButton :loading="busy" @click="save">{{ t('admin.save') }}</UiButton>
      </template>
    </UiDialog>
  </div>
  <UiEmptyState v-else :title="t('admin.noAccessTitle')" :description="t('admin.noAccessBody')" />
</template>

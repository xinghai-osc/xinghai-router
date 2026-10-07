<script setup lang="ts">
import { Server } from 'lucide-vue-next'
import { endpoints, type Cluster, type Node, type NodeForm } from '~/src/api'

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
const { data: nodes, pending: nodesPending, error: nodesError, refresh: refreshNodes } = useResource(
  () => endpoints.getAdminNodes(),
  { data: [] as Node[] },
  { immediate: false },
)

const clusterRows = computed(() => clusters.value.data)
const nodeRows = computed(() => nodes.value.data)
const clusterOptions = computed(() => clusterRows.value.map(cluster => ({ value: cluster.id, label: cluster.name })))
const clusterNames = computed(() => new Map(clusterRows.value.map(cluster => [cluster.id, cluster.name])))
const activePending = computed(() => tab.value === 'clusters' ? clustersPending.value : nodesPending.value)
const activeError = computed(() => tab.value === 'clusters' ? clustersError.value : nodesError.value)
const activeEmpty = computed(() => tab.value === 'clusters' ? clusterRows.value.length === 0 : nodeRows.value.length === 0)

async function refresh() {
  if (allowed.value) await Promise.all([refreshClusters(), refreshNodes()])
}

onMounted(refresh)

const open = ref(false)
const formTab = ref<'clusters' | 'instances'>('clusters')
const editing = ref<Cluster | Node | null>(null)
const form = reactive({
  name: '',
  endpoint: '',
  cluster_id: '',
  ssh_host: '',
  ssh_port: 22,
  ssh_user: '',
  ssh_auth_method: 'password' as 'password' | 'private_key',
  ssh_password: '',
  ssh_private_key: '',
  ssh_passphrase: '',
  ssh_host_key: '',
  deploy_path: '',
  router_image: '',
})
const authOptions = computed(() => [
  { value: 'password', label: t('admin.nodeAuthPassword') },
  { value: 'private_key', label: t('admin.nodeAuthPrivateKey') },
])

function resetNodeSecrets() {
  form.ssh_password = ''
  form.ssh_private_key = ''
  form.ssh_passphrase = ''
}

function edit(item: Cluster | Node) {
  if (busy.value) return
  formTab.value = 'cluster_id' in item ? 'instances' : 'clusters'
  editing.value = item
  form.name = item.name
  form.endpoint = 'cluster_id' in item ? item.address : item.endpoint
  form.cluster_id = 'cluster_id' in item ? item.cluster_id : ''
  if ('cluster_id' in item) {
    form.ssh_host = item.ssh_host
    form.ssh_port = item.ssh_port
    form.ssh_user = item.ssh_user
    form.ssh_auth_method = item.ssh_auth_method
    form.ssh_host_key = item.ssh_host_key
    form.deploy_path = item.deploy_path
    form.router_image = item.router_image
    resetNodeSecrets()
  }
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
  form.ssh_host = ''
  form.ssh_port = 22
  form.ssh_user = ''
  form.ssh_auth_method = 'password'
  form.ssh_host_key = ''
  form.deploy_path = ''
  form.router_image = ''
  resetNodeSecrets()
  actionError.value = ''
  open.value = true
}

function nodeBody(deploy: boolean): NodeForm {
  return {
    name: form.name.trim(),
    address: form.endpoint.trim(),
    ssh_host: form.ssh_host.trim(),
    ssh_port: Number(form.ssh_port) || 22,
    ssh_user: form.ssh_user.trim(),
    ssh_auth_method: form.ssh_auth_method,
    ssh_password: form.ssh_password,
    ssh_private_key: form.ssh_private_key,
    ssh_passphrase: form.ssh_passphrase,
    ssh_host_key: form.ssh_host_key.trim(),
    deploy_path: form.deploy_path.trim(),
    router_image: form.router_image.trim(),
    deploy,
  }
}

async function save() {
  if (busy.value) return
  const name = form.name.trim()
  if (!name) {
    toast.error(t('admin.nameRequired'))
    return
  }
  if (formTab.value === 'instances') {
    if (!form.cluster_id) {
      toast.error(t('admin.clusterRequired'))
      return
    }
    if (!form.ssh_host.trim() || !form.ssh_user.trim()) {
      toast.error(t('admin.nodeSSHRequired'))
      return
    }
    if (!form.ssh_host_key.trim()) {
      toast.error(t('admin.nodeHostKeyRequired'))
      return
    }
    if (!editing.value && form.ssh_auth_method === 'password' && !form.ssh_password) {
      toast.error(t('admin.nodePasswordRequired'))
      return
    }
    if (!editing.value && form.ssh_auth_method === 'private_key' && !form.ssh_private_key) {
      toast.error(t('admin.nodePrivateKeyRequired'))
      return
    }
  }

  const ok = await run(() => {
    const item = editing.value
    if (formTab.value === 'clusters') {
      const body = { name, endpoint: form.endpoint.trim() }
      return item && !('cluster_id' in item)
        ? endpoints.updateAdminCluster(item.id, { ...body, description: item.description, enabled: item.enabled })
        : endpoints.createAdminCluster(body)
    }
    return item && 'cluster_id' in item
      ? endpoints.updateAdminNode(item.id, nodeBody(false))
      : endpoints.createAdminNode(form.cluster_id, nodeBody(true))
  })
  if (!ok) {
    toast.error(actionError.value || t('common.actionFailed'))
    return
  }

  open.value = false
  toast.success(formTab.value === 'instances' && !editing.value ? t('admin.nodeDeploymentQueued') : t('admin.saved'))
  await refresh()
}

async function remove(item: Cluster | Node) {
  if (busy.value || !confirm(t('admin.confirmDelete'))) return
  const ok = await run(() => 'cluster_id' in item
    ? endpoints.deleteAdminNode(item.id)
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

async function deploy(item: Node) {
  if (busy.value) return
  const ok = await run(() => endpoints.deployAdminNode(item.id))
  if (!ok) {
    toast.error(actionError.value || t('common.actionFailed'))
    return
  }
  toast.success(t('admin.nodeDeploymentQueued'))
  await refreshNodes()
}

function deploymentTone(status: Node['deploy_status']) {
  if (status === 'success') return 'success'
  if (status === 'failed') return 'danger'
  if (status === 'pending' || status === 'running') return 'warn'
  return 'neutral'
}

function deploymentLabel(status: Node['deploy_status']) {
  return t(`admin.nodeStatus${status.charAt(0).toUpperCase()}${status.slice(1)}`)
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
            <th v-if="tab === 'instances'">{{ t('admin.nodeDeployment') }}</th>
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
            <tr v-for="item in nodeRows" :key="item.id">
              <td class="num">{{ item.id }}</td>
              <td>
                <p>{{ item.name }}</p>
                <p class="text-xs text-muted">{{ item.ssh_user }}@{{ item.ssh_host }}:{{ item.ssh_port }}</p>
              </td>
              <td>{{ clusterNames.get(item.cluster_id) || item.cluster_name }}</td>
              <td>{{ item.address || t('common.none') }}</td>
              <td><UiBadge :tone="deploymentTone(item.deploy_status)" dot>{{ deploymentLabel(item.deploy_status) }}</UiBadge><p v-if="item.deploy_message" class="mt-1 max-w-xs truncate text-xs text-muted">{{ item.deploy_message }}</p></td>
              <td class="text-right">
                <UiButton size="sm" variant="ghost" :disabled="busy || item.deploy_status === 'running'" @click="edit(item)">{{ t('admin.edit') }}</UiButton>
                <UiButton size="sm" variant="secondary" :disabled="busy || item.deploy_status === 'running'" @click="deploy(item)">{{ t('admin.nodeDeploy') }}</UiButton>
                <UiButton size="sm" variant="danger" :disabled="busy" @click="remove(item)">{{ t('admin.delete') }}</UiButton>
              </td>
            </tr>
          </template>
        </tbody>
      </UiTable>
    </UiCard>

    <UiDialog v-model:open="open" :title="editing ? t('admin.edit') : t('admin.create')">
      <div class="max-h-[70vh] space-y-4 overflow-y-auto pr-1">
        <UiAlert v-if="actionError" tone="danger" :title="actionError" />
        <UiAlert v-if="formTab === 'instances'" tone="warn" :title="t('admin.nodeSecurityNotice')" />
        <UiField :label="t(formTab === 'clusters' ? 'admin.clusterName' : 'admin.instanceName')" required>
          <UiInput v-model="form.name" :disabled="busy" />
        </UiField>
        <template v-if="formTab === 'clusters'">
          <UiField :label="t('admin.endpoint')">
            <UiInput v-model="form.endpoint" :disabled="busy" />
          </UiField>
        </template>
        <template v-else>
          <UiField :label="t('admin.clusterName')" required>
            <UiAlert v-if="clustersError" tone="danger" :title="clustersError" />
            <UiSelect v-model="form.cluster_id" :options="clusterOptions" :disabled="busy || !!editing || clustersPending" />
          </UiField>
          <div class="grid gap-4 md:grid-cols-2">
            <UiField :label="t('admin.nodeSSHHost')" required>
              <UiInput v-model="form.ssh_host" :placeholder="t('admin.nodeSSHHostPlaceholder')" :disabled="busy" />
            </UiField>
            <UiField :label="t('admin.nodeSSHPort')" required>
              <UiInput v-model.number="form.ssh_port" type="number" min="1" max="65535" :disabled="busy" />
            </UiField>
          </div>
          <div class="grid gap-4 md:grid-cols-2">
            <UiField :label="t('admin.nodeSSHUser')" required>
              <UiInput v-model="form.ssh_user" :placeholder="t('admin.nodeSSHUserPlaceholder')" :disabled="busy" />
            </UiField>
            <UiField :label="t('admin.nodeAuthMethod')" required>
              <UiSelect v-model="form.ssh_auth_method" :options="authOptions" :disabled="busy" />
            </UiField>
          </div>
          <UiField v-if="form.ssh_auth_method === 'password'" :label="t('admin.nodeSSHPassword')" :hint="editing ? t('admin.nodeSecretHint') : undefined" :required="!editing">
            <UiInput v-model="form.ssh_password" type="password" autocomplete="new-password" :placeholder="t('admin.nodePasswordPlaceholder')" :disabled="busy" />
          </UiField>
          <UiField v-else :label="t('admin.nodePrivateKey')" :hint="editing ? t('admin.nodeSecretHint') : t('admin.nodePrivateKeyHint')" :required="!editing">
            <UiTextarea v-model="form.ssh_private_key" :rows="5" mono :placeholder="t('admin.nodePrivateKeyPlaceholder')" :disabled="busy" />
          </UiField>
          <UiField :label="t('admin.nodePassphrase')" :hint="t('admin.nodePassphraseHint')">
            <UiInput v-model="form.ssh_passphrase" type="password" autocomplete="new-password" :disabled="busy" />
          </UiField>
          <UiField :label="t('admin.nodeHostKey')" :hint="t('admin.nodeHostKeyHint')" required>
            <UiInput v-model="form.ssh_host_key" mono :placeholder="t('admin.nodeHostKeyPlaceholder')" :disabled="busy" />
          </UiField>
          <UiField :label="t('admin.endpoint')" :hint="t('admin.nodeAddressHint')">
            <UiInput v-model="form.endpoint" :placeholder="t('admin.nodeAddressPlaceholder')" :disabled="busy" />
          </UiField>
          <UiField :label="t('admin.nodeDeployPath')" :hint="t('admin.nodeDeployPathHint')">
            <UiInput v-model="form.deploy_path" mono :placeholder="t('admin.nodeDeployPathPlaceholder')" :disabled="busy" />
          </UiField>
          <UiField :label="t('admin.nodeRouterImage')" :hint="t('admin.nodeRouterImageHint')">
            <UiInput v-model="form.router_image" mono :placeholder="t('admin.nodeRouterImagePlaceholder')" :disabled="busy" />
          </UiField>
        </template>
      </div>
      <template #footer>
        <UiButton variant="secondary" :disabled="busy" @click="open = false">{{ t('common.cancel') }}</UiButton>
        <UiButton :loading="busy" @click="save">{{ t('admin.save') }}</UiButton>
      </template>
    </UiDialog>
  </div>
  <UiEmptyState v-else :title="t('admin.noAccessTitle')" :description="t('admin.noAccessBody')" />
</template>

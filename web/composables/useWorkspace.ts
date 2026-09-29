import { endpoints, setWorkspaceRequestScope, type Account, type Workspace } from '~/src/api'

interface WorkspaceState {
  userId: string
  workspaces: Workspace[]
  currentId: string
  ready: boolean
  loading: boolean
  error: string
  revision: number
  requestId: number
}

export function useWorkspace() {
  const { t } = useI18n()
  const { toast } = useToast()
  const account = useState<Account | null>('account', () => null)
  const state = useState<WorkspaceState>('workspace', () => ({
    userId: '', workspaces: [], currentId: '', ready: false, loading: false, error: '', revision: 0, requestId: 0,
  }))
  const ready = computed(() => state.value.ready && state.value.userId === account.value?.id)
  const current = computed(() => ready.value ? state.value.workspaces.find(workspace => workspace.id === state.value.currentId) ?? null : null)
  const pageKey = computed(() => `${state.value.userId}:${state.value.currentId}:${state.value.revision}`)

  function resetWorkspace() {
    if (!import.meta.client) return
    setWorkspaceRequestScope(null)
    state.value = {
      userId: '', workspaces: [], currentId: '', ready: false, loading: false, error: '',
      revision: state.value.revision + 1, requestId: state.value.requestId + 1,
    }
  }

  function storedSelection(userId: string): string {
    try { return localStorage.getItem(`xinghai.workspace:${userId}`) ?? '' } catch { return '' }
  }

  function rememberSelection(userId: string, id: string) {
    try { localStorage.setItem(`xinghai.workspace:${userId}`, id) } catch { return }
  }

  async function loadWorkspaces(force = false): Promise<void> {
    if (!import.meta.client || !account.value || account.value.must_change_password) return
    const userId = account.value.id
    if (state.value.userId !== userId) {
      resetWorkspace()
      state.value.userId = userId
    }
    if (state.value.loading || (ready.value && !force)) return
    const requestId = ++state.value.requestId
    state.value.loading = true
    state.value.error = ''
    try {
      const result = await endpoints.getWorkspaces()
      if (requestId !== state.value.requestId || account.value?.id !== userId) return
      const saved = state.value.currentId || storedSelection(userId)
      const selected = result.data.find(workspace => workspace.id === saved)
        ?? result.data.find(workspace => workspace.id === result.current_id && workspace.is_personal)
        ?? result.data.find(workspace => workspace.is_personal)
      state.value.workspaces = result.data
      if (!selected) throw new Error(t('console.workspaceUnavailable'))
      if (!ready.value || selected.id !== state.value.currentId) {
        state.value.ready = false
        state.value.revision += 1
        setWorkspaceRequestScope(null)
        await nextTick()
        if (requestId !== state.value.requestId || account.value?.id !== userId) return
        state.value.currentId = selected.id
        setWorkspaceRequestScope(selected.id)
      }
      rememberSelection(userId, selected.id)
      state.value.ready = true
    } catch (cause) {
      if (requestId === state.value.requestId) {
        state.value.error = cause instanceof Error ? cause.message : t('common.loadFailed')
        if (!state.value.workspaces.some(workspace => workspace.id === state.value.currentId)) {
          state.value.ready = false
          setWorkspaceRequestScope(null)
        }
      }
    } finally {
      if (requestId === state.value.requestId) state.value.loading = false
    }
  }

  async function switchWorkspace(id: string): Promise<void> {
    if (!import.meta.client || !ready.value || state.value.loading || id === state.value.currentId) return
    if (!state.value.workspaces.some(workspace => workspace.id === id)) return
    const userId = state.value.userId
    const requestId = ++state.value.requestId
    state.value.ready = false
    state.value.loading = true
    state.value.error = ''
    state.value.revision += 1
    setWorkspaceRequestScope(null)
    await nextTick()
    try {
      const result = await endpoints.selectWorkspace(id)
      if (requestId !== state.value.requestId || account.value?.id !== userId) return
      if (result.workspace_id !== id) throw new Error(t('console.workspaceUnavailable'))
      state.value.currentId = id
      rememberSelection(userId, id)
      setWorkspaceRequestScope(id)
      state.value.ready = true
    } catch (cause) {
      if (requestId !== state.value.requestId || account.value?.id !== userId) return
      toast.error(cause instanceof Error ? cause.message : t('common.actionFailed'))
      state.value.loading = false
      await loadWorkspaces(true)
    } finally {
      if (requestId === state.value.requestId) state.value.loading = false
    }
  }

  return {
    workspaces: computed(() => state.value.workspaces),
    loading: computed(() => state.value.loading),
    error: computed(() => state.value.error),
    ready, current, pageKey, loadWorkspaces, switchWorkspace, resetWorkspace,
  }
}

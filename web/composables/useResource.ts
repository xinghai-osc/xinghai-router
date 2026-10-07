/**
 * Minimal client-side resource loader for console views.
 *
 * Console pages are behind auth and must not be prerendered, so they fetch on
 * mount rather than through Nuxt's useAsyncData.
 */
export function useResource<T>(loader: () => Promise<T>, initial: T, options: { immediate?: boolean } = {}) {
  const { t } = useI18n()
  const data = ref<T>(initial) as Ref<T>
  const pending = ref(false)
  const error = ref('')
  let requestId = 0
  let activeRequest: Promise<void> | null = null
  let queuedRequest: Promise<void> | null = null

  function startRequest() {
    const currentRequestId = ++requestId
    const request = (async () => {
      pending.value = true
      error.value = ''
      try {
        const next = await loader()
        if (currentRequestId === requestId) data.value = next
      } catch (cause) {
        if (currentRequestId === requestId) {
          error.value = cause instanceof Error ? cause.message : t('common.loadFailed')
        }
      } finally {
        if (currentRequestId === requestId) pending.value = false
      }
    })()
    const trackedRequest = request.finally(() => {
      if (activeRequest === trackedRequest) activeRequest = null
    })
    activeRequest = trackedRequest
    return trackedRequest
  }

  function refresh() {
    if (!activeRequest) return startRequest()

    requestId += 1
    if (!queuedRequest) {
      const currentRequest = activeRequest
      queuedRequest = currentRequest.then(() => {
        queuedRequest = null
        return startRequest()
      })
    }
    return queuedRequest
  }

  if (import.meta.client && options.immediate !== false) onMounted(refresh)

  return { data, pending, error, refresh }
}

/**
 * Wraps a mutating action with busy state and toast-friendly error capture.
 * Returns true when the action completed without throwing.
 */
export function useAction() {
  const { t } = useI18n()
  const busy = ref(false)
  const error = ref('')

  async function run(work: () => Promise<unknown>): Promise<boolean> {
    if (busy.value) return false
    busy.value = true
    error.value = ''
    try {
      await work()
      return true
    } catch (cause) {
      error.value = cause instanceof Error ? cause.message : t('common.actionFailed')
      return false
    } finally {
      busy.value = false
    }
  }

  return { busy, error, run }
}

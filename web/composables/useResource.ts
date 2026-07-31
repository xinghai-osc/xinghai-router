/**
 * Minimal client-side resource loader for console views.
 *
 * Console pages are behind auth and must not be prerendered, so they fetch on
 * mount rather than through Nuxt's useAsyncData.
 */
export function useResource<T>(loader: () => Promise<T>, initial: T) {
  const { t } = useI18n()
  const data = ref<T>(initial) as Ref<T>
  const pending = ref(false)
  const error = ref('')

  async function refresh() {
    pending.value = true
    error.value = ''
    try {
      data.value = await loader()
    } catch (cause) {
      error.value = cause instanceof Error ? cause.message : t('common.loadFailed')
    } finally {
      pending.value = false
    }
  }

  if (import.meta.client) onMounted(refresh)

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

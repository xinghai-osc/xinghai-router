export function useDeferredLoad(load: () => void, options: { rootMargin?: string } = {}) {
  const target = ref<HTMLElement | null>(null)
  const started = ref(false)
  let observer: IntersectionObserver | null = null
  let fallbackTimer: ReturnType<typeof window.setTimeout> | null = null

  function start() {
    if (started.value) return
    started.value = true
    load()
  }

  onMounted(() => {
    const element = target.value
    if (!element || typeof IntersectionObserver === 'undefined') {
      fallbackTimer = window.setTimeout(start, 0)
      return
    }

    observer = new IntersectionObserver((entries) => {
      if (!entries.some(entry => entry.isIntersecting)) return
      observer?.disconnect()
      observer = null
      start()
    }, { rootMargin: options.rootMargin ?? '480px 0px' })
    observer.observe(element)
  })

  onBeforeUnmount(() => {
    observer?.disconnect()
    observer = null
    if (fallbackTimer !== null) window.clearTimeout(fallbackTimer)
    fallbackTimer = null
  })

  return { target, started }
}

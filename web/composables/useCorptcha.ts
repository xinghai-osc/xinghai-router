/**
 * Corptcha verification helper.
 *
 * The SDK script is third-party and only fetched the first time a challenge
 * is actually needed, so sites without captcha never pay for it. The widget is
 * rendered into an off-screen container and auto-starts, so the visitor only
 * sees the challenge popup.
 */

const SCRIPT_SRC = 'https://res.25y.cn/corptcha/corptcha.iife.js'
const API_BASE_URL = 'https://cpt-api.25y.cn'
const READY_TIMEOUT_MS = 15_000

/** Reject reasons, distinguished so the caller can pick the right message. */
export const CORPTCHA_UNAVAILABLE = 'corptcha_unavailable'
export const CORPTCHA_CANCELLED = 'corptcha_cancelled'

/** Declared as a type alias, not an interface, so it stays assignable to Record<string, string>. */
export type CorptchaResult = {
  captcha_token: string
  captcha_purpose: string
}

interface CorptchaInstance {
  execute: () => void
  reset: () => void
  destroy: () => void
}

interface CorptchaOptions {
  apiBaseUrl: string
  siteKey: string
  purpose: string
  language: string
  autoExecute: boolean
  onSuccess: (token: string) => void
  onError: (error: { errorCode?: string; message?: string }) => void
  onExpired: () => void
}

interface CorptchaStatic {
  render: (container: HTMLElement, options: CorptchaOptions) => CorptchaInstance
}

declare global {
  interface Window { Corptcha?: CorptchaStatic }
}

let loader: Promise<CorptchaStatic> | null = null

function loadWidget(): Promise<CorptchaStatic> {
  if (!import.meta.client) return Promise.reject(new Error(CORPTCHA_UNAVAILABLE))
  if (window.Corptcha) return Promise.resolve(window.Corptcha)
  if (loader) return loader
  loader = new Promise<CorptchaStatic>((resolve, reject) => {
    const script = document.createElement('script')
    script.src = SCRIPT_SRC
    script.async = true
    script.onload = () => {
      if (window.Corptcha) resolve(window.Corptcha)
      else reject(new Error(CORPTCHA_UNAVAILABLE))
    }
    script.onerror = () => reject(new Error(CORPTCHA_UNAVAILABLE))
    document.head.append(script)
  })
  loader = loader.catch((cause) => {
    loader = null
    throw cause
  })
  return loader
}

export function useCorptcha() {
  const { locale } = useI18n()

  /**
   * Run the verification and resolve with the one-time token the Go handlers
   * expect. Rejects with CORPTCHA_CANCELLED when the visitor fails or dismisses
   * the challenge; CORPTCHA_UNAVAILABLE when the SDK cannot load.
   */
  async function challenge(siteKey: string, purpose: string): Promise<CorptchaResult> {
    if (!siteKey) throw new Error(CORPTCHA_UNAVAILABLE)
    const factory = await loadWidget()
    const container = document.createElement('div')
    container.style.cssText = 'position:fixed;left:-9999px;top:0;width:1px;height:1px;overflow:hidden'
    document.body.append(container)
    let widget: CorptchaInstance | null = null
    let settled = false
    const finish = (action: () => void) => {
      if (settled) return
      settled = true
      try {
        widget?.destroy()
      } catch {
        // the widget may already be gone
      }
      container.remove()
      action()
    }
    return new Promise<CorptchaResult>((resolve, reject) => {
      const timeout = setTimeout(() => finish(() => reject(new Error(CORPTCHA_UNAVAILABLE))), READY_TIMEOUT_MS)
      const onError = () => finish(() => reject(new Error(CORPTCHA_CANCELLED)))
      try {
        widget = factory.render(container, {
          apiBaseUrl: API_BASE_URL,
          siteKey,
          purpose,
          language: locale.value === 'en' ? 'en-US' : 'zh-CN',
          autoExecute: true,
          onSuccess: (token) => finish(() => resolve({ captcha_token: token, captcha_purpose: purpose })),
          onError,
          onExpired: () => widget?.execute(),
        })
      } catch {
        clearTimeout(timeout)
        container.remove()
        reject(new Error(CORPTCHA_UNAVAILABLE))
      }
    })
  }

  return { challenge }
}
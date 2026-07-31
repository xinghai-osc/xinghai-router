/**
 * GeeTest v4 challenge helper.
 *
 * The widget script is third-party and only fetched the first time a challenge
 * is actually needed, so sites without captcha never pay for it.
 */

const SCRIPT_SRC = 'https://static.geetest.com/v4/gt4.js'
const READY_TIMEOUT_MS = 15_000

/** Reject reasons, distinguished so the caller can pick the right message. */
export const GEETEST_UNAVAILABLE = 'geetest_unavailable'
export const GEETEST_CANCELLED = 'geetest_cancelled'

/** Declared as a type alias, not an interface, so it stays assignable to Record<string, string>. */
export type GeetestResult = {
  lot_number: string
  captcha_output: string
  pass_token: string
  gen_time: string
}

interface GeetestInstance {
  onReady: (handler: () => void) => GeetestInstance
  onSuccess: (handler: () => void) => GeetestInstance
  onError: (handler: () => void) => GeetestInstance
  onClose: (handler: () => void) => GeetestInstance
  showCaptcha: () => void
  getValidate: () => GeetestResult | false
  destroy?: () => void
}

type GeetestFactory = (
  config: { captchaId: string; product: string; language: string },
  handler: (instance: GeetestInstance) => void,
) => void

declare global {
  interface Window { initGeetest4?: GeetestFactory }
}

let loader: Promise<GeetestFactory> | null = null

function loadWidget(): Promise<GeetestFactory> {
  if (!import.meta.client) return Promise.reject(new Error(GEETEST_UNAVAILABLE))
  if (window.initGeetest4) return Promise.resolve(window.initGeetest4)
  if (loader) return loader
  loader = new Promise<GeetestFactory>((resolve, reject) => {
    const script = document.createElement('script')
    script.src = SCRIPT_SRC
    script.async = true
    script.onload = () => {
      if (window.initGeetest4) resolve(window.initGeetest4)
      else reject(new Error(GEETEST_UNAVAILABLE))
    }
    script.onerror = () => reject(new Error(GEETEST_UNAVAILABLE))
    document.head.append(script)
  })
  loader = loader.catch((cause) => {
    loader = null
    throw cause
  })
  return loader
}

export function useGeetest() {
  const { locale } = useI18n()

  function create(factory: GeetestFactory, captchaId: string): Promise<GeetestInstance> {
    return new Promise((resolve, reject) => {
      const timeout = setTimeout(() => reject(new Error(GEETEST_UNAVAILABLE)), READY_TIMEOUT_MS)
      factory(
        { captchaId, product: 'bind', language: locale.value === 'zh' ? 'zho' : 'eng' },
        (instance) => {
          instance.onReady(() => { clearTimeout(timeout); resolve(instance) })
          instance.onError(() => { clearTimeout(timeout); reject(new Error(GEETEST_UNAVAILABLE)) })
        },
      )
    })
  }

  /**
   * Show the challenge and resolve with the validation payload the Go handlers
   * expect. Rejects with GEETEST_CANCELLED when the visitor closes the widget.
   */
  async function challenge(captchaId: string): Promise<GeetestResult> {
    if (!captchaId) throw new Error(GEETEST_UNAVAILABLE)
    const factory = await loadWidget()
    const instance = await create(factory, captchaId)
    return new Promise<GeetestResult>((resolve, reject) => {
      let settled = false
      const finish = (action: () => void) => {
        if (settled) return
        settled = true
        instance.destroy?.()
        action()
      }
      instance.onSuccess(() => {
        const result = instance.getValidate()
        finish(() => (result ? resolve(result) : reject(new Error(GEETEST_UNAVAILABLE))))
      })
      instance.onError(() => finish(() => reject(new Error(GEETEST_UNAVAILABLE))))
      instance.onClose(() => finish(() => reject(new Error(GEETEST_CANCELLED))))
      instance.showCaptcha()
    })
  }

  return { challenge }
}

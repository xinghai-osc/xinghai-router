export type ToastTone = 'info' | 'success' | 'warn' | 'danger'

export interface Toast {
  id: number
  tone: ToastTone
  title: string
  body?: string
}

let sequence = 0

export function useToast() {
  const toasts = useState<Toast[]>('toasts', () => [])

  function dismiss(id: number) {
    toasts.value = toasts.value.filter(toast => toast.id !== id)
  }

  function push(tone: ToastTone, title: string, body?: string) {
    sequence += 1
    const id = sequence
    toasts.value = [...toasts.value, { id, tone, title, body }]
    if (import.meta.client) setTimeout(() => dismiss(id), tone === 'danger' ? 6000 : 3500)
    return id
  }

  return {
    toasts,
    dismiss,
    toast: {
      info: (title: string, body?: string) => push('info', title, body),
      success: (title: string, body?: string) => push('success', title, body),
      warn: (title: string, body?: string) => push('warn', title, body),
      error: (title: string, body?: string) => push('danger', title, body),
    },
  }
}

let pending: { promise: Promise<boolean>; resolve: (verified: boolean) => void } | null = null

export function useReauthentication() {
  const open = useState('reauthentication-open', () => false)
  const challengeId = useState('reauthentication-id', () => 0)

  function finish(verified: boolean, id = challengeId.value) {
    if (!import.meta.client || id !== challengeId.value) return
    const current = pending
    pending = null
    open.value = false
    current?.resolve(verified)
  }

  function cancel() {
    finish(false)
  }

  function request(): Promise<boolean> {
    if (!import.meta.client) return Promise.resolve(false)
    if (pending) return pending.promise
    const promise = new Promise<boolean>((resolve) => {
      pending = { promise: Promise.resolve(false), resolve }
    })
    pending!.promise = promise
    challengeId.value += 1
    open.value = true
    return promise
  }

  return { open: readonly(open), challengeId: readonly(challengeId), request, cancel, finish }
}

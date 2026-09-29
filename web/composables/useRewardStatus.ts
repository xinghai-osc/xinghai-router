export function useRewardStatus() {
  const { t } = useI18n()
  const keys: Record<string, string> = {
    pending: 'console.rewardPending',
    credited: 'console.rewardCredited',
    rejected: 'console.rewardRejected',
    withdrawn: 'console.rewardWithdrawn',
  }

  function rewardLabel(status = 'credited') {
    return keys[status] ? t(keys[status]) : status
  }

  function rewardTone(status = 'credited'): 'success' | 'warn' | 'danger' | 'neutral' {
    if (status === 'credited') return 'success'
    if (status === 'pending') return 'warn'
    if (status === 'rejected') return 'danger'
    return 'neutral'
  }

  return { rewardLabel, rewardTone }
}

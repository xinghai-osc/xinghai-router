import { endpoints, type Notification } from '~/src/api'

/**
 * Enabled site notifications for the login popup. Dismissed notification IDs
 * stay hidden in this browser until a notification with a new ID is published.
 */
const dismissedStorageKey = 'xinghai-dismissed-notifications'

export function useNotifications() {
  const notifications = useState<Notification[]>('notifications', () => [])
  const open = useState('notifications-open', () => false)
  const loaded = useState('notifications-loaded', () => false)

  function dismissedIds() {
    if (!import.meta.client) return new Set<string>()
    try {
      const stored = JSON.parse(localStorage.getItem(dismissedStorageKey) ?? '[]')
      return new Set<string>(Array.isArray(stored) ? stored.filter((id): id is string => typeof id === 'string') : [])
    } catch {
      return new Set<string>()
    }
  }

  function saveDismissedIds(ids: Set<string>) {
    localStorage.setItem(dismissedStorageKey, JSON.stringify([...ids]))
  }

  function hasNewNotifications(items: Notification[]) {
    const dismissed = dismissedIds()
    return items.some(notification => !dismissed.has(notification.id))
  }

  function dismissNotifications() {
    if (!import.meta.client) return
    const dismissed = dismissedIds()
    for (const notification of notifications.value) dismissed.add(notification.id)
    saveDismissedIds(dismissed)
    open.value = false
  }

  async function loadNotifications() {
    if (!import.meta.client || loaded.value) return
    loaded.value = true
    try {
      const remote = await endpoints.getNotifications()
      notifications.value = remote.data
      open.value = hasNewNotifications(remote.data)
    } catch {
      loaded.value = false
    }
  }

  return { notifications, open, loadNotifications, dismissNotifications }
}

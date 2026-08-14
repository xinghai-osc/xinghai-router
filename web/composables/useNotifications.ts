import { endpoints, type Notification } from '~/src/api'

/**
 * Enabled site notifications for the login popup. Loaded once per page load by
 * the console layout, so the popup reappears on every fresh login.
 */
export function useNotifications() {
  const notifications = useState<Notification[]>('notifications', () => [])
  const open = useState('notifications-open', () => false)
  const loaded = useState('notifications-loaded', () => false)

  async function loadNotifications() {
    if (!import.meta.client || loaded.value) return
    loaded.value = true
    try {
      const remote = await endpoints.getNotifications()
      notifications.value = remote.data
      open.value = remote.data.length > 0
    } catch {
      loaded.value = false
    }
  }

  return { notifications, open, loadNotifications }
}

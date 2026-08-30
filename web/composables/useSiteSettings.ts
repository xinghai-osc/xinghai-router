import { endpoints, type SiteSettings } from '~/src/api'

const FALLBACK: Omit<SiteSettings, 'name'> = {
  icon_url: '',
  announcement: '',
  contact_email: '',
  auto_disable_failed_channels: false,
}

export function useSiteSettings() {
  const { t } = useI18n()
  const fallback = (): SiteSettings => ({ name: t('common.brand'), ...FALLBACK })
  const settings = useState<SiteSettings>('site-settings', fallback)
  const loaded = useState('site-settings-loaded', () => false)

  async function loadSiteSettings() {
    if (loaded.value || !import.meta.client) return
    loaded.value = true
    try {
      const remote = await endpoints.getSiteSettings()
      settings.value = { ...fallback(), ...remote, name: remote.name || fallback().name }
    } catch {
      loaded.value = false
    }
  }

  return { settings, loadSiteSettings }
}

import { endpoints, type FeaturedCopy, type FeaturedLocale, type SiteSettings } from '~/src/api'

function emptyFeaturedCopy(): FeaturedCopy {
  return {
    zh: { badge: '', title: '', body: '', cta: '' },
    'zh-Hant': { badge: '', title: '', body: '', cta: '' },
    en: { badge: '', title: '', body: '', cta: '' },
  }
}

function mergeFeaturedCopy(value: Partial<FeaturedCopy> | null | undefined): FeaturedCopy {
  const merged = emptyFeaturedCopy()
  for (const locale of ['zh', 'zh-Hant', 'en'] as FeaturedLocale[]) {
    const configured = value?.[locale]
    if (configured) merged[locale] = { ...merged[locale], ...configured }
  }
  return merged
}

const FALLBACK: Omit<SiteSettings, 'name'> = {
  icon_url: '',
  announcement: '',
  contact_email: '',
  auto_disable_failed_channels: false,
  featured_enabled: true,
  featured_model: '',
  featured_copy: emptyFeaturedCopy(),
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
      const defaults = fallback()
      settings.value = {
        ...defaults,
        ...remote,
        name: remote.name || defaults.name,
        featured_enabled: remote.featured_enabled ?? defaults.featured_enabled,
        featured_model: remote.featured_model ?? defaults.featured_model,
        featured_copy: mergeFeaturedCopy(remote.featured_copy),
      }
    } catch {
      loaded.value = false
    }
  }

  return { settings, loadSiteSettings }
}

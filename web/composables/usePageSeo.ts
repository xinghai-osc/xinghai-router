import { computed, toValue, type MaybeRefOrGetter } from 'vue'

interface PageSeoOptions {
  title: MaybeRefOrGetter<string>
  description: MaybeRefOrGetter<string>
  type?: MaybeRefOrGetter<'website' | 'article'>
  noindex?: MaybeRefOrGetter<boolean>
  image?: MaybeRefOrGetter<string>
  structuredData?: MaybeRefOrGetter<Record<string, unknown> | Record<string, unknown>[] | undefined>
}

const LOCALE_TO_OG: Record<string, string> = {
  zh: 'zh_CN',
  'zh-Hant': 'zh_TW',
  en: 'en_US',
}

function escapeJsonLd(value: string): string {
  return value.replace(/</g, '\\u003c').replace(/>/g, '\\u003e').replace(/&/g, '\\u0026')
}

export function usePageSeo(options: PageSeoOptions) {
  const route = useRoute()
  const { locale, t } = useI18n()
  const { settings } = useSiteSettings()
  const siteUrl = useSiteOrigin()
  const canonical = computed(() => new URL(route.path, `${siteUrl.value}/`).toString())
  const title = computed(() => toValue(options.title))
  const description = computed(() => toValue(options.description))
  const siteName = computed(() => settings.value.name || t('common.brand'))
  const type = computed(() => options.type ? toValue(options.type) : 'website')
  const image = computed(() => {
    const configured = options.image ? toValue(options.image) : ''
    return configured ? new URL(configured, `${siteUrl.value}/`).toString() : new URL('/og-image.png', `${siteUrl.value}/`).toString()
  })
  const noindex = computed(() => Boolean(options.noindex && toValue(options.noindex)))
  const structuredData = computed(() => {
    const data = options.structuredData ? toValue(options.structuredData) : undefined
    if (!data) return []
    const entries = Array.isArray(data) ? data : [data]
    return entries.map(entry => escapeJsonLd(JSON.stringify(entry)))
  })

  useHead(() => ({
    title: title.value,
    link: [{ rel: 'canonical', href: canonical.value }],
    meta: [
      { name: 'description', content: description.value },
      { name: 'robots', content: noindex.value ? 'noindex, nofollow' : 'index, follow' },
      { property: 'og:type', content: type.value },
      { property: 'og:title', content: title.value },
      { property: 'og:description', content: description.value },
      { property: 'og:url', content: canonical.value },
      { property: 'og:site_name', content: siteName.value },
      { property: 'og:locale', content: LOCALE_TO_OG[locale.value] || LOCALE_TO_OG.zh },
      { property: 'og:image', content: image.value },
      { property: 'og:image:width', content: '1200' },
      { property: 'og:image:height', content: '630' },
      { property: 'og:image:alt', content: title.value },
      { name: 'twitter:card', content: 'summary_large_image' },
      { name: 'twitter:title', content: title.value },
      { name: 'twitter:description', content: description.value },
      { name: 'twitter:image', content: image.value },
      { name: 'twitter:image:alt', content: title.value },
    ],
    script: structuredData.value.map((innerHTML, index) => ({
      key: `structured-data-${index}`,
      type: 'application/ld+json',
      innerHTML,
    })),
  }))
}

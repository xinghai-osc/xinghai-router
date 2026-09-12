import { resolveSiteOrigin } from '~/utils/site-url'

export function useSiteOrigin() {
  const config = useRuntimeConfig()
  const requestUrl = useRequestURL()
  return computed(() => resolveSiteOrigin(String(config.public.siteUrl || ''), requestUrl.origin))
}

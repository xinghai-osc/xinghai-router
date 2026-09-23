export default defineNuxtRouteMiddleware(async (to) => {
  if (import.meta.server) return
  const { authenticated, loaded, loadAccount } = useAccount()
  await loadAccount(true)
  if (loaded.value && !authenticated.value) {
    return navigateTo({ path: '/auth', query: { redirect: to.fullPath } }, { redirectCode: 302 })
  }
})

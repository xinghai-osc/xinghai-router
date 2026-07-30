import tailwindcss from '@tailwindcss/vite'

const publicRoutes = ['/', '/models', '/rankings', '/pricing', '/terms', '/privacy']

export default defineNuxtConfig({
  modules: ['@nuxt/eslint'],
  css: ['~/assets/css/main.css'],
  components: [{ path: '~/components', pathPrefix: true }],
  app: {
    head: {
      htmlAttrs: { lang: 'zh-CN' },
      link: [
        { rel: 'preconnect', href: 'https://fonts.googleapis.com' },
        { rel: 'preconnect', href: 'https://fonts.gstatic.com', crossorigin: '' },
        {
          rel: 'stylesheet',
          href: 'https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&family=Instrument+Serif:ital@0;1&family=JetBrains+Mono:wght@400;500&display=swap',
        },
      ],
    },
  },
  hooks: {
    // /design is an internal style guide with hard-coded copy — it is the one
    // page exempt from the i18n rule, so it must never reach production.
    'pages:extend'(pages) {
      if (process.env.NODE_ENV !== 'production') return
      const index = pages.findIndex(page => page.path === '/design')
      if (index >= 0) pages.splice(index, 1)
    },
  },
  vite: { plugins: [tailwindcss()] },
  nitro: { prerender: { routes: publicRoutes } },
  routeRules: Object.fromEntries(publicRoutes.map(route => [route, { prerender: true }])),
  devServer: { port: 5173, host: '127.0.0.1' },
  compatibilityDate: '2026-07-16',
})

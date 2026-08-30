import tailwindcss from '@tailwindcss/vite'

const publicRoutes = ['/', '/auth', '/auth/reset', '/redeem', '/models', '/rankings', '/pricing', '/terms', '/privacy']

export default defineNuxtConfig({
  modules: ['@nuxt/eslint'],
  css: ['~/assets/css/main.css'],
  components: [{ path: '~/components', pathPrefix: true }],
  app: {
    pageTransition: { name: 'page' },
    head: {
      htmlAttrs: { lang: 'zh-CN' },
      link: [
        { rel: 'preconnect', href: 'https://fonts.googleapis.com' },
        { rel: 'preconnect', href: 'https://fonts.gstatic.com', crossorigin: '' },
        {
          rel: 'stylesheet',
          href: 'https://fonts.googleapis.com/css2?family=DM+Sans:wght@400;500;600;700&family=Instrument+Serif:ital@0;1&family=Inter:wght@400;500;600;700&family=JetBrains+Mono:wght@400;500&family=Montserrat:wght@400;500;600&display=swap',
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
  // Disable sourcemaps in production — kills server-side .map generation
  // (117 files, ~3 MB) and reduces build time & memory. Nuxt default resolves
  // to { server: true, client: dev }, so only server maps are lost in prod.
  sourcemap: false,
  vite: {
    plugins: [tailwindcss()],
    build: {
      // Skip gzip-size reporting saves a small amount of build time per chunk
      reportCompressedSize: false,
      chunkSizeWarningLimit: 1024,
    },
  },
  nitro: {
    prerender: {
      routes: publicRoutes,
      // Default concurrency is 1 (sequential) — 8 independent routes each take
      // ~365 ms, totalling 4.4 s. Parallelising saves ~3 s per build.
      concurrency: 8,
    },
  },
  routeRules: Object.fromEntries(publicRoutes.map(route => [route, { prerender: true }])),
  devServer: { port: 5173, host: '127.0.0.1' },
  compatibilityDate: '2026-07-16',
})

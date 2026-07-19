import tailwindcss from '@tailwindcss/vite'

export default defineNuxtConfig({
  modules: ['@nuxt/eslint'],
  css: ['~/src/style.css'],
  vite: {
    plugins: [tailwindcss()],
  },
  nitro: {
    prerender: {
      routes: ['/', '/auth', '/rankings', '/terms', '/privacy'],
    },
  },
  routeRules: {
    '/': { prerender: true },
    '/auth': { prerender: true },
    '/rankings': { prerender: true },
    '/terms': { prerender: true },
    '/privacy': { prerender: true },
  },
  devServer: { port: 5173, host: '127.0.0.1' },
  compatibilityDate: '2026-07-16',
})

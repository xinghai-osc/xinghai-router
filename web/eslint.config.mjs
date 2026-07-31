// @ts-check
import withNuxt from './.nuxt/eslint.config.mjs'

export default withNuxt({
  rules: {
    // Optional props are declared through TypeScript (`prop?: T`); `undefined`
    // is the intended default, so a runtime default would only add noise.
    'vue/require-default-prop': 'off',
  },
})

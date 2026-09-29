import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { test } from 'node:test'
import vm from 'node:vm'
import ts from 'typescript'
import { readonly, ref } from 'vue'

const root = new URL('../', import.meta.url)
const source = await readFile(new URL('composables/useTheme.ts', root), 'utf8')
const appSource = await readFile(new URL('app.vue', root), 'utf8')
const cssImports = await readFile(new URL('assets/css/main.css', root), 'utf8')

function fixture(saved = {}, blocked = false) {
  const storage = new Map(Object.entries(saved))
  const context = vm.createContext({
    exports: {}, ref, readonly,
    document: { documentElement: { dataset: {} } },
    localStorage: {
      getItem(key) {
        if (blocked) throw new Error('Storage unavailable')
        return storage.get(key) ?? null
      },
      setItem(key, value) {
        if (blocked) throw new Error('Storage unavailable')
        storage.set(key, value)
      },
    },
  })
  const compiled = ts.transpileModule(source.replaceAll('import.meta.client', 'true'), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
  }).outputText
  vm.runInContext(compiled, context)
  const theme = context.exports.useTheme()
  return { context, storage, theme }
}

function runNoFlash(context) {
  Object.assign(context, context.exports)
  const declaration = appSource.split('\n').find(line => line.startsWith('const noFlashTheme = '))
  assert.ok(declaration)
  vm.runInContext(declaration.replace('const noFlashTheme = ', 'globalThis.noFlashTheme = '), context)
  vm.runInContext(context.noFlashTheme, context)
}

test('new visitors receive the same light blue theme before and after hydration', () => {
  const { context, theme } = fixture()
  runNoFlash(context)
  assert.equal(context.document.documentElement.dataset.theme, 'light')
  assert.equal(context.document.documentElement.dataset.preset, 'newapi')
  theme.initializeTheme()
  assert.equal(theme.mode.value, 'light')
  assert.equal(theme.preset.value, 'newapi')
})

test('every saved preset and mode is preserved by initialization and the no-flash script', () => {
  const { context: defaults } = fixture()
  for (const preset of defaults.exports.THEME_PRESETS) {
    for (const mode of ['light', 'dark']) {
      const { context, theme } = fixture({ 'xinghai.theme': mode, 'xinghai.preset': preset.value })
      runNoFlash(context)
      assert.equal(context.document.documentElement.dataset.theme, mode)
      assert.equal(context.document.documentElement.dataset.preset, preset.value)
      theme.initializeTheme()
      assert.equal(theme.mode.value, mode)
      assert.equal(theme.preset.value, preset.value)
    }
  }
})

test('invalid values and blocked storage gracefully fall back without breaking theme controls', () => {
  for (const blocked of [false, true]) {
    const { context, theme } = fixture({ 'xinghai.theme': 'invalid', 'xinghai.preset': 'unknown' }, blocked)
    runNoFlash(context)
    theme.initializeTheme()
    assert.equal(context.document.documentElement.dataset.theme, 'light')
    assert.equal(context.document.documentElement.dataset.preset, 'newapi')
    theme.toggleMode()
    theme.setPreset('galaxy')
    assert.equal(context.document.documentElement.dataset.theme, 'dark')
    assert.equal(context.document.documentElement.dataset.preset, 'galaxy')
  }
})

test('theme changes persist and all registered presets have styles and bilingual labels', async () => {
  const { context, theme, storage } = fixture()
  theme.setMode('dark')
  theme.setPreset('newapi')
  assert.equal(storage.get('xinghai.theme'), 'dark')
  assert.equal(storage.get('xinghai.preset'), 'newapi')
  for (const { value } of context.exports.THEME_PRESETS) {
    assert.ok(cssImports.includes(`./themes/${value}.css`))
    const css = await readFile(new URL(`assets/css/themes/${value}.css`, root), 'utf8')
    assert.ok(css.includes(`[data-preset='${value}'][data-theme='dark']`))
    for (const locale of ['zh', 'zh-Hant', 'en']) {
      const messages = await readFile(new URL(`src/locales/${locale}/theme.ts`, root), 'utf8')
      assert.ok(messages.includes(`${value}Label:`))
      assert.ok(messages.includes(`${value}Hint:`))
    }
  }
})

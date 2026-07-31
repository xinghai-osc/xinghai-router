# web/AGENTS.md

Conventions for the Nuxt 3 console. Read this before writing any component here.
Repo-wide rules live in the root `AGENTS.md`; this file overrides it for `web/`.

## Stack

Nuxt 3 · Vue 3 `<script setup lang="ts">` · Tailwind CSS v4 · reka-ui (headless
primitives) · lucide-vue-next (icons) · @vueuse/core. No shadcn-vue, no
`class-variance-authority`, no component library beyond what is listed here.

Verify with `pnpm run lint` (must be 0 errors, 0 warnings) and `pnpm run build`.

## Layout

```
assets/css/     tokens.css (Tailwind mapping) · base.css · utilities.css · themes/*.css
components/ui/       design-system primitives, auto-imported as <UiButton> …
components/site/     public-site blocks, <SiteHero> …
components/console/  console blocks, <ConsoleSidebar> …
composables/         useI18n useTheme useAccount useToast useResource useSiteSettings …
layouts/             default.vue (public) · console.vue (signed-in)
pages/               file-based routes; console views under pages/console/
src/api.ts           typed API client — mirrors the Go JSON exactly
src/locales/         i18n messages, one file per namespace per locale
src/nav.ts           console navigation + permission declarations
src/format.ts        formatNumber / formatMoney / formatDateTime / …
src/marketplace.ts   model-square pricing and filtering
```

One component per file. Do not concatenate unrelated views into one file.

## Design tokens — never hardcode a colour

Use only these Tailwind classes for colour. They resolve through CSS variables,
so all three themes (`default` / `cool` / `galaxy`) and both light and dark
modes work for free. A literal hex or a stock Tailwind colour (`bg-gray-100`,
`text-blue-500`) is a bug — it will break in the other themes.

| Purpose | Class |
| --- | --- |
| Page background | `bg-paper` |
| Card / panel | `bg-surface` |
| Inset (table head, code) | `bg-sunken` |
| Primary text | `text-ink` |
| Secondary text | `text-muted` |
| Tertiary text | `text-faint` |
| Border | `border-line`, `border-line-strong` |
| Accent | `bg-clay` `text-clay` `bg-clay-soft` `text-clay-ink` |
| Semantic | `success` `warn` `danger` (+ `-soft`) |

Other tokens: `rounded-card` (16px), `rounded-control` (10px), `shadow-pop`
(overlays only), `text-2xs`, and the utilities `display` (serif headline),
`numeric` (tabular mono digits), `shell` (page container).

Style rules: borders over shadows; shadows only on dialogs and dropdowns.
Transitions are 150ms ease-out on colour/opacity/transform only. Section
padding on public pages is `py-20 md:py-24`.

## i18n is mandatory

**Every user-visible string goes through `t()`.** No Chinese or English literals
in templates — including `aria-label`, `placeholder`, `title` and toast text.

```vue
<script setup lang="ts">
const { t } = useI18n()
</script>

<template>
  <UiButton>{{ t('console.createKey') }}</UiButton>
  <UiInput :placeholder="t('common.search')" />
</template>
```

Add keys to **your own namespace file only**, in both `src/locales/zh/<ns>.ts`
and `src/locales/en/<ns>.ts`. Namespaces: `common` `nav` `site` `auth`
`console` `admin` `system` `theme`. Interpolation uses `{name}`:
`t('site.wallModelCount', { count: 12 })`.

Never edit a namespace you were not assigned — parallel agents share this tree.

Data from the API (model names, user names, plan names, error messages from the
server) is passed through as-is, not translated.

The single exemption is `pages/design.vue` plus `components/design/`: an internal
style guide whose sample copy is hard-coded on purpose. `nuxt.config.ts` strips
the `/design` route from production builds, so it never reaches users.

## UI primitives

`UiButton` (`variant`: primary/secondary/ghost/danger/link, `size`:
sm/md/lg/icon, `loading`, `block`, `to`) · `UiCard` (`title`, `description`,
`flush`, slots `actions`/`footer`) · `UiInput` `UiTextarea` (v-model, slots
`leading`/`trailing`) · `UiField` (`label`, `hint`, `error`, `required`) ·
`UiSelect` (`options: {value,label}[]`, v-model) · `UiSwitch` `UiCheckbox`
(v-model) · `UiBadge` (`tone`, `dot`) · `UiTable` (plain `<thead>/<tbody>`
inside; add class `num` to numeric cells) · `UiDialog` (`v-model:open`, slot
`footer`) · `UiDropdownMenu` + `UiDropdownItem` (`as`: item/label/separator,
`danger`) · `UiTabs` (`items`, v-model) · `UiTooltip` (`content`) ·
`UiSkeleton` (`rows`) · `UiEmptyState` (`icon`, `title`, `description`) ·
`UiAlert` (`tone`, `title`, `dismissible`).

Compose these plus Tailwind utilities. Add a new primitive only when three or
more views need it.

## Data access

All calls go through `endpoints` in `src/api.ts` — never `fetch('/api/...')`
directly, and never hardcode `http://127.0.0.1:8080`. If a response shape is
wrong, fix the interface in `src/api.ts` to match the Go handler.

Console pages fetch on the client (they are behind auth and not prerendered):

```ts
const { data, pending, error, refresh } = useResource(() => endpoints.getAccountKeys(), { data: [] })
const { busy, run } = useAction()
const { toast } = useToast()

async function revoke(id: string) {
  const ok = await run(() => endpoints.revokeAccountKey(id))
  if (ok) { toast.success(t('console.keyRevoked')); await refresh() }
  else toast.error(t('common.actionFailed'))
}
```

Every console page starts with:

```ts
definePageMeta({ layout: 'console', middleware: 'console-auth' })
```

Permission-gated views also check `useAccount().can('users.read')` and render
an `UiEmptyState` instead of the content when the check fails.

## Every list view needs four states

Loading (`UiSkeleton`), empty (`UiEmptyState`), error (`UiAlert tone="danger"`),
and loaded. Do not ship a table that renders nothing while pending.

## Formatting

Use `src/format.ts` — `formatNumber` `formatCompact` `formatMoney`
`formatPercent` `formatSignedPercent` `formatDateTime` `formatDate` `shortId`.
Numeric table cells get `class="num"`; inline figures get the `numeric` utility.

## Things to avoid

- No comments that restate the code. Comment only a non-obvious constraint.
- No new npm dependencies.
- No `any`. Type API results from `src/api.ts` interfaces.
- No `dark:` variants for colour — the tokens already switch.
- No hardcoded strings, no hardcoded colours, no `console.log` left behind.

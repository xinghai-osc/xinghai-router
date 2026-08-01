# AGENTS.md

Guidance for AI coding agents (opencode, Claude Code, etc.) working in this repository.

## Project overview

Xinghai Router is an LLM gateway and operations console. The Go service (`cmd/router`) exposes an OpenAI-compatible gateway (`/v1/*`), an Anthropic-compatible gateway (`/v1/messages`), account APIs (`/auth/*`, `/account/*`), public APIs (`/rankings`, `/subscription-plans`, `/model-catalog`, `/site-settings`), and admin APIs (`/admin/*`). A Nuxt 3 console in `web/` proxies `/api/*` to the Go service. PostgreSQL is the source of truth; Redis is used for shared API-key rate limiting when `REDIS_URL` is set (memory fallback otherwise). Channel (provider) API keys are stored plaintext; merchant keys and other service secrets are encrypted at rest with `ENCRYPTION_KEY`.

The repository is bilingual: README and user-facing copy are in Chinese and English. Match the language of the surrounding content when editing; do not translate existing strings unless asked.

## Repository layout

```
cmd/router/        Go entrypoint (main.go)
internal/app/      All Go application code (single package `app`)
  migrations/      Embedded SQL migrations, applied at startup (sorted by filename)
  *.go             HTTP handlers, gateway proxy, providers, reliability, payments, subscriptions
web/               Nuxt 3 + Vue 3 management console (see web/AGENTS.md)
  assets/css/      tokens.css, base.css, utilities.css, themes/*.css
  components/      ui/* (design-system primitives), site/*, console/*, marketplace/*
  composables/     useI18n, useTheme, useAccount, useToast, useResource, useSiteSettings
  layouts/         default.vue (public), console.vue (signed-in)
  pages/           File-based routes (index, models, rankings, pricing, auth, console/*)
  server/api/      [...path].ts proxies /api/* to the Go service
  src/             api.ts (typed API client + interfaces), locales/, nav.ts, format.ts, marketplace.ts
docker-compose.yml PostgreSQL 17, Redis 7, router, web
Dockerfile         Multi-stage build for router (Go) and web (Nuxt)
.env.example       Required env vars; copy to .env and replace secrets
```

## Tech stack and key libraries

- Go 1.26, module `github.com/xinghai-osc/xinghai-router`. Only stdlib plus `github.com/jackc/pgx/v5` (pgxpool) and `golang.org/x/crypto` (bcrypt). Do not introduce new dependencies without strong reason.
- Web: Nuxt 3 (`nuxt`), Vue 3, TypeScript, Tailwind CSS v4 (via `@tailwindcss/vite`), `reka-ui` (headless primitives), `lucide-vue-next`, `clsx` + `tailwind-merge`, `@vueuse/core`. Package manager is pnpm. ESLint is configured via flat config (`web/eslint.config.mjs` wrapping `@nuxt/eslint`); no test runner is configured for the web app.
- DB: PostgreSQL 17. Migrations are plain `.sql` files embedded with `//go:embed migrations/*.sql` and applied idempotently by `internal/app/migrate.go`.

## Build and run

Local Go service (requires running Postgres + `DATABASE_URL` + `ENCRYPTION_KEY`):

```sh
docker compose up -d            # postgres + redis
cp .env.example .env            # then edit secrets
set -a; . ./.env; set +a        # export env for the shell
go run ./cmd/router             # http://localhost:8080
```

Full stack with Docker:

```sh
cp .env.example .env && docker compose up -d --build
```

Web console dev server (run the Go service first):

```sh
cd web && pnpm install && pnpm run dev   # http://localhost:5173, proxies /api/* to :8080
```

## Verification commands

Always run these before considering Go work done:

```sh
go build ./...
go vet ./...
go test ./...
```

From `web/`, `pnpm run build` validates the Nuxt app and `pnpm run generate` emits prerendered pages. Lint is configured via ESLint flat config (`web/eslint.config.mjs`, wrapping `@nuxt/eslint`); run it before considering web work done:

```sh
cd web && pnpm run lint        # nuxt prepare && eslint .
cd web && pnpm run lint:fix    # auto-fix stylistic rules
```

There is no web test script. No `vue-tsc` typecheck or Prettier is wired in yet.

## Conventions

### Go

- Everything lives in one package, `internal/app`. Add new handlers there; do not split into subpackages without reason.
- HTTP routing uses Go 1.22+ method-pattern `http.ServeMux` (`mux.HandleFunc("GET /path", s.handler)`). See `routes.go` for the canonical pattern and middleware order (`s.optionalAccount`, `s.account`, `s.permission("perm", handler)`).
- Handlers read request bodies with `io.LimitReader(r.Body, 2<<20)` and `decode(r, &v)`; respond with `writeJSON(w, status, body)` or `writeError(w, status, code, msg)`. Match this style.
- DB access goes through `s.db` (`*pgxpool.Pool`) using `QueryRow`/`Query`/`Exec` with `$1, $2, ...` placeholders. Never build SQL by string-concatenating user input.
- Secrets: API keys are hashed (`hashSecret`) and only the full key is returned once at creation. Channel (provider) keys are stored plaintext; reads go through `channelKeyValue`, which transparently decrypts rows written before the switch to plaintext. Merchant keys, SMTP/Geetest credentials, and OAuth client secrets are encrypted with `crypt(ENCRYPTION_KEY, value, false)` and decrypted on use. Never log or return channel keys, decrypted secrets, or merchant keys.
- New schema changes ship as a new `internal/app/migrations/NNNN_name.sql` file (zero-padded, incrementing). Migrations must be idempotent-safe within their own statements and are wrapped in a transaction by `migrate.go`. Never edit an applied migration in a way that breaks already-deployed databases; add a new migration instead.
- Embeds: any new migration file is picked up automatically by the `//go:embed migrations/*.sql` directive — no registration needed.
- Error wrapping: use `fmt.Errorf("...: %w", err)` for propagated errors.
- Keep files focused; large handlers (e.g. `admin.go`, `gateway.go`, `subscriptions.go`) are already big, so prefer adding focused new files for substantial new features.

### Web (Nuxt / Vue / TypeScript)

**`web/AGENTS.md` is the authoritative guide for this directory — read it before touching any file under `web/`.** The essentials:

- File-based routing under `web/pages`; each console view is its own file under `web/pages/console/`, and each declares `definePageMeta({ layout: 'console', middleware: 'console-auth' })`.
- Global state is per-concern composables — `useAccount`, `useTheme`, `useI18n`, `useToast`, `useSiteSettings`, `useCatalog`, `usePlans`. Views fetch their own data with `useResource` and mutate with `useAction`. There is no single console store.
- Typed API client and DTOs live in `web/src/api.ts`. When adding/changing a backend endpoint, update the interfaces there to match the Go response JSON exactly.
- Browser requests go through `/api/*` and are proxied to the Go service by `web/server/api/[...path].ts` (Nuxt) — do not hardcode `http://127.0.0.1:8080` in client code.
- CSS lives in `web/assets/css/`: `tokens.css` (Tailwind `@theme inline` mapping + non-colour tokens), `themes/{default,cool,galaxy}.css` (colour presets), `base.css`, `utilities.css`. Colour is addressed only through semantic utilities — `bg-paper`, `bg-surface`, `bg-sunken`, `text-ink`, `text-muted`, `text-faint`, `border-line`, `bg-clay`, and the `success`/`warn`/`danger` tokens. Never hardcode a hex or a stock Tailwind colour.
- Theming has two axes on `<html>`: `data-theme="light|dark"` and `data-preset="default|cool|galaxy"`, both set by `composables/useTheme.ts` plus a no-flash inline script in `app.vue`. Every preset defines its own light and dark values, so `dark:` variants are not needed for colour.
- UI primitives are hand-rolled in `web/components/ui/` (one component per file, auto-imported as `<UiButton>`, `<UiCard>`, …) over `reka-ui` headless primitives. Compose these plus Tailwind utilities; add a new primitive only when three or more views need it.
- `cn` helper is `web/lib/utils.ts` — `clsx` + `tailwind-merge`. Use it for class composition.
- **i18n is mandatory.** Every user-visible string, including `aria-label`/`placeholder`/`title`, goes through `t()` from `useI18n()`. Messages live in `web/src/locales/{zh,en}/<namespace>.ts` (`common`, `nav`, `site`, `auth`, `console`, `admin`, `system`, `theme`); the two locales must stay in sync key-for-key. Data returned by the API is passed through untranslated.
- Console navigation and its permission gates are declared in `web/src/nav.ts`, not in the sidebar component.
- Prerendered public routes are declared in `nuxt.config.ts` (`nitro.prerender.routes` and `routeRules`); update both when adding prerendered pages.

### Security

- `ENCRYPTION_KEY` must be ≥24 chars and kept stable across restarts; losing it makes encrypted merchant keys and other service secrets unrecoverable. Channel API keys are plaintext and do not depend on it.
- `base_url` for channels accepts HTTP or HTTPS to any host (validated by `validUpstreamURL`), so admins can point at plaintext or private-network upstreams. Payment and site icon URLs stay stricter (`validOutboundURL`: HTTPS to a public host, or loopback HTTP).
- `/admin/*` endpoints require an authenticated session plus a specific permission (`users.read`, `keys.manage`, `channels.read/manage`, `logs.read`, `audit.read`, `pricing.read/manage`, `wallets.manage`, `routes.manage`, `quotas.manage`, `system.manage`). Use the existing `s.permission("...", handler)` wrapper.
- Gateway endpoints authenticate API keys via `Authorization: Bearer $KEY` (OpenAI) or `x-api-key: $KEY` (Anthropic). Anthropic requests also require `anthropic-version`.
- Passwords: bcrypt only; minimum 8 chars. Session tokens are 7-day bearer tokens.
- Payment/subscription notifications: only the signed async `epay/notify` callback credits balances/subscriptions, and it must be idempotent on `order_no`. Browser sync returns must never credit.

### Known limits (do not "fix" silently)

- The rate limiter uses Redis fixed-window counters when `REDIS_URL` is set (`internal/app/redis_limiter.go`), with automatic fallback to the in-process limiter (`internal/app/limiter.go`) if Redis is unreachable.
- Streaming (SSE) responses are parsed for usage events and billed after the stream closes. OpenAI upstreams receive `stream_options.include_usage: true` so the final chunk carries `usage`; Anthropic events carry usage in `message_start` (input/cache) and `message_delta` (output). If a stream yields no usage events, no tokens are recorded or billed for that request.
- Balance reservation happens before the upstream call to prevent concurrent overspend; releases/refunds must go through the wallet ledger (`internal/app/gateway.go`).

## Git and commits

- Follow the existing commit format: `feat: ...`, `fix: ...`, `chore: ...` (see `git log`). Subject in English, concise.
- Never commit `.env`, `web/node_modules`, `web/.nuxt`, `web/.output`, `web/dist`, the compiled `router` binary, or `*.tar.gz` build artifacts — they are in `.gitignore`.
- Do not commit secrets. If a test needs credentials, use clearly-fake placeholders or read from env.

## Things to avoid

- Don't add third-party Go or npm dependencies for things the stdlib / existing stack already does.
- Don't introduce a CORS policy for the dev setup — Nuxt proxying `/api/*` is intentional.
- Don't add `/v1` to a channel `base_url`; it is appended by the provider adapters.
- Don't edit applied migrations destructively, or expose encrypted/decrypted secrets via any API.
- Don't add comments to code unless explicitly requested.
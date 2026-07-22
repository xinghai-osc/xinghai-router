# AGENTS.md

Guidance for AI coding agents (opencode, Claude Code, etc.) working in this repository.

## Project overview

Xinghai Router is an LLM gateway and operations console. The Go service (`cmd/router`) exposes an OpenAI-compatible gateway (`/v1/*`), an Anthropic-compatible gateway (`/v1/messages`), account APIs (`/auth/*`, `/account/*`), public APIs (`/rankings`, `/subscription-plans`, `/model-catalog`, `/site-settings`), and admin APIs (`/admin/*`). A Nuxt 3 console in `web/` proxies `/api/*` to the Go service. PostgreSQL is the source of truth; Redis is provisioned in `docker-compose.yml` but not yet wired in (in-process limiter only). Provider credentials are encrypted at rest with `ENCRYPTION_KEY`.

The repository is bilingual: README and user-facing copy are in Chinese and English. Match the language of the surrounding content when editing; do not translate existing strings unless asked.

## Repository layout

```
cmd/router/        Go entrypoint (main.go)
internal/app/      All Go application code (single package `app`)
  migrations/      Embedded SQL migrations, applied at startup (sorted by filename)
  *.go             HTTP handlers, gateway proxy, providers, reliability, payments, subscriptions
web/               Nuxt 3 + Vue 3 management console
  pages/           File-based routes (auth.vue, console/*, index.vue, rankings.vue, ...)
  components/      console/* (admin/user UI), marketplace/* (model square)
  composables/     useConsoleStore, useTheme, useI18n
  server/api/      [...path].ts proxies /api/* to the Go service
  src/             api.ts (typed API client + interfaces), views.ts, marketplace.ts, style.css
docker-compose.yml PostgreSQL 17, Redis 7, router, web
Dockerfile         Multi-stage build for router (Go) and web (Nuxt)
.env.example       Required env vars; copy to .env and replace secrets
```

## Tech stack and key libraries

- Go 1.26, module `github.com/xinghai-osc/xinghai-router`. Only stdlib plus `github.com/jackc/pgx/v5` (pgxpool) and `golang.org/x/crypto` (bcrypt). Do not introduce new dependencies without strong reason.
- Web: Nuxt 3 (`nuxt`), Vue 3, TypeScript, Tailwind CSS v4 (via `@tailwindcss/vite`), shadcn-vue (reka-ui + `class-variance-authority` + `clsx` + `tailwind-merge`), `lucide-vue-next`, `@lobehub/icons-static-svg`. ESLint is configured via flat config (`web/eslint.config.mjs` wrapping `@nuxt/eslint`); no test runner is configured for the web app.
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
cd web && npm install && npm run dev   # http://localhost:5173, proxies /api/* to :8080
```

## Verification commands

Always run these before considering Go work done:

```sh
go build ./...
go vet ./...
go test ./...
```

From `web/`, `npm run build` validates the Nuxt app and `npm run generate` emits prerendered pages. Lint is configured via ESLint flat config (`web/eslint.config.mjs`, wrapping `@nuxt/eslint`); run it before considering web work done:

```sh
cd web && npm run lint        # eslint .  (errors fail, warnings do not)
cd web && npm run lint:fix    # auto-fix stylistic rules (html-self-closing, etc.)
```

There is no web test script. No `vue-tsc` typecheck or Prettier is wired in yet.

## Conventions

### Go

- Everything lives in one package, `internal/app`. Add new handlers there; do not split into subpackages without reason.
- HTTP routing uses Go 1.22+ method-pattern `http.ServeMux` (`mux.HandleFunc("GET /path", s.handler)`). See `routes.go` for the canonical pattern and middleware order (`s.optionalAccount`, `s.account`, `s.permission("perm", handler)`).
- Handlers read request bodies with `io.LimitReader(r.Body, 2<<20)` and `decode(r, &v)`; respond with `writeJSON(w, status, body)` or `writeError(w, status, code, msg)`. Match this style.
- DB access goes through `s.db` (`*pgxpool.Pool`) using `QueryRow`/`Query`/`Exec` with `$1, $2, ...` placeholders. Never build SQL by string-concatenating user input.
- Secrets: API keys are hashed (`hashSecret`) and only the full key is returned once at creation. Provider keys are encrypted with `crypt(ENCRYPTION_KEY, value, false)` and decrypted on use. Never log or return decrypted provider secrets or merchant keys.
- New schema changes ship as a new `internal/app/migrations/NNNN_name.sql` file (zero-padded, incrementing). Migrations must be idempotent-safe within their own statements and are wrapped in a transaction by `migrate.go`. Never edit an applied migration in a way that breaks already-deployed databases; add a new migration instead.
- Embeds: any new migration file is picked up automatically by the `//go:embed migrations/*.sql` directive — no registration needed.
- Error wrapping: use `fmt.Errorf("...: %w", err)` for propagated errors.
- Keep files focused; large handlers (e.g. `admin.go`, `gateway.go`, `subscriptions.go`) are already big, so prefer adding focused new files for substantial new features.

### Web (Nuxt / Vue / TypeScript)

- File-based routing under `web/pages`; admin/user console lives under `web/pages/console` with the active view selected by `[view].vue` and dedicated routes for `subscriptions`, `subscription-plans`, `admin-subscriptions`.
- Console state is in `composables/useConsoleStore.ts`; theming in `composables/useTheme.ts`; i18n in `composables/useI18n.ts`. Reuse these rather than introducing global state.
- Typed API client and DTOs live in `web/src/api.ts`. When adding/changing a backend endpoint, update the interfaces there to match the Go response JSON exactly.
- Browser requests go through `/api/*` and are proxied to the Go service by `web/server/api/[...path].ts` (Nuxt) — do not hardcode `http://127.0.0.1:8080` in client code.
- CSS is a single global stylesheet at `web/src/style.css`, imported via `nuxt.config.ts` `css`. It loads Tailwind v4 (`@import "tailwindcss"`) + `tw-animate-css` and exposes the shadcn-vue token system through an `@theme inline` block, so `bg-background`/`text-foreground`/`bg-primary`/`border-border`/`ring-ring` utilities resolve to the HSL `--background`/`--foreground`/... variables defined on `:root`. Dark mode is driven by `data-theme="dark"` on `<html>` (set by `composables/useTheme.ts` + a no-flash inline script in `app.vue`); Tailwind's `dark:` variant is routed to that attribute via `@custom-variant dark (&:where([data-theme="dark"], [data-theme="dark"] *))`. Multi-color / multi-radius / `a-site` preset switching still works through `:root[data-theme-color="..."]`, `:root[data-theme-radius="..."]`, `:root[data-theme-preset="a-site"]` overriding the same tokens.
- shadcn-vue base components live in `web/components/ui/<name>/` (added via the `shadcn-vue` CLI, config in `web/components.json`). They import `cn` from `@/lib/utils` (which resolves to `web/lib/utils.ts` through Nuxt's default `@` alias). Prefer composing these primitives + Tailwind utilities for new UI rather than extending the hand-rolled classes in `style.css`. The legacy hand-rolled classes (`.button`, `.panel`, `.state`, `.msq-*`, ...) still work because they consume the same HSL tokens, and are intended to be incrementally replaced by shadcn-vue primitives.
- `cn` helper is `web/lib/utils.ts` (not `src/lib/`) — `clsx` + `tailwind-merge`. Use it for class composition in new components.
- Prerendered public routes are declared in `nuxt.config.ts` (`nitro.prerender.routes` and `routeRules`); update both when adding prerendered pages.

### Security

- `ENCRYPTION_KEY` must be ≥24 chars and kept stable across restarts; losing it makes encrypted provider credentials and merchant keys unrecoverable.
- `base_url` for channels must be HTTPS, except loopback HTTP (`127.0.0.1` / `localhost`) for local services like Ollama. Validate with `isLoopbackHost` before accepting HTTP origins.
- `/admin/*` endpoints require an authenticated session plus a specific permission (`users.read`, `keys.manage`, `channels.read/manage`, `logs.read`, `audit.read`, `pricing.read/manage`, `wallets.manage`, `routes.manage`, `quotas.manage`, `system.manage`). Use the existing `s.permission("...", handler)` wrapper.
- Gateway endpoints authenticate API keys via `Authorization: Bearer $KEY` (OpenAI) or `x-api-key: $KEY` (Anthropic). Anthropic requests also require `anthropic-version`.
- Passwords: bcrypt only; minimum 8 chars. Session tokens are 7-day bearer tokens.
- Payment/subscription notifications: only the signed async `epay/notify` callback credits balances/subscriptions, and it must be idempotent on `order_no`. Browser sync returns must never credit.

### Known limits (do not "fix" silently)

- The rate limiter uses Redis fixed-window counters when `REDIS_URL` is set (`internal/app/redis_limiter.go`), with automatic fallback to the in-process limiter (`internal/app/limiter.go`) if Redis is unreachable.
- Streaming (SSE) responses are passed through transparently and are **not** settled against the wallet; only non-stream requests record tokens and bill. Do not introduce streaming billing without solving inconsistent upstream SSE usage events.
- Balance reservation happens before the upstream call to prevent concurrent overspend; releases/refunds must go through the wallet ledger (`internal/app/gateway.go`).

## Git and commits

- Follow the existing commit format: `feat: ...`, `fix: ...`, `chore: ...` (see `git log`). Subject in English, concise.
- Never commit `.env`, `web/node_modules`, `web/.nuxt`, `web/.output`, `web/dist`, the compiled `router` binary, or `*.tar.gz` build artifacts — they are in `.gitignore`.
- Do not commit secrets. If a test needs credentials, use clearly-fake placeholders or read from env.

## Things to avoid

- Don't add third-party Go or npm dependencies for things the stdlib / existing stack already does.
- Don't introduce a CORS policy for the dev setup — Nuxt proxying `/api/*` is intentional.
- Don't add `/v1` to a channel `base_url`; it is appended by the provider adapters.
- Don't bill streaming requests, edit applied migrations destructively, or expose encrypted/decrypted secrets via any API.
- Don't add comments to code unless explicitly requested.
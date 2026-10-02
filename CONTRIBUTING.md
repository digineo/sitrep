# Contributing to SitRep

## Setup

You need Go (see `go.mod` for the version), Node.js 24, git and make.

```sh
cat > .env.local <<'ENV'
SITREP_BASE_DOMAINS=sitrep.localhost
SITREP_AUTH=bypass
ENV
make dev
```

`make dev` runs the Vite dev server and the Go server with live reload.
Browse `http://sitrep.localhost:2607/`: the Go server proxies `/assets/`
to Vite, so hot module replacement works on that single URL. Development
builds contain the `bypass` auth provider, which signs you in without a
password; release builds never do.

## Conventions

- Commits follow [Conventional Commits](https://www.conventionalcommits.org/)
  and are written in English.
- Code, identifiers and comments are in English. Every user-facing text
  lives in the catalogs in `locales/`, complete in every language.
- Go code is checked by `go vet` and golangci-lint, the frontend by ESLint
  and vue-tsc. `make lint` runs all of them.
- Frontend views build on the shared `PT*` components and `use*`
  composables in `frontend/src/shared/` instead of repeating markup and
  logic.
- Every rule that is not trivial gets a regression test.

## Tests

| Command | Runs |
|---|---|
| `make test-backend` | Go tests, also with the `testauth` build tag |
| `make test-frontend` | Vitest component and unit tests |
| `make test-e2e` | Playwright tests against a `testauth` build; install the browser once with `cd frontend && npx playwright install chromium` |
| `make test` | all of the above |

## Adding a language

Add `locales/<code>.json`, where the code is a lowercase tag like `fr` or
`pt-br`. Copy `locales/en.json`, the reference catalog, and translate every
message. Keep the `{placeholders}` as they are, and pick URL slugs for the
legal pages (`legal.imprint.slug`, `legal.privacy.slug`) that are lowercase
words joined by dashes. Nothing else needs to change: the server and both
frontends pick up every catalog file. The Go tests fail if keys or
placeholders differ from English or the slugs are invalid.

## Adding an auth provider

1. Create a package `internal/auth/<id>` that implements `auth.Provider`:
   its login method (`redirect` or `credentials`), whether logins
   currently work, and its HTTP routes below `/auth/<id>/`.
2. Register it in an `init` function with `auth.Register("<id>", New)`.
   `New` reads and validates the provider's own environment variables
   through the `config.Env` it receives.
3. On successful authentication, call `core.Login` with the identity. The
   core creates the session and sets the cookie.
4. Add one blank import to `cmd/sitrep/providers.go`.

The core applies CSRF checks to all routes below `/auth/`.

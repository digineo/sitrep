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
password; release builds never do. It signs in `test-admin`, or with
`/auth/bypass/login?as=<user>` `test-maintainer` or `test-responder`.

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
| `make test-e2e` | Playwright tests against a `testauth` build, a stub of the Prometheus API (`frontend/e2e/prometheus.ts`) and a mock OpenID provider (`frontend/e2e/idp.ts`); install the browser once with `cd frontend && npx playwright install chromium` |
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
   currently work, and its HTTP routes below `/auth/<id>/`. `Routes` is
   called once at startup and may start background work.
2. Register it in an `init` function with `auth.Register("<id>", New)`.
   `New` reads and validates the provider's own environment variables
   through the `config.Env` it receives.
3. On successful authentication, call `core.Login` with the identity. The
   core signs in the identity's account, creating it if needed, creates
   the session and sets the cookie. Set the identity's email only if the
   provider verified it: accounts added by email are bound to the first
   identity with that email.
4. Add one blank import to `cmd/sitrep/providers.go`.

A provider that knows all its users, like `basic`, implements
`auth.Directory`. Accounts are then added by username instead of email.

The login screen calls the provider's `/auth/<id>/login`:

- **redirect:** a link with `?return=<console path>`. Pass the path
  through `auth.ReturnPath` and send the browser back to it after signing
  in. On failure, return with `login-error=<code>` in the query; the login
  screen has notices for `denied`, `not_member`, `idp_error` and
  `unavailable`.
- **credentials:** a JSON post of `{"username", "password"}`, answered
  with 204 on success, 401 for wrong credentials and 429 when throttled.

The core applies CSRF checks to all routes below `/auth/`.

## Adding a data source type

1. Create a package `internal/datasource/<id>` that implements
   `datasource.Type`: its configuration fields, the panel types it feeds,
   evaluation of a panel's query into a normalized result (a scalar,
   labeled samples, or labeled series on a shared time axis), a connection
   test and a summary for lists. Read at most `datasource.MaxResponse`
   bytes per response and report the size read in `Result.Bytes`; large
   responses get a warning in the console. Reduce, thresholds, series
   handling, caching and polling are shared and need no code.
2. Optionally implement `datasource.Router` for admin-only routes below
   `/api/admin/datasources/{id}/<id>/`, and `datasource.Editor` to pick a
   query editor in the console. Without an editor hint, queries get a plain
   text area.
3. Register it in an `init` function with `datasource.Register("<id>", Type{})`.
4. Add one blank import to `cmd/sitrep/datasources.go`.
5. Add the type's texts to every catalog: `datasource.<id>.name`, and per
   field `datasource.<id>.<field>.label`, an optional `.help`, and
   `.options.<value>` for select fields. The console renders the
   configuration form from the field description.

Secret fields are encrypted by the core; the type receives them in plain
text when it evaluates or tests. A change of any `url` field discards the
stored secrets, so the type can rely on a secret only ever going to the URL
it was entered for.

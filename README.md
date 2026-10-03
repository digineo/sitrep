# SitRep

SitRep is a self-hosted, multi-tenant server for public status pages. It
ships as one binary with the web frontend and an embedded database file.

> SitRep is under active development. This README describes what works
> today.

## Features

- One instance serves many status pages, reachable below a base domain
  path, as subdomain or on their own domain.
- Fully localized: German and English ship with the binary, adding a
  language means adding one catalog file. Visitors get their language from
  the URL, their earlier choice or their browser.
- Panels fed by Prometheus queries: single values, up/down states by
  thresholds, and charts. SitRep polls the data sources and serves
  visitors from memory; visitors never cause queries.
- Status pages refresh themselves and fetch only what changed.
- Light, dark and system color schemes, remembered per visitor.
- Admin console with sign-in by username and password (bcrypt or argon2id
  hashes), login throttling, data sources, status pages with their panels
  and a live preview, and instance settings for languages and the default
  color scheme.
- No third-party requests, no tracking, no consent banner needed.

## Quick start

```sh
make build
./sitrep hash-password -user admin > users   # asks for the password twice
SITREP_BASE_DOMAINS=status.example.com \
SITREP_AUTH=basic \
SITREP_BASIC_USERS_FILE=users \
./sitrep serve
```

The console is at `http://status.example.com:2607/admin`. For a local try,
use `sitrep.localhost` as base domain: browsers resolve every
`*.localhost` name to your machine.

## Configuration

All configuration comes from environment variables. Optional `.env.local`
and `.env` files in the working directory are read too, in that order of
priority; real environment variables always win. The files hold lines of
`KEY=VALUE`, optionally prefixed with `export`; values may be quoted with
single or double quotes (without escape sequences), and unquoted values end
at ` #`. For a quick setup, copy the documented `.env.sample` to
`.env.local` and fill in the blanks.

Invalid or missing values are reported together, and the server does not
start. Booleans accept `true/false/1/0/yes/no/on/off`. Durations combine
the units `d`, `h`, `m` and `s` in this order, e.g. `90s`, `1h30m` or `7d`.

| Variable | Default | Meaning |
|---|---|---|
| `SITREP_LISTEN` | `:2607` | HTTP listen address. |
| `SITREP_DB` | `sitrep.db` | Database file. One instance per file. |
| `SITREP_SECRET_KEY` | | Base64-encoded 32-byte key for data source secrets. Generate with `openssl rand -base64 32`. |
| `SITREP_DEFAULT_REFRESH` | `30s` | Default poll interval, 5s to 24h. |
| `SITREP_BASE_DOMAINS` | required | Comma-separated base domains, e.g. `status.example.com`. |
| `SITREP_TRUST_PROXY` | `false` | Honor `X-Forwarded-Host`, `X-Forwarded-Proto` and `X-Forwarded-For`. Enable only behind a trusted proxy. |
| `SITREP_AUTH` | `oidc` | Auth provider: `oidc` or `basic`. |
| `SITREP_SESSION_TTL` | `12h` | Admin session lifetime, 5m to 30d. |
| `SITREP_OIDC_ISSUER` | required for oidc | Issuer URL. |
| `SITREP_OIDC_CLIENT_ID` | required for oidc | Client ID. |
| `SITREP_OIDC_CLIENT_SECRET` | | Client secret, for confidential clients. |
| `SITREP_OIDC_REDIRECT_URL` | required for oidc | `https://<base domain>/auth/oidc/callback` |
| `SITREP_OIDC_SCOPES` | `openid profile email` | Space-separated scopes. |
| `SITREP_OIDC_GROUPS_CLAIM` | `groups` | Claim with the user's groups. |
| `SITREP_OIDC_ADMIN_GROUP` | required for oidc | Group required to sign in. |
| `SITREP_BASIC_USERS_FILE` | required for basic | Users file, see below. |
| `SITREP_LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error`. |
| `SITREP_LOG_FORMAT` | `text` | `text`, `json` or `pretty` (colored console output). |

The OIDC provider currently validates its configuration but cannot sign
anyone in yet; its login screen says that sign-in is unavailable.

## Signing in with username and password

With `SITREP_AUTH=basic`, admins are listed in an htpasswd-style file, one
`username:hash` per line. Blank lines and lines starting with `#` are
ignored. Every user in the file is an admin. SitRep reloads the file when
it changes; a broken file keeps the previous users and logs an error.

Create argon2id hashes with SitRep, or bcrypt hashes with Apache's
`htpasswd`:

```sh
./sitrep hash-password -user alice >> users
htpasswd -B -C 12 users bob
```

bcrypt needs a cost of at least 10. argon2id hashes need at least
m=19456, t=2 and p=1, and at most m=1048576, t=10 and p=16.

Failed attempts are counted per username and, independently, per client
address. After five failures within 15 minutes for a username, or from an
address, further attempts for that username, or from that address, are
refused for 15 minutes. SitRep keeps addresses only as keyed hash under a
secret that changes daily; the change resets the address counters.

At most four password verifications run at a time; an attempt that cannot
start one within five seconds is refused as too many attempts. Each argon2id
verification allocates its memory parameter, so hashes with m=1048576 (1 GiB)
can make logins use up to 4 GiB. The default of `sitrep hash-password`,
m=65536, needs 64 MiB each. Usernames are limited to 200 bytes.

## Data sources

Panels query data sources, which you configure in the console under "Data
sources". SitRep ships the Prometheus type: a base URL (a path prefix is
allowed), optional basic or bearer authentication, and a request timeout
of 1s to 2m (default 10s). Requests never follow redirects, and responses
over 10 MiB are refused. Panels whose responses exceed 1 MiB still work,
but the console warns about them: they load the backend and enlarge the
public page.

Passwords and tokens are stored encrypted with `SITREP_SECRET_KEY` and are
never shown again. They are bound to the URL they were entered for: saving
another URL removes them unless you enter them again, so credentials are
never sent to another host. Without the key, data sources cannot store
secrets. If the key is lost or changed, data sources with secrets become
unusable: their panels show no new data and the console marks them until
you enter the secrets again. Public pages keep working.

Admins can point a data source at any URL the server reaches. That is fine
because admins are trusted anyway: they also run arbitrary queries.

Each panel is polled at once and then at its refresh interval, by default
`SITREP_DEFAULT_REFRESH`. Panels of a status page are polled whether or
not anyone visits it.

## URLs and languages

A base domain host serves the landing page at `/`, the console at `/admin`
and path-mode status pages at `/<slug>/`. Subdomain-mode pages answer on
`<slug>.<base domain>`, custom-domain pages on their own host.

With one enabled language, URLs carry no language: `/`, `/incidents`,
`/imprint`. With several, they start with it: `/en/`, `/de/incidents`,
`/de/impressum`. Legal pages use slugs in their language. Unprefixed URLs
redirect to the visitor's language: the one chosen in the language switcher
(cookie `lang`), else the best match of the browser's languages, else the
primary language. Following a link to another language does not change the
stored choice.

## Cookies

None of these cookies is needed to read public pages, and none tracks
anyone:

| Cookie | Set when | Content | Lifetime |
|---|---|---|---|
| `theme` | a visitor picks a color scheme | `light`, `dark` or `system` | 1 year |
| `lang` | a visitor picks a language | the language code | 1 year |
| `sitrep_session`, on HTTPS `__Host-sitrep_session` | an admin signs in | a random session token | the session lifetime |

## Stored personal data

SitRep stores the subject, display name and, if the identity provider
sends one, the email address of signed-in admins in their session. Sessions
expire after `SITREP_SESSION_TTL` and are deleted at startup and hourly.
Logs contain the usernames of failed sign-ins, but no client addresses at
level `info` or above.

## Development

See [CONTRIBUTING.md](CONTRIBUTING.md). `make help` lists the targets.

## License

MIT, see [LICENSE](LICENSE).

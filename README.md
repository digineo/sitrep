# SitRep

SitRep is a self-hosted, multi-tenant server for public status pages. It
ships as one binary with the web frontend and an embedded database file.

## Features

- One instance serves many status pages, reachable below a base domain
  path, as subdomain or on their own domain.
- Fully localized: German and English ship with the binary, adding a
  language means adding one catalog file. Visitors get their language from
  the URL, their earlier choice or their browser.
- Panels fed by Prometheus queries: single values, up/down states by
  thresholds, and charts. SitRep polls the data sources and serves
  visitors from memory; visitors never cause queries.
- Incidents and maintenance with a timeline of updates written in Markdown,
  shown on the status page, shaded in charts, in an archive, in an Atom
  feed per language and as JSON for other websites.
- Status pages refresh themselves and fetch only what changed.
- Imprint and privacy statement per status page or for the whole instance,
  as Markdown text or link, and a landing page text.
- A brand color and an SVG logo per status page; the favicon shows the
  status.
- Status pages can be taken offline or paused, and exported to and
  imported from YAML files.
- Light, dark and system color schemes, remembered per visitor.
- Admin console with sign-in by single sign-on (OpenID Connect, limited to
  an admin group) or by username and password (bcrypt or argon2id hashes,
  with login throttling). It manages data sources, status pages with their
  panels, incidents and a live preview, and instance settings for
  languages, the default color scheme, legal pages and the landing text.
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
start. A variable that is set but empty is an error, except for the two
optional secrets, which then count as unset. Booleans accept
`true/false/1/0/yes/no/on/off`. Durations combine the units `d`, `h`, `m`
and `s` in this order, e.g. `90s`, `1h30m` or `7d`. Only the variables of
the selected auth provider are read.

| Variable | Default | Meaning |
|---|---|---|
| `SITREP_LISTEN` | `:2607` | HTTP listen address. |
| `SITREP_DB` | `sitrep.db` | Database file. One instance per file. |
| `SITREP_SECRET_KEY` | | Base64-encoded 32-byte key for data source secrets, see [Data sources](#data-sources). Generate with `openssl rand -base64 32`. |
| `SITREP_DEFAULT_REFRESH` | `30s` | Default poll interval, 5s to 24h. |
| `SITREP_BASE_DOMAINS` | required | Comma-separated base domains, e.g. `status.example.com`: lowercase hostnames with at least two labels, each listed once. |
| `SITREP_TRUST_PROXY` | `false` | Honor `X-Forwarded-Host`, `X-Forwarded-Proto` and `X-Forwarded-For`. Enable only behind a trusted proxy. |
| `SITREP_AUTH` | `oidc` | Auth provider: `oidc` or `basic`. |
| `SITREP_SESSION_TTL` | `12h` | Admin session lifetime, 5m to 30d. |
| `SITREP_OIDC_ISSUER` | required for oidc | Issuer URL, exactly as the identity provider reports it. |
| `SITREP_OIDC_CLIENT_ID` | required for oidc | Client ID. |
| `SITREP_OIDC_CLIENT_SECRET` | | Client secret, for confidential clients. |
| `SITREP_OIDC_REDIRECT_URL` | required for oidc | `https://<base domain>/auth/oidc/callback`; the host must be a base domain. |
| `SITREP_OIDC_SCOPES` | `openid profile email` | Space-separated scopes; `openid` is always requested. |
| `SITREP_OIDC_GROUPS_CLAIM` | `groups` | Claim with the user's groups: a list or a single string. |
| `SITREP_OIDC_ADMIN_GROUP` | required for oidc | Group required to sign in. |
| `SITREP_BASIC_USERS_FILE` | required for basic | Users file, see below. |
| `SITREP_LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error`. |
| `SITREP_LOG_FORMAT` | `text` | `text`, `json` or `pretty` (colored console output). |

The startup log shows the effective configuration, with secrets and
credentials in URLs redacted.

## Signing in with single sign-on (OIDC)

With `SITREP_AUTH=oidc` (the default), admins sign in at an OpenID
Connect identity provider. Register SitRep there as a client with the
authorization code flow and the redirect URL
`https://<base domain>/auth/oidc/callback`. A confidential client needs
`SITREP_OIDC_CLIENT_SECRET`; a public client works without one, since
SitRep always uses PKCE.

Only members of `SITREP_OIDC_ADMIN_GROUP` may sign in. SitRep reads
the groups from the claim `SITREP_OIDC_GROUPS_CLAIM` of the ID token, or
from the user info endpoint if the ID token lacks the claim. Admins are
shown by their `name` claim, else `preferred_username`, else their subject.

- **Group changes** are checked only at sign-in. Someone removed from the
  admin group keeps access until their session expires, at most
  `SITREP_SESSION_TTL` after they signed in.
- **Signing out** ends the SitRep session, but not the session at the
  identity provider, which may sign the admin in again without asking.
- **Startup** does not wait for the identity provider. SitRep fetches
  its configuration in the background and retries at growing intervals of
  up to a minute; until then, the login screen says that sign-in is
  temporarily unavailable.
- **Several base domains:** sign-in always completes on the host of
  `SITREP_OIDC_REDIRECT_URL`, and the console continues there.

Notes for common identity providers:

- **Keycloak:** the issuer is `https://<host>/realms/<realm>`. Keycloak
  sends no groups by default: add a "Group Membership" mapper to the
  client with the token claim name `groups`, included in the ID token.
  With "Full group path" on, groups read `/admins`; turn it off or set
  `SITREP_OIDC_ADMIN_GROUP=/admins`.
- **authentik:** the issuer is
  `https://<host>/application/o/<application slug>/`, with the trailing
  slash. The default `profile` scope mapping sends the group names in
  `groups`.
- **Microsoft Entra ID:** the issuer is
  `https://login.microsoftonline.com/<tenant ID>/v2.0`. Use app roles
  rather than groups: define a role with the value `sitrep-admin` in the
  app registration, assign it to the admins, and set
  `SITREP_OIDC_GROUPS_CLAIM=roles` and
  `SITREP_OIDC_ADMIN_GROUP=sitrep-admin`. A groups claim would carry
  group object IDs, and Entra ID leaves it out entirely for users in more
  than 200 groups, which SitRep then treats as not being a member.

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
secret that changes daily; the change resets the address counters. It
keeps at most 10,000 counters in memory; while all are in use, attempts
that would need a new one are refused, too.

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
Requests to data sources and to the identity provider honor the proxy
environment variables `HTTPS_PROXY`, `HTTP_PROXY` and `NO_PROXY`.

Each panel is polled at once and then at its refresh interval, by default
`SITREP_DEFAULT_REFRESH`. Panels of a status page are polled whether or
not anyone visits it.

## Running behind a reverse proxy

SitRep speaks plain HTTP and neither terminates TLS nor compresses
responses; a reverse proxy in front of it does both. Set
`SITREP_TRUST_PROXY=true` so that SitRep takes the scheme, host and
client address from the proxy's `X-Forwarded-*` headers: it needs the
scheme to set secure cookies and to accept the console's requests. Expose
SitRep only to the proxy then, since anyone else could forge the
headers.

With [Caddy](https://caddyserver.com/), on-demand TLS gets a certificate
for each host the first time it is visited. Before it requests one, Caddy
asks `/tls/authorize?domain=<host>`, which answers 200 only for the base
domains and the hosts of existing status pages, whatever their
availability:

```caddyfile
{
	on_demand_tls {
		ask http://127.0.0.1:2607/tls/authorize
	}
}

https:// {
	tls {
		on_demand
	}
	encode zstd gzip
	reverse_proxy 127.0.0.1:2607
}
```

Caddy passes the original `Host` header and sets the `X-Forwarded-*`
headers by default. SitRep sends no `Strict-Transport-Security` header;
add one in the proxy if you want it. `/healthz` answers `ok` while the
database is readable, for health checks.

### DNS

- **Base domains:** point them at the proxy with A and AAAA records.
- **Subdomain mode:** add a wildcard record, e.g. `*.status.example.com`,
  pointing at the proxy. Every subdomain-mode page answers below every base
  domain.
- **Custom domains:** whoever owns the domain points it at the proxy,
  usually with a CNAME record to a base domain, e.g.
  `status.customer.example CNAME status.example.com`. SitRep does not
  check who owns a domain; entering it in the status page's settings is
  enough.

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

## Incidents

Admins open an incident with a title and a first update that sets the
status to planned (maintenance) or active, and add updates as things
change. Each update sets a status, a severity (minor, major or critical) or
both, and has a Markdown description. Ongoing incidents count in the status
of the page: a critical one as an outage, any other as degraded.

Visitors see upcoming and ongoing incidents, and finished ones for seven
days after their last update; the archive at `/incidents` lists all of
them. Charts shade the time an incident was ongoing in its severity's
color. Each language has an Atom feed at `/feed.atom` with one entry per
update.

A status page can delete finished incidents a number of days after their
last update ("Keep finished incidents" in its settings). Upcoming and
ongoing incidents are never deleted. Retention runs at startup and hourly.

### incidents.json

`/incidents.json` (with the language prefix on pages with several
languages, e.g. `/de/incidents.json`) lists the incidents visitors see, in
the URL's language: ID, title, phase (`upcoming`, `ongoing`, `finished`),
current status and severity, and every update with its time in UTC, status,
severity and description as HTML. Responses are cached for 60 seconds. It
is not linked from any page.

Other websites can read it in the browser when their origin is listed under
"Allowed origins for incidents.json" in the status page's settings, e.g.
`https://www.example.com`. SitRep then answers with
`Access-Control-Allow-Origin` for that origin; it never allows credentials.

## Legal pages and the landing page

The global settings set an imprint and a privacy statement, each either
none, a link to a page elsewhere, or a Markdown text that SitRep shows at
`/imprint` and `/privacy` (in German `/impressum` and `/datenschutz`). Each
status page uses them unless it sets its own. The footer links them.

The landing page on the base domains shows the landing text from the
global settings, or without one a neutral page with a link to the console.

## Branding

A status page can have a brand color, which colors its header and footer
with black or white text, whichever contrasts more, and an SVG logo of at
most 64 KiB. SitRep sanitizes logos when they are saved: it keeps only
plain SVG shapes, text, gradients, masks and filters, and removes scripts,
event handlers, links to other documents and anything that loads other
resources. Logos are served with a content security policy that sandboxes
them.

## Availability

A status page is online, offline or paused. Visitors of an offline or
paused page see its name, logo and legal pages and a notice that it is
unavailable; everything else answers 503, including the feed and
incidents.json. SitRep keeps polling offline pages, so their data is
current when they come back, but stops polling paused ones. The console
previews every page regardless of its availability.

## Import and export

The settings of a status page export it as a YAML file: its settings and
panels in display order, with localized texts by language. Panels name
their data source, so a file can be imported on another instance with data
sources of the same names and types. Exports leave out IDs, timestamps,
availability, incidents and the data sources themselves.

Importing a file creates a new status page ("Import status page" in the
sidebar), or replaces the settings and panels of an existing one, keeping
its incidents and availability. Files must start with `version: 1`, may
not contain unknown keys and are validated completely before anything is
saved.

## Cookies

None of these cookies is needed to read public pages, and none tracks
anyone:

| Cookie | Set when | Content | Lifetime |
|---|---|---|---|
| `theme` | a visitor picks a color scheme | `light`, `dark` or `system` | 1 year |
| `lang` | a visitor picks a language | the language code | 1 year |
| `sitrep_session`, on HTTPS `__Host-sitrep_session` | an admin signs in | a random session token | the session lifetime |
| `sitrep_oidc` | an admin starts single sign-on | random values that secure the sign-in, and the console page to return to | 10 minutes, deleted when the sign-in completes |

## Stored personal data

SitRep stores the subject, display name and, if the identity provider
sends one, the email address of signed-in admins in their session.
Signing out deletes the session. Sessions expire after
`SITREP_SESSION_TTL`, and expired ones are deleted at startup and
hourly.

Incidents and their updates store the subject and display name of the
admin who created them and of the admin who last edited each update. Only
the console shows them; status pages, feeds and incidents.json never do.
They are deleted with their incident, either by an admin or by the status
page's incident retention.

Logs contain the usernames of failed sign-ins and the subjects of sign-ins
refused by single sign-on, but no email addresses and no client
addresses.

## Development and testing

`make dev` runs the server with live reload of the backend and the
frontend, `make lint` runs every linter, and `make test` runs the Go,
Vitest and Playwright tests. `make help` lists all targets.
[CONTRIBUTING.md](CONTRIBUTING.md) explains the setup, the conventions and
how to add a language, an auth provider or a data source type.

## License

MIT, see [LICENSE](LICENSE).

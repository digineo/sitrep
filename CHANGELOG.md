# Changelog

All notable changes to this project are documented in this file. The format
is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- `sitrep serve`, configured by environment variables and dotenv files.
- `sitrep hash-password` for argon2id hashes in the users file.
- Embedded BBolt database with schema version and file lock.
- German and English catalogs; adding a language means adding a file.
- Host and path routing for the landing page, the admin console and status
  pages, with canonical, language-prefixed URLs and redirects.
- Admin sessions with CSRF protection, sign-in by username and password,
  and login throttling per username and per client address.
- Admin console frame: login screen, sidebar, theme and language switchers.
- Instance settings for languages and the default color scheme.
- Data sources with pluggable types, secrets encrypted at rest and bound to
  the data source's URL, connection tests and a console form generated from
  each type's fields.
- The Prometheus data source type with a read-only discovery proxy, and a
  PromQL editor with autocompletion in the console.
- Status pages with time zone and color scheme, created and edited in the
  console.
- Stat, status and chart panels, polled on schedules and cached in memory,
  with previews in the panel editor.
- Public status pages with a status banner, panels and charts, refreshed
  incrementally; the console's preview orders panels by drag and drop or
  keyboard.
- Incidents with a timeline of Markdown updates, managed in the console and
  shown on status pages, shaded in charts and listed in an archive.
- An Atom feed per language and incidents.json, readable cross-origin by
  each status page's allowed origins.
- Retention that deletes finished incidents after a status page's number of
  days.
- Imprint and privacy statement for the instance and per status page, as
  Markdown text or link, with localized URLs, and a landing page text.
- Brand colors and sanitized SVG logos for status pages, and a favicon in
  the color of the status.
- Offline and paused status pages; paused pages are not polled.
- Export and import of status pages as YAML files.
- Sign-in by OpenID Connect, limited to the members of an admin group, with
  localized notices for failed sign-ins on the login screen.
- `/tls/authorize` for reverse proxies with on-demand TLS, which confirms
  the base domains and the hosts of existing status pages.
- A status page can be promoted to start page of the base domains; its own
  route redirects there.
- `sitrep healthcheck`, which exits with 0 if the server's `/healthz`
  answers 200.
- A Dockerfile for a distroless image with a health check, published for
  amd64 and arm64 to `ghcr.io/digineo/sitrep` after CI passes: as
  `latest-dev` from the main branch, as `latest` and the tag from release
  tags.
- `sitrep version` prints the release tag, commit and build date, which
  `make build` and the image embed. `sitrep serve` logs them on startup,
  the console shows them below the signed-in admin.

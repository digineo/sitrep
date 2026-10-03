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
- Data sources with pluggable types, secrets encrypted at rest, connection
  tests and a console form generated from each type's fields.
- The Prometheus data source type with a read-only discovery proxy, and a
  PromQL editor with autocompletion in the console.
- Status pages with time zone and color scheme, created and edited in the
  console.
- Stat, status and chart panels, polled on schedules and cached in memory,
  with previews in the panel editor.
- Public status pages with a status banner, panels and charts, refreshed
  incrementally; the console's preview orders panels by drag and drop or
  keyboard.

# feedr handoff

Last updated: 2026-08-20

## Current state

`feedr` is a TypeScript library for collecting normalized feed items and sending
them to one or more publishing destinations. It is intentionally stateless:
there is no deduplication, database, email, logger, or notification integration.
Every item a `Feed` collector returns is published to each configured
`Publisher` on every run.

The working tree was clean when this handoff was written.

## Architecture

- `src/types.ts` defines the public `Feed`, `FeedItem`, `Publisher`, and run
  result contracts.
- `src/runner.ts` collects a feed's items and publishes them to every declared
  destination. It prevents concurrent runs of the same feed within one process.
- `src/scheduler.ts` provides the optional in-process cron scheduler.
- `src/publishers/bluesky.ts` is the current platform adapter. It supports
  login with an app password, links/facets, up to four images, and reply chains
  from `FeedItem.thread`.
- `src/index.ts` exports the public API. `feedr/bluesky` remains a supported
  package subpath and resolves to `dist/publishers/bluesky.js`.

## Package/tooling

- Node.js: `>=20`
- TypeScript: `^7.0.2` (installed: `7.0.2`)
- `@atproto/api`: `^0.20.41` (installed: `0.20.41`)
- `cron`: `^4.3.3`
- Prettier: `^3.9.6`, configured at 80 columns in `.prettierrc.json`

Useful commands:

```sh
npm run format        # Apply Prettier
npm run format:check  # Verify formatting
npm run build         # Compile to dist/
npm test              # Run the core publisher test
npm run check         # Formatting + build + tests
npm pack --dry-run    # Inspect the package contents
```

`package.json` has a strict `files` allowlist. This prevents stale local build
files from shipping; the published tarball includes only current entrypoints
and the Bluesky publisher under `dist/publishers/`.

## Validation completed

After the TypeScript and AT Protocol API upgrades, `npm run check` passed:

- Prettier check passed.
- TypeScript 7.0.2 compiled successfully.
- The test confirming every item reaches every destination passed.

## Open decision

The Bluesky publisher uses `AtpAgent`, `login`, `post`, `uploadBlob`, and
`RichText`; all compile against `@atproto/api@0.20.41`, so no migration is
needed for compatibility.

The current AT Protocol documentation recommends `Agent` plus
`CredentialSession` for new app-password integrations, and `@atproto/lex` for
new projects. Migrating to those APIs would be an optional intentional refactor,
not an urgent fix. Do not make that change without deciding whether feedr should
continue to offer the simple app-password constructor API.

## Intentional omissions

- No persistence or deduplication (`JsonFilePublicationStore` and
  `InMemoryPublicationStore` were removed).
- No SendGrid/database integration.
- No built-in observability abstraction; callers can instrument `FeedRunner` or
  their own schedulers however they choose.
- No X publisher yet. Add it by implementing the public `Publisher` interface.

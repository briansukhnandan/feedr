# feedr

`feedr` is a local, file-driven publishing daemon. Run it in Docker, let any
language generate JSON posts, and feedr sends them to configured publishers.
The first publisher is Bluesky; X is intentionally reserved for a future
adapter.

There is no application SDK in this design. A Python script, Node script, shell
script, or CI job only needs to write a JSON file. Publisher credentials remain
with the daemon, never with content-generating scripts.

## Quick start

Create the data directory and install configuration:

```sh
mkdir -p "$HOME/.feedr/$(date +%Y_%m_%d)"
cp examples/config.json "$HOME/.feedr/config.json"
cp examples/posts.json "$HOME/.feedr/$(date +%Y_%m_%d)/posts.json"
export FEEDR_BLUESKY_IDENTIFIER='you.bsky.social'
export FEEDR_BLUESKY_APP_PASSWORD='xxxx-xxxx-xxxx-xxxx'
export FEEDR_UID="$(id -u)"
export FEEDR_GID="$(id -g)"
docker compose up --build -d
```

The bundled Compose file mounts `$HOME/.feedr` at `/feedr` in the container.
The container sets `FEEDR_HOME=/feedr`, so it observes the host directory while
the native binary uses `$HOME/.feedr` by default. Set `FEEDR_DATA_DIR` before
starting Compose to mount a different host directory.

Use a [Bluesky app password](https://bsky.app/settings/app-passwords), not your
normal account password. The example references environment variables so
credentials are not saved in `config.json`.

## Directory contract

```
~/.feedr/
├── config.json
├── 2026_08_29/
│   └── posts.json
└── .state/
    └── deliveries.json       # maintained by feedr
```

On startup and every `pollIntervalSeconds` (60 seconds by default), feedr
looks only for today's `YYYY_MM_DD/posts.json` in the configured IANA timezone
(UTC by default). It safely ignores a day with no file. It reloads
configuration and the posts file on every pass, so generators and configuration
can update them without restarting the daemon.

The delivery journal is keyed by date, stable post ID, and publisher ID. Once a
delivery is recorded it is not sent again by ordinary polling passes. Preserve
stable IDs when regenerating a file. This is **at-least-once** delivery: an
abrupt process or filesystem failure after the remote API accepts a post but
before the journal is saved can cause a duplicate after the daemon restarts.

Do not edit `.state/deliveries.json` except to intentionally force a retry.

## Configuration

`~/.feedr/config.json` selects publishers. Every post goes to every listed
publisher.

```json
{
  "pollIntervalSeconds": 60,
  "timezone": "America/New_York",
  "publishers": [
    {
      "id": "bluesky:main",
      "type": "bluesky",
      "identifierEnv": "FEEDR_BLUESKY_IDENTIFIER",
      "appPasswordEnv": "FEEDR_BLUESKY_APP_PASSWORD"
    }
  ]
}
```

`identifier` and `appPassword` can also be set directly, but environment
variables are the recommended credential mechanism. A Bluesky publisher also
accepts an optional `service` URL for a compatible PDS. Publisher IDs must be
unique and are part of delivery de-duplication, so do not change them casually.

## `posts.json`

`posts.json` is a JSON array. Each object retains the former feedr `FeedItem`
shape: `id`, `text`, optional `url`, `publishedAt`, `media`, `thread`, and
`metadata`. The complete example is in [examples/posts.json](examples/posts.json).

```json
[
  {
    "id": "city-council-123",
    "text": "The city council approved the transit plan.",
    "url": "https://example.com/council/123",
    "thread": [
      { "text": "The city council approved the transit plan." },
      { "text": "Funding begins in the next budget cycle." }
    ]
  }
]
```

Posts require a unique, stable `id` within their daily file. A thread produces
a Bluesky reply chain; when `thread` is present it replaces the root `text`,
`url`, and `media`, matching the original publisher behavior. Media is fetched
from its `url`; each item needs `url`, `mimeType`, and an accessible `alt`
description. Up to four images are attached to a Bluesky post.

`text` and the optional URL are rendered to Bluesky’s 300-Unicode-character
limit. The daemon adds a rich-link facet for its appended URL.

## Generate posts from any runtime

Your job only needs to produce a complete file. Write to a temporary filename
in the same directory and rename it into place so the daemon never reads a
partial document:

```sh
#!/usr/bin/env sh
set -eu

day="$(date +%Y_%m_%d)"
target="$HOME/.feedr/$day"
mkdir -p "$target"
your-generator-command >"$target/posts.json.tmp"
mv "$target/posts.json.tmp" "$target/posts.json"
```

Schedule that script using whichever scheduler fits its environment (cron, a
CI schedule, systemd, Kubernetes, etc.). `feedr` owns publishing rather than
content collection, so it does not need to know the generator’s language.

## Development

The daemon is a standard-library-only Go module. Build and test locally with:

```sh
go test ./...
go build ./cmd/feedr
```

Or run it in the container:

```sh
docker build -t feedr:local .
docker run --rm \
  -e FEEDR_BLUESKY_IDENTIFIER \
  -e FEEDR_BLUESKY_APP_PASSWORD \
  -v "$HOME/.feedr:/feedr" \
  feedr:local
```

## Child nodes: proposed next step

The implementation intentionally stops at the shared file boundary. The
recommended follow-on is a `feedr-node` runner that schedules a user command,
validates its JSON, and atomically publishes it to a shared volume. It should
not use a port-per-node or give generators publisher credentials. The detailed
proposal, including the remaining merge and ownership decisions, is in
[docs/child-nodes.md](docs/child-nodes.md).

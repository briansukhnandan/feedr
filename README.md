# feedr

`feedr` is a local, file-driven publishing daemon. Run it in Docker, let any
language generate JSON posts, and feedr sends them to configured publishers.
The first publisher is Bluesky; X is intentionally reserved for a future
adapter.

There is no application SDK in this design. A Python script, Node script, shell
script, or CI job only needs to write a JSON file. Publisher credentials remain
with the daemon, never with content-generating scripts.

## Quick start

Initialize the local feedr directory and deploy the daemon:

```sh
make start
```

To stop and remove the running container while preserving your feed data:

```sh
make stop
```

To open a shell in the running container:

```sh
make shell
```

To export the daemon logs to a timestamped file in `/tmp`:

```sh
make export
```

On its first run, the script creates `~/.feedr/`, `config.json`, today's
example feed file, and an empty, mode-0600 `~/.feedr/.env`. It never
replaces existing configuration or generated content. Add your account values
to `.env`, then rerun the script to deploy with them:

```sh
FEEDR_BLUESKY_IDENTIFIER=you.bsky.social
FEEDR_BLUESKY_APP_PASSWORD=xxxx-xxxx-xxxx-xxxx
```

The script derives the host UID/GID, stops the existing `feedr` container, then
builds and force-recreates it. Run it again after changing this repository or
your deployment settings.

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
│   └── feeds/
│       └── example/posts.json
├── .env                         # account credentials; mode 0600
└── state/
    └── deliveries.db         # maintained by feedr
```

On startup and every `pollIntervalSeconds` (60 seconds by default), feedr
looks for each configured feed at
`YYYY_MM_DD/feeds/<feed-id>/posts.json`. The date uses the configured IANA
timezone (UTC by default). It safely ignores missing files and reloads
configuration and posts on every pass, so generators can update them without
restarting the daemon.

The delivery database is keyed by date, feed ID, stable post ID, and destination
account. Once a delivery is recorded it is not sent again by ordinary polling
passes. Preserve stable IDs when regenerating a file. This is **at-least-once**
delivery: an abrupt process or filesystem failure after the remote API accepts
a post but before the database write can cause a duplicate after the daemon
restarts.

Do not edit `state/deliveries.db` except to intentionally force a retry.

## Configuration

`~/.feedr/config.json` has three separate concerns: publisher implementations,
their accounts, and feed-to-account routing. A feed does not carry credentials
or choose an account; the parent owns that mapping.

```json
{
  "pollIntervalSeconds": 60,
  "timezone": "America/New_York",
  "publishers": [
    {
      "id": "bluesky",
      "type": "bluesky",
      "accounts": [
        {
          "id": "main",
          "identifierEnv": "FEEDR_BLUESKY_IDENTIFIER",
          "appPasswordEnv": "FEEDR_BLUESKY_APP_PASSWORD"
        }
      ]
    }
  ],
  "feeds": [
    {
      "id": "example",
      "destinations": [
        { "publisher": "bluesky", "account": "main" }
      ]
    }
  ]
}
```

For that configuration, feedr sends
`YYYY_MM_DD/feeds/example/posts.json` to the configured account. Add a feed by
adding its `id` and destinations to this list, then have its generator write to
the matching directory. A feed can list multiple destinations when intentional
cross-posting is needed. Directories without a configured feed ID are ignored.

Account values can be supplied directly as `identifier` and `appPassword`, but
environment variables are recommended. Compose reads arbitrary account
variables from `~/.feedr/.env`; for the configuration above it
contains:

```sh
FEEDR_BLUESKY_IDENTIFIER=example.bsky.social
FEEDR_BLUESKY_APP_PASSWORD=xxxx-xxxx-xxxx-xxxx
```

The quick-start creates an empty, mode-0600 `.env` file once. Add
account-specific values there. A Bluesky account also accepts an optional
`service` URL for a compatible PDS.

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
target="$HOME/.feedr/$day/feeds/example"
mkdir -p "$target"
your-generator-command >"$target/posts.json.tmp"
mv "$target/posts.json.tmp" "$target/posts.json"
```

Replace `example` with a feed ID from `config.json`. The generator does not
register feeds or select destinations; the parent configuration owns both.

Schedule that script using whichever scheduler fits its environment (cron, a
CI schedule, systemd, Kubernetes, etc.). `feedr` owns publishing rather than
content collection, so it does not need to know the generator’s language.

## Development

The only requirement to run this is `docker`
Run the test suite with:

```sh
make test
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

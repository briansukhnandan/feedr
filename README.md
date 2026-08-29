# feedr

`feedr` is a local, file-driven publishing daemon. Run it in Docker, let any
language generate JSON posts, and feedr sends them to configured publishers.
The first publisher is Bluesky; X is intentionally reserved for a future
adapter.

There is no application SDK in this design. A Python script, Node script, shell
script, or CI job only needs to write a JSON file. Publisher credentials remain
with the daemon, never with content-generating scripts.

## Quick start

Set the two credentials and run the deployment script:

```sh
export FEEDR_BLUESKY_IDENTIFIER='you.bsky.social'
export FEEDR_BLUESKY_APP_PASSWORD='xxxx-xxxx-xxxx-xxxx'
./scripts/quickstart.sh
```

The script creates `config.json` and today's example `posts.json` only when
they do not already exist; it never replaces your configuration or generated
content. It derives the host UID/GID, stops the existing `feedr` container,
then builds and force-recreates it. Run it again after changing this repository
or your deployment settings.

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
│   ├── posts.json
│   └── nodes/
│       ├── reddit/posts.json
│       └── congress/posts.json
├── credentials.env             # account credentials; mode 0600
└── .state/
    └── deliveries.json       # maintained by feedr
```

On startup and every `pollIntervalSeconds` (60 seconds by default), feedr
looks for today's named-node files and, when `defaultFeed` is configured, the
top-level `YYYY_MM_DD/posts.json`. The date uses the configured IANA timezone
(UTC by default). It safely ignores missing files and reloads configuration and
posts on every pass, so generators can update them without restarting the
daemon.

The delivery journal is keyed by date, feed ID, stable post ID, and destination
account. Once a delivery is recorded it is not sent again by ordinary polling
passes. Preserve stable IDs when regenerating a file. This is **at-least-once**
delivery: an abrupt process or filesystem failure after the remote API accepts
a post but before the journal is saved can cause a duplicate after the daemon
restarts.

Do not edit `.state/deliveries.json` except to intentionally force a retry.

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
          "id": "brinet-reddit",
          "identifierEnv": "FEEDR_REDDIT_BLUESKY_IDENTIFIER",
          "appPasswordEnv": "FEEDR_REDDIT_BLUESKY_APP_PASSWORD"
        },
        {
          "id": "brinet-congress",
          "identifierEnv": "FEEDR_CONGRESS_BLUESKY_IDENTIFIER",
          "appPasswordEnv": "FEEDR_CONGRESS_BLUESKY_APP_PASSWORD"
        }
      ]
    }
  ],
  "feeds": [
    {
      "id": "reddit",
      "destinations": [
        { "publisher": "bluesky", "account": "brinet-reddit" }
      ]
    },
    {
      "id": "congress",
      "destinations": [
        { "publisher": "bluesky", "account": "brinet-congress" }
      ]
    }
  ]
}
```

For that configuration, the two nodes write to
`YYYY_MM_DD/nodes/reddit/posts.json` and
`YYYY_MM_DD/nodes/congress/posts.json`, respectively. The parent sends each
file only to its configured account. A feed can list multiple destinations when
intentional cross-posting is needed.

The quick-start's one-account configuration uses `defaultFeed` to route the
legacy top-level `YYYY_MM_DD/posts.json`. `defaultFeed` is optional; omit it
when every producer is a named node.

Account values can be supplied directly as `identifier` and `appPassword`, but
environment variables are recommended. Compose reads arbitrary account
variables from `~/.feedr/credentials.env`; for the configuration above it
contains:

```sh
FEEDR_REDDIT_BLUESKY_IDENTIFIER=reddit.example
FEEDR_REDDIT_BLUESKY_APP_PASSWORD=xxxx-xxxx-xxxx-xxxx
FEEDR_CONGRESS_BLUESKY_IDENTIFIER=congress.example
FEEDR_CONGRESS_BLUESKY_APP_PASSWORD=xxxx-xxxx-xxxx-xxxx
```

The quick-start creates a 0600 credentials file for its single sample account
once. Add account-specific values yourself, or set `FEEDR_CREDENTIALS_FILE` to
an existing protected env file before running the script. A Bluesky account
also accepts an optional `service` URL for a compatible PDS.

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
target="$HOME/.feedr/$day/nodes/reddit"
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

## Child nodes

Named child-node directories and parent-side routing are implemented. A future
`feedr-node` runner can add portable command scheduling and JSON validation;
the execution model and boundary rationale are in
[docs/child-nodes.md](docs/child-nodes.md).

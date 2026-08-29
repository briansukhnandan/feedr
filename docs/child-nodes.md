# Child-node design proposal

The daemon implemented today is deliberately file-driven. A producer of any
language writes a complete `posts.json` into its own workspace and a shared
volume exposes it to the parent daemon. This is the compatibility boundary;
producers do not import a feedr library or call a language-specific adapter.

The next layer should be a separate `feedr-node` runner, rather than a port per
producer or a long-running parent-to-child connection.

```
user command + cron expression
             |
        feedr-node container
             |
 atomic write to a named shared volume
             |
          feedr daemon
             |
       publisher adapters
```

## Proposed node manifest

Keep this outside `posts.json`, for example in a repository-local
`feedr.node.json`:

```json
{
  "id": "city-alerts",
  "schedule": "15 * * * *",
  "timezone": "America/New_York",
  "command": ["./generate-posts.sh"],
  "output": "./.feedr/posts.json"
}
```

`feedr-node` executes the command on its schedule, validates the output against
the public post schema, then atomically writes it to the parent's current-date
directory. It should write to a temporary filename and rename only after the
JSON is complete. The parent sees either an old valid file or a new valid file,
never partial JSON.

## Why not ports or parent-controlled cron

Ports add discovery, authentication, lifecycle, and networking concerns even
though the only data exchanged is a small JSON document. A shared volume gives
local, durable, inspectable handoff with no service discovery. Putting the
scheduler in the node also lets the user's runtime be fully isolated: a Python
node needs Python, a Node node needs Node, and the parent needs neither.

The parent should remain the owner of publisher credentials and the delivery
journal. Nodes get write access only to their own staging area; they should not
receive Bluesky credentials or modify publisher state.

## Decisions to make before building nodes

1. **Merge rule.** Several nodes cannot safely overwrite one shared
   `posts.json`. Give each node a namespace, such as
   `YYYY_MM_DD/nodes/<node-id>/posts.json`, and have the parent merge stable
   post IDs.
2. **Freshness rule.** Decide whether a new node output replaces the whole
   node's desired set or only appends posts. Replacing the set is simpler when
   combined with stable IDs and the existing delivery journal.
3. **Failure reporting.** Start with structured container logs and non-zero
   node exits. Add webhooks or notifications only after the delivery contract
   is settled.
4. **Execution model.** Prefer a one-shot `feedr-node run` plus the host or
   container scheduler initially. A long-running node daemon can follow if a
   single deployment command is important.

# feedr

`feedr` is a TypeScript toolkit for moving normalized items from a source to
one or more publishing destinations. A source collector stays in your
application; feedr runs it, sends the resulting items to publishers, and can
schedule that work with cron.

The package does not prescribe how you fetch data, persist state, deduplicate
items, or observe runs. Those choices remain with the host application.

## Install

```sh
npm install feedr
```

## Core concepts

- A `Feed` collects an array of portable `FeedItem` objects.
- A `Publisher` knows how to deliver those items to one destination account.
- `FeedRunner` sends every collected item to every configured publisher.
- `FeedScheduler` is an optional in-process cron scheduler; call
  `runner.run(feed)` directly when another system owns scheduling.

## Example: scheduled Bluesky publishing

```ts
import { BlueskyPublisher, FeedRunner, FeedScheduler, type Feed } from "feedr";

const updates: Feed = {
  id: "product-updates",
  schedule: { expression: "0 * * * *", timezone: "America/New_York" },
  destinations: ["bluesky:product-updates"],
  async collect() {
    const items = await getUpdatesFromYourSource();

    return items.map((item) => ({
      id: item.id,
      text: item.summary,
      url: item.url,
      publishedAt: item.publishedAt,
      media: item.imageUrl
        ? [
            {
              alt: item.imageAlt,
              mimeType: "image/jpeg",
              url: item.imageUrl,
            },
          ]
        : undefined,
    }));
  },
};

const runner = new FeedRunner({
  publishers: [
    new BlueskyPublisher({
      id: "bluesky:product-updates",
      credentials: {
        identifier: process.env.BLUESKY_IDENTIFIER!,
        password: process.env.BLUESKY_APP_PASSWORD!,
      },
    }),
  ],
});

new FeedScheduler(runner, [{ feed: updates }]).start();
```

## Feed items

Every collector returns `FeedItem` objects:

```ts
{
  id: "source-item-id",
  text: "The primary post text",
  url: "https://source.example/items/123",
  media: [{ alt: "Accessible image description", mimeType: "image/jpeg", data }],
  thread: [
    { text: "The root post" },
    { text: "A reply to the root post" },
  ],
  metadata: { source: "my-source" },
}
```

`thread` is optional. When present, the Bluesky publisher creates a reply
chain. Otherwise it publishes `text`, `url`, and `media` as a single post.
The Bluesky publisher detects link facets, uploads up to four images, truncates
to its configured post limit, and logs in once per process.

## Add another platform

Implement `Publisher` to add a destination. It receives the same
`FeedItem` contract and returns the platform receipt:

```ts
import type { Publisher } from "feedr";

class XPublisher implements Publisher {
  id = "x:my-account";

  async publish(item) {
    const response = await xClient.post(item.text);
    return { id: response.id, url: response.url };
  }
}
```

Each run publishes every item returned by the feed to each configured
destination.

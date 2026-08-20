# feedr

`feedr` is a small TypeScript package for collecting normalized feed items and publishing them on a schedule. It keeps source collectors (Reddit, Congress.gov, RSS, etc.) separate from publishing platforms (Bluesky now; X or others later).

It carries forward the useful BriNet patterns—cron execution, source/account-specific logging, post-after-success deduplication, and email-ready run notifications—without making Reddit, Congress.gov, SQLite, SendGrid, or Bluesky credentials part of the feed model.

## Install

```sh
npm install feedr
```

## Define a feed and a Bluesky account

```ts
import {
  BlueskyPublisher,
  FeedRunner,
  FeedScheduler,
  JsonFilePublicationStore,
  type Feed,
} from "feedr";

const worldNews: Feed = {
  id: "reddit-worldnews",
  schedule: { expression: "0 6 * * *", timezone: "America/New_York" },
  destinations: ["bluesky:worldnews-tracker"],
  async collect() {
    const posts = await getRedditPosts(); // your application's collector
    return posts.map((post) => ({
      id: post.id,                    // stable source ID: required for deduplication
      text: `Posted on ${post.date}\n\n${post.title}`,
      url: `https://reddit.com${post.permalink}`,
      publishedAt: post.createdAt,
      media: post.image
        ? [{ alt: post.title, mimeType: "image/jpeg", url: post.image }]
        : undefined,
    }));
  },
};

const runner = new FeedRunner({
  publishers: [
    new BlueskyPublisher({
      id: "bluesky:worldnews-tracker",
      credentials: {
        identifier: process.env.BLUESKY_IDENTIFIER!,
        password: process.env.BLUESKY_APP_PASSWORD!,
      },
    }),
  ],
  // Use your own SQL/Redis store in a multi-instance deployment.
  store: new JsonFilePublicationStore("./data/feedr-publications.json"),
  notifier: {
    // Adapt SendGrid, Resend, SES, etc. in the host app.
    async notify({ type, result }) {
      if (type === "run-failed") await sendOpsEmail(result);
    },
  },
});

new FeedScheduler(runner, [{ feed: worldNews }]).start();
```

`FeedRunner#run(feed)` is also public for cron systems such as GitHub Actions, systemd timers, or serverless schedulers; `FeedScheduler` is only the in-process cron option.

## The normalized contract

Each source yields `FeedItem` objects:

```ts
{
  id: "source-stable-id",
  text: "Root post text",
  url: "https://canonical-source.example/item",
  media: [{ alt: "Accessible description", mimeType: "image/jpeg", data }],
  thread: [
    { text: "Root post text" },
    { text: "Second post, published as a reply" },
  ],
  metadata: { source: "congress.gov" },
}
```

`thread` is optional. When present it maps to a Bluesky reply chain; when absent feedr publishes `text`, `url`, and `media` as one post. The Bluesky adapter detects link facets, uploads up to four images, truncates to the configured post limit, and logs in once per process. A post is marked published only after its destination confirms success. Publication state is scoped by feed, item, and destination, so a failed X publisher later can retry without reposting to Bluesky.

## Extending platforms

Implement `Publisher` to add a platform. It receives the same `FeedItem` contract and returns a platform receipt:

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

For production deployments, provide a `PublicationStore` backed by your existing database and enforce a unique `(feedId, itemId, publisherId)` index. `JsonFilePublicationStore` is intentionally a single-process convenience, suitable for the BriNet-style Raspberry Pi setup.

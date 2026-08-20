import assert from "node:assert/strict";
import test from "node:test";
import { FeedRunner, type Feed, type Publisher } from "../src/index.js";

test("publishes every collected item to every destination", async () => {
  const published: string[] = [];
  const bluesky: Publisher = {
    id: "bluesky:test",
    async publish(item) {
      published.push(`bluesky:${item.id}`);
      return { id: `at://${item.id}` };
    },
  };
  const x: Publisher = {
    id: "x:test",
    async publish(item) {
      published.push(`x:${item.id}`);
      return { id: item.id };
    },
  };
  const feed: Feed = {
    id: "example",
    destinations: [bluesky.id, x.id],
    collect: () => [
      { id: "one", text: "First" },
      { id: "two", text: "Second" },
    ],
  };

  const result = await new FeedRunner({ publishers: [bluesky, x] }).run(feed);
  assert.equal(result.published, 4);
  assert.equal(result.failed, 0);
  assert.deepEqual(published, ["bluesky:one", "x:one", "bluesky:two", "x:two"]);
});

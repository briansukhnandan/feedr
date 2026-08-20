import assert from "node:assert/strict";
import { mkdtemp, rm } from "node:fs/promises";
import { join } from "node:path";
import { tmpdir } from "node:os";
import test from "node:test";
import { FeedRunner, InMemoryPublicationStore, JsonFilePublicationStore, type Feed, type Publisher } from "../src/index.js";

test("records each destination separately and skips it on the next run", async () => {
  const calls: string[] = [];
  const publisher: Publisher = {
    id: "bluesky:test",
    async publish(item) {
      calls.push(item.id);
      return { id: `at://${item.id}` };
    },
  };
  const feed: Feed = {
    id: "example",
    destinations: [publisher.id],
    collect: () => [{ id: "one", text: "First" }, { id: "two", text: "Second" }],
  };
  const runner = new FeedRunner({ publishers: [publisher], store: new InMemoryPublicationStore() });

  assert.equal((await runner.run(feed)).published, 2);
  const retry = await runner.run(feed);
  assert.equal(retry.published, 0);
  assert.equal(retry.skipped, 2);
  assert.deepEqual(calls, ["one", "two"]);
});

test("does not record a failed destination, allowing only it to retry", async () => {
  let shouldFail = true;
  const successful: Publisher = { id: "bluesky:ok", publish: async () => ({ id: "ok" }) };
  const flaky: Publisher = {
    id: "x:flaky",
    publish: async () => {
      if (shouldFail) throw new Error("network unavailable");
      return { id: "repaired" };
    },
  };
  const feed: Feed = {
    id: "multi-platform",
    destinations: [successful.id, flaky.id],
    collect: () => [{ id: "one", text: "First" }],
  };
  const runner = new FeedRunner({ publishers: [successful, flaky], store: new InMemoryPublicationStore() });

  assert.equal((await runner.run(feed)).failed, 1);
  shouldFail = false;
  const retry = await runner.run(feed);
  assert.equal(retry.skipped, 1);
  assert.equal(retry.published, 1);
});

test("persists publication state between store instances", async () => {
  const directory = await mkdtemp(join(tmpdir(), "feedr-test-"));
  const path = join(directory, "publications.json");
  try {
    const first = new JsonFilePublicationStore(path);
    await first.markPublished({
      feedId: "test-feed",
      itemId: "test-item",
      publisherId: "bluesky:test",
      receipt: { id: "at://test" },
      publishedAt: new Date(),
    });
    const second = new JsonFilePublicationStore(path);
    assert.equal(await second.wasPublished("test-feed", "test-item", "bluesky:test"), true);
  } finally {
    await rm(directory, { recursive: true, force: true });
  }
});

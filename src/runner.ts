import type { Feed, FeedRunError, FeedRunResult, Publisher } from "./types.js";

export interface FeedRunnerOptions {
  publishers: Iterable<Publisher>;
}

/** Runs source collectors through one or more platform publishers. */
export class FeedRunner {
  private readonly publishers: Map<string, Publisher>;
  private readonly activeFeeds = new Set<string>();

  constructor(options: FeedRunnerOptions) {
    this.publishers = new Map(
      [...options.publishers].map((publisher) => [publisher.id, publisher]),
    );
  }

  async run(feed: Feed, signal?: AbortSignal): Promise<FeedRunResult> {
    if (this.activeFeeds.has(feed.id)) {
      throw new Error(`Feed "${feed.id}" is already running.`);
    }
    this.activeFeeds.add(feed.id);
    const startedAt = new Date();
    const errors: FeedRunError[] = [];
    let collected = 0;
    let published = 0;

    try {
      this.assertFeed(feed);
      const items = await feed.collect({ signal, now: startedAt });
      collected = items.length;

      for (const item of items) {
        for (const publisherId of feed.destinations) {
          if (signal?.aborted)
            throw signal.reason ?? new Error("Feed run aborted.");
          const publisher = this.publishers.get(publisherId)!;

          try {
            await publisher.publish(item, { feed, signal });
            published += 1;
          } catch (error) {
            errors.push({ itemId: item.id, publisherId: publisher.id, error });
          }
        }
      }
    } catch (error) {
      errors.push({ error });
    } finally {
      this.activeFeeds.delete(feed.id);
    }

    const result: FeedRunResult = {
      feedId: feed.id,
      startedAt,
      finishedAt: new Date(),
      collected,
      published,
      failed: errors.filter((error) => error.itemId !== undefined).length,
      errors,
    };
    return result;
  }

  private assertFeed(feed: Feed): void {
    if (!feed.id) throw new Error("A feed requires a stable id.");
    if (!feed.destinations.length)
      throw new Error(`Feed "${feed.id}" has no destinations.`);

    for (const destination of feed.destinations) {
      if (!this.publishers.has(destination))
        throw new Error(
          `Feed "${feed.id}" references unknown publisher "${destination}".`,
        );
    }
  }
}

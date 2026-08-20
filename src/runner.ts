import { ConsoleLogger } from "./logging.js";
import type {
  Feed,
  FeedLogger,
  FeedNotifier,
  FeedRunError,
  FeedRunResult,
  PublicationStore,
  Publisher,
} from "./types.js";

export interface FeedRunnerOptions {
  publishers: Iterable<Publisher>;
  store: PublicationStore;
  logger?: FeedLogger;
  notifier?: FeedNotifier;
}

/**
 * Runs source collectors through one or more platform publishers. A successful
 * destination is recorded independently, so a retry only repairs failed targets.
 */
export class FeedRunner {
  private readonly publishers: Map<string, Publisher>;
  private readonly store: PublicationStore;
  private readonly logger: FeedLogger;
  private readonly notifier?: FeedNotifier;
  private readonly activeFeeds = new Set<string>();

  constructor(options: FeedRunnerOptions) {
    this.publishers = new Map([...options.publishers].map((publisher) => [publisher.id, publisher]));
    this.store = options.store;
    this.logger = options.logger ?? new ConsoleLogger();
    this.notifier = options.notifier;
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
    let skipped = 0;

    try {
      this.assertFeed(feed);
      await this.log({ level: "info", message: "Feed run started", feedId: feed.id });
      const items = await feed.collect({ signal, now: startedAt, logger: this.logger });
      collected = items.length;
      this.assertItems(feed, items);

      for (const item of items) {
        for (const publisherId of feed.destinations) {
          if (signal?.aborted) throw signal.reason ?? new Error("Feed run aborted.");
          const publisher = this.publishers.get(publisherId)!;
          if (await this.store.wasPublished(feed.id, item.id, publisher.id)) {
            skipped += 1;
            await this.log({ level: "debug", message: "Skipping already-published item", feedId: feed.id, publisherId: publisher.id, itemId: item.id });
            continue;
          }

          try {
            const receipt = await publisher.publish(item, { feed, signal, logger: this.logger });
            await this.store.markPublished({
              feedId: feed.id,
              itemId: item.id,
              publisherId: publisher.id,
              receipt,
              publishedAt: receipt.publishedAt ?? new Date(),
            });
            published += 1;
            await this.log({ level: "info", message: "Published item", feedId: feed.id, publisherId: publisher.id, itemId: item.id, data: { receiptId: receipt.id } });
          } catch (error) {
            errors.push({ itemId: item.id, publisherId: publisher.id, error });
            await this.log({ level: "error", message: "Failed to publish item", feedId: feed.id, publisherId: publisher.id, itemId: item.id, error });
          }
        }
      }
    } catch (error) {
      errors.push({ error });
      await this.log({ level: "error", message: "Feed run failed", feedId: feed.id, error });
    } finally {
      this.activeFeeds.delete(feed.id);
    }

    const result: FeedRunResult = {
      feedId: feed.id,
      startedAt,
      finishedAt: new Date(),
      collected,
      published,
      skipped,
      failed: errors.filter((error) => error.itemId !== undefined).length,
      errors,
    };
    await this.notifier?.notify({ type: errors.length ? "run-failed" : "run-completed", result });
    await this.log({ level: errors.length ? "warn" : "info", message: "Feed run finished", feedId: feed.id, data: { collected, published, skipped, failed: result.failed } });
    return result;
  }

  private assertFeed(feed: Feed): void {
    if (!feed.id) throw new Error("A feed requires a stable id.");
    if (!feed.destinations.length) throw new Error(`Feed "${feed.id}" has no destinations.`);
    for (const destination of feed.destinations) {
      if (!this.publishers.has(destination)) throw new Error(`Feed "${feed.id}" references unknown publisher "${destination}".`);
    }
  }

  private assertItems(feed: Feed, items: readonly { id: string; text: string }[]): void {
    const ids = new Set<string>();
    for (const item of items) {
      if (!item.id) throw new Error(`Feed "${feed.id}" returned an item without an id.`);
      if (ids.has(item.id)) throw new Error(`Feed "${feed.id}" returned duplicate item id "${item.id}".`);
      ids.add(item.id);
    }
  }

  private log(event: Omit<Parameters<FeedLogger["log"]>[0], "at">): Promise<void> {
    return Promise.resolve(this.logger.log({ ...event, at: new Date() }));
  }
}

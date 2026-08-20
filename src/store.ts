import type { PublicationStore, PublishedRecord } from "./types.js";
import { mkdir, readFile, rename, writeFile } from "node:fs/promises";
import { dirname } from "node:path";

const recordKey = (feedId: string, itemId: string, publisherId: string) =>
  `${feedId}\u0000${itemId}\u0000${publisherId}`;

/** Useful for tests, one-off scripts, and serverless invocations with external state. */
export class InMemoryPublicationStore implements PublicationStore {
  private readonly records = new Map<string, PublishedRecord>();

  async wasPublished(feedId: string, itemId: string, publisherId: string): Promise<boolean> {
    return this.records.has(recordKey(feedId, itemId, publisherId));
  }

  async markPublished(record: PublishedRecord): Promise<void> {
    this.records.set(recordKey(record.feedId, record.itemId, record.publisherId), record);
  }

  get(feedId: string, itemId: string, publisherId: string): PublishedRecord | undefined {
    return this.records.get(recordKey(feedId, itemId, publisherId));
  }
}

/**
 * A compact, durable default for a Raspberry Pi or single-process deployment.
 * For multi-instance deployments, implement `PublicationStore` with a database
 * unique constraint on `(feedId, itemId, publisherId)` instead.
 */
export class JsonFilePublicationStore implements PublicationStore {
  private records: Record<string, PublishedRecord> | undefined;
  private writeChain = Promise.resolve();

  constructor(private readonly path: string) {}

  async wasPublished(feedId: string, itemId: string, publisherId: string): Promise<boolean> {
    const records = await this.load();
    return recordKey(feedId, itemId, publisherId) in records;
  }

  async markPublished(record: PublishedRecord): Promise<void> {
    await this.queueWrite(async () => {
      const records = await this.load();
      records[recordKey(record.feedId, record.itemId, record.publisherId)] = record;
      await mkdir(dirname(this.path), { recursive: true });
      const temporaryPath = `${this.path}.tmp`;
      await writeFile(temporaryPath, JSON.stringify(records, null, 2), "utf8");
      await rename(temporaryPath, this.path);
    });
  }

  private async load(): Promise<Record<string, PublishedRecord>> {
    if (this.records) return this.records;
    try {
      const raw = await readFile(this.path, "utf8");
      this.records = JSON.parse(raw) as Record<string, PublishedRecord>;
    } catch (error: unknown) {
      if ((error as NodeJS.ErrnoException).code !== "ENOENT") throw error;
      this.records = {};
    }
    return this.records;
  }

  private queueWrite(work: () => Promise<void>): Promise<void> {
    const next = this.writeChain.then(work, work);
    this.writeChain = next.catch(() => undefined);
    return next;
  }
}

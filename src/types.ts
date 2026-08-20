/** A source-independent unit of content. */
export interface FeedItem {
  /** Source identifier, e.g. a Reddit post id or a Congress action id. */
  id: string;

  /** Main text for a one-post item. Ignored when `thread` is supplied. */
  text: string;

  /** Canonical source URL. Publishers can render this as a rich link. */
  url?: string;

  /** Original source timestamp, retained for formatters and audit logs. */
  publishedAt?: Date | string;

  /** Optional media accompanying the root post. */
  media?: FeedMedia[];

  /**
   * An ordered series of posts. The first segment is the root; later segments
   * are replies to the preceding segment. This maps to a Bluesky thread today
   * and leaves room for a native X thread later.
   */
  thread?: FeedPost[];

  /** Arbitrary source data for a consuming app's formatter or publisher. */
  metadata?: Record<string, unknown>;
}

export interface FeedPost {
  text: string;
  url?: string;
  media?: FeedMedia[];
}

export interface FeedMedia {
  alt: string;
  mimeType: string;
  /** Bytes already fetched by the feed. Preferred to make publication deterministic. */
  data?: Uint8Array;
  /** Optional URL for a publisher that supports remote-media fetching. */
  url?: string;
  width?: number;
  height?: number;
}

export interface FeedRunContext {
  signal?: AbortSignal;
  now: Date;
}

export interface Feed {
  /** Stable feed identity, useful to the host application's scheduling and observability. */
  id: string;
  name?: string;

  /** A cron expression used by `FeedScheduler`; no schedule is required for manual runs. */
  schedule?: CronSchedule;

  /** Publisher ids that should receive every collected item. */
  destinations: string[];

  /** Collect source data and return normalized, stable-id items. */
  collect(
    context: FeedRunContext,
  ): Promise<readonly FeedItem[]> | readonly FeedItem[];
}

export interface CronSchedule {
  expression: string;
  timezone?: string;
}

export interface Publisher {
  /** Stable destination identity, such as `bluesky:congress-tracker`. */
  id: string;
  publish(item: FeedItem, context: PublishContext): Promise<PublicationReceipt>;
}

export interface PublishContext {
  feed: Feed;
  signal?: AbortSignal;
}

export interface PublicationReceipt {
  /** Platform-native identifier or URI. */
  id: string;
  url?: string;
  publishedAt?: Date;
  metadata?: Record<string, unknown>;
}

export interface FeedRunResult {
  feedId: string;
  startedAt: Date;
  finishedAt: Date;
  collected: number;
  published: number;
  failed: number;
  errors: FeedRunError[];
}

export interface FeedRunError {
  itemId?: string;
  publisherId?: string;
  error: unknown;
}

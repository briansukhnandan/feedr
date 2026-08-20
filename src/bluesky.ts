import { AtpAgent, RichText } from "@atproto/api";
import type { AppBskyFeedPost } from "@atproto/api";
import type {
  FeedItem,
  FeedMedia,
  FeedPost,
  PublicationReceipt,
  Publisher,
  PublishContext,
} from "./types.js";

export interface BlueskyCredentials {
  identifier: string;
  password: string;
}

export interface BlueskyPublisherOptions {
  /** Publisher identity, e.g. `bluesky:worldnews-tracker`. */
  id: string;
  credentials: BlueskyCredentials;
  service?: string;
  /** Defaults to 300, Bluesky's current post limit. */
  maxGraphemes?: number;
}

type StrongRef = { uri: string; cid: string };

/** Bluesky adapter. Authentication belongs to the consuming app, never a feed source. */
export class BlueskyPublisher implements Publisher {
  readonly id: string;
  private readonly agent: AtpAgent;
  private readonly credentials: BlueskyCredentials;
  private readonly maxGraphemes: number;
  private prepared = false;

  constructor(options: BlueskyPublisherOptions) {
    this.id = options.id;
    this.credentials = options.credentials;
    this.maxGraphemes = options.maxGraphemes ?? 300;
    this.agent = new AtpAgent({
      service: options.service ?? "https://bsky.social",
    });
  }

  async publish(
    item: FeedItem,
    context: PublishContext,
  ): Promise<PublicationReceipt> {
    await this.prepare();
    const segments = item.thread?.length
      ? item.thread
      : [{ text: item.text, url: item.url, media: item.media }];
    let root: StrongRef | undefined;
    let parent: StrongRef | undefined;
    let last: StrongRef | undefined;

    for (const segment of segments) {
      const result = await this.postSegment(segment, root, parent, context);
      const ref = { uri: result.uri, cid: result.cid };
      root ??= ref;
      parent = ref;
      last = ref;
    }
    if (!last)
      throw new Error("A Bluesky item must contain at least one post.");
    return {
      id: last.uri,
      url: `https://bsky.app/profile/${this.agent.session?.did}/post/${last.uri.split("/").at(-1)}`,
      publishedAt: new Date(),
      metadata: { rootUri: root?.uri },
    };
  }

  private async prepare(): Promise<void> {
    if (!this.prepared) {
      await this.agent.login(this.credentials);
      this.prepared = true;
    }
  }

  private async postSegment(
    segment: FeedPost,
    root: StrongRef | undefined,
    parent: StrongRef | undefined,
    context: PublishContext,
  ): Promise<StrongRef> {
    const text = this.renderText(segment.text, segment.url);
    const richText = new RichText({ text });
    await richText.detectFacets(this.agent);
    const record: AppBskyFeedPost.Record = {
      $type: "app.bsky.feed.post",
      text: richText.text,
      facets: richText.facets,
      createdAt: new Date().toISOString(),
    };
    if (root && parent) record.reply = { root, parent };
    if (segment.media?.length)
      record.embed = await this.createImagesEmbed(segment.media, context);
    return this.agent.post(record);
  }

  private renderText(body: string, url?: string): string {
    if (!url) return this.truncate(body, this.maxGraphemes);
    const suffix = body ? `\n${url}` : url;
    return `${this.truncate(body, Math.max(0, this.maxGraphemes - this.length(suffix)))}${suffix}`;
  }

  private async createImagesEmbed(
    media: FeedMedia[],
    context: PublishContext,
  ): Promise<AppBskyFeedPost.Record["embed"]> {
    const images = await Promise.all(
      media.slice(0, 4).map(async (image) => {
        const bytes =
          image.data ?? (await this.fetchMedia(image, context.signal));
        const upload = await this.agent.uploadBlob(bytes, {
          encoding: image.mimeType,
        });
        return {
          alt: image.alt,
          image: upload.data.blob,
          aspectRatio:
            image.width && image.height
              ? { width: image.width, height: image.height }
              : undefined,
        };
      }),
    );
    return { $type: "app.bsky.embed.images", images };
  }

  private async fetchMedia(
    media: FeedMedia,
    signal?: AbortSignal,
  ): Promise<Uint8Array> {
    if (!media.url) throw new Error("Feed media requires either data or url.");
    const response = await fetch(media.url, { signal });
    if (!response.ok)
      throw new Error(
        `Could not fetch media (${response.status}) from ${media.url}.`,
      );
    return new Uint8Array(await response.arrayBuffer());
  }

  private length(text: string): number {
    return [...text].length;
  }

  private truncate(text: string, max: number): string {
    return [...text].slice(0, max).join("");
  }
}

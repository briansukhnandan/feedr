import { CronJob } from "cron";
import type { Feed } from "./types.js";
import { FeedRunner } from "./runner.js";

export interface ScheduledFeed {
  feed: Feed;

  /** Override `feed.schedule` when one deployment needs a different cadence. */
  schedule?: Feed["schedule"];
}

/** Owns cron jobs but leaves lifecycle management to the host application. */
export class FeedScheduler {
  private readonly jobs: CronJob[] = [];

  constructor(
    private readonly runner: FeedRunner,
    feeds: readonly ScheduledFeed[],
  ) {
    for (const { feed, schedule = feed.schedule } of feeds) {
      if (!schedule)
        throw new Error(`Feed "${feed.id}" needs a cron schedule.`);
      this.jobs.push(
        new CronJob(
          schedule.expression,
          async () => {
            await this.runner.run(feed);
          },
          undefined,
          false,
          schedule.timezone,
        ),
      );
    }
  }

  start(): void {
    this.jobs.forEach((job) => job.start());
  }

  stop(): void {
    this.jobs.forEach((job) => job.stop());
  }
}

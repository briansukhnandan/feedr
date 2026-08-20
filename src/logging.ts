import type { FeedLogger, LogEvent } from "./types.js";

/** Small default logger that can be replaced by pino, Winston, or BriNet-style file logging. */
export class ConsoleLogger implements FeedLogger {
  log(event: LogEvent): void {
    const prefix = [event.at.toISOString(), event.level.toUpperCase(), event.feedId, event.publisherId]
      .filter(Boolean)
      .join("][");
    const line = `[${prefix}] ${event.message}`;
    if (event.level === "error") console.error(line, event.error ?? "");
    else if (event.level === "warn") console.warn(line);
    else console.log(line);
  }
}

export const noopLogger: FeedLogger = { log: () => undefined };

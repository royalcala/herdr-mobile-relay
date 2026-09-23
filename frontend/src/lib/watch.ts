import type { Agent, QueueTask } from './types';

/**
 * The watchdog writes its state as flat key=value pairs, and the relay hands
 * them over untouched. Everything the panel shows — how alive it is, how deep
 * its queue is, how each scope answered last — is derived here, so the shape of
 * the file stays the watchdog's business.
 */
export type WatchFields = Record<string, string>;

export interface WatchScope {
  name: string;
  result: string;
  streak: string;
  epoch: number;
  error: string;
  /** Seconds since that scope last answered, or null when it never has. */
  ageSeconds: number | null;
}

export interface WatchView {
  alive: boolean;
  pid: string;
  manager: string;
  intervalSeconds: number | null;
  passes: number | null;
  queueDepth: number | null;
  /** Seconds since the last heartbeat: the number that has to keep moving. */
  heartbeatAgeSeconds: number | null;
  uptimeSeconds: number | null;
  scopes: WatchScope[];
  lastDelivery: { name: string; scope: string; why: string; pane: string } | null;
}

function num(fields: WatchFields, key: string): number | null {
  const raw = fields[key];
  if (!raw) return null;
  const value = Number(raw);
  return Number.isFinite(value) ? value : null;
}

function epochAge(seconds: number | null, nowSeconds: number): number | null {
  if (seconds === null || seconds <= 0) return null;
  return Math.max(0, Math.round(nowSeconds - seconds));
}

export function watchScopes(fields: WatchFields, nowSeconds: number): WatchScope[] {
  const names = new Set<string>();
  for (const key of Object.keys(fields)) {
    const match = /^scope\.([^.]+)\./.exec(key);
    if (match) names.add(match[1]);
  }
  return [...names].sort().map((name) => {
    const epoch = num(fields, `scope.${name}.epoch`) ?? 0;
    return {
      name,
      result: fields[`scope.${name}.result`] || 'unknown',
      streak: fields[`scope.${name}.streak`] || '',
      epoch,
      error: fields[`scope.${name}.error`] || '',
      ageSeconds: epochAge(epoch, nowSeconds),
    };
  });
}

export function watchView(fields: WatchFields, nowSeconds: number): WatchView {
  const pid = fields.pid || '';
  const lastDeliveryName = fields.last_delivery_name || '';
  return {
    alive: pid !== '',
    pid,
    manager: fields.manager || '',
    intervalSeconds: num(fields, 'interval'),
    passes: num(fields, 'passes'),
    queueDepth: num(fields, 'queue'),
    heartbeatAgeSeconds: epochAge(num(fields, 'heartbeat_epoch'), nowSeconds),
    uptimeSeconds: epochAge(num(fields, 'started_epoch'), nowSeconds),
    scopes: watchScopes(fields, nowSeconds),
    lastDelivery: lastDeliveryName
      ? {
        name: lastDeliveryName,
        scope: fields.last_delivery_scope || '',
        why: fields.last_delivery_why || '',
        pane: fields.last_delivery_pane || '',
      }
      : null,
  };
}

/** The watchdog's log lines, split into their timestamp and their event. */
export function watchEvents(lines: string[]): Array<{ at: string; text: string }> {
  return lines.slice().reverse().map((line) => {
    const match = /^(\S+)\s+(.*)$/.exec(line.trim());
    return match ? { at: match[1], text: match[2] } : { at: '', text: line.trim() };
  });
}

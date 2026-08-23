// Lightweight structured client logger. Keeps a rolling in-memory buffer
// (visible via the in-app log viewer / Share sheet) alongside console
// output, so a player's move history and connection lifecycle can be traced
// after the fact even without a remote log collector. Mirrors the server's
// `key=value` log shape (see server/internal/game/manager.go log.Printf
// calls) so client and server logs read the same way when compared side by
// side.

export type LogLevel = 'debug' | 'info' | 'warn' | 'error';

export type LogEntry = {
  ts: number;
  level: LogLevel;
  tag: string;
  message: string;
  data?: Record<string, unknown>;
};

const MAX_ENTRIES = 500;
const buffer: LogEntry[] = [];
let context: Record<string, unknown> = {};

type Listener = (entry: LogEntry) => void;
const listeners = new Set<Listener>();

function formatValue(v: unknown): string {
  if (v === undefined) return '';
  if (typeof v === 'object') {
    try {
      return JSON.stringify(v);
    } catch {
      return String(v);
    }
  }
  return String(v);
}

function formatEntry(entry: LogEntry): string {
  const time = new Date(entry.ts).toISOString().slice(11, 23);
  const parts = [`${time} [${entry.level}] [${entry.tag}]`, entry.message];
  const merged = { ...context, ...entry.data };
  for (const [k, v] of Object.entries(merged)) {
    if (v === undefined) continue;
    parts.push(`${k}=${formatValue(v)}`);
  }
  return parts.join(' ');
}

function push(level: LogLevel, tag: string, message: string, data?: Record<string, unknown>) {
  const entry: LogEntry = { ts: Date.now(), level, tag, message, data };
  buffer.push(entry);
  if (buffer.length > MAX_ENTRIES) buffer.shift();

  const line = formatEntry(entry);
  if (level === 'warn') console.warn(line);
  else if (level === 'error') console.error(line);
  else console.log(line);

  for (const listener of listeners) listener(entry);
}

export const logger = {
  /** Merges into every subsequent log line (e.g. gameId, userId). */
  setContext(patch: Record<string, unknown>) {
    context = { ...context, ...patch };
  },
  clearContext() {
    context = {};
  },
  debug(tag: string, message: string, data?: Record<string, unknown>) {
    push('debug', tag, message, data);
  },
  info(tag: string, message: string, data?: Record<string, unknown>) {
    push('info', tag, message, data);
  },
  warn(tag: string, message: string, data?: Record<string, unknown>) {
    push('warn', tag, message, data);
  },
  error(tag: string, message: string, data?: Record<string, unknown>) {
    push('error', tag, message, data);
  },
  getEntries(): LogEntry[] {
    return [...buffer];
  },
  formatAll(): string {
    return buffer.map(formatEntry).join('\n');
  },
  clear() {
    buffer.length = 0;
  },
  /** Called with every new entry as it's logged; returns an unsubscribe fn. */
  subscribe(listener: Listener): () => void {
    listeners.add(listener);
    return () => listeners.delete(listener);
  },
};

export function formatLogEntry(entry: LogEntry): string {
  return formatEntry(entry);
}

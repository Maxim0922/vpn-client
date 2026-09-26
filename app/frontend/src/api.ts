import { Call, Events } from "@wailsio/runtime";

const SERVICE = "main.VPN";
const underWails = typeof (globalThis as any).wails !== "undefined" || typeof window !== "undefined";

export interface Status {
  state: "disconnected" | "connecting" | "connected" | "error";
  server: string;
  publicIP?: string;
  since?: string;
  error?: string;
}
export interface Stats {
  rxBytes: number; txBytes: number; rxRate: number; txRate: number;
  lastHandshake: number; latencyMs: number;
}
export interface Settings {
  killSwitch: boolean; allowLAN: boolean; customDNS: string[];
  connectOnLaunch: boolean; lastServer: string; favorites: string[];
  publicIPService: string;
}
export interface ServerInfo { id: string; favorite: boolean; protocol: "wireguard" | "vless"; }
export interface LogEntry { ts: string; level: string; msg: string; }

async function call<T>(method: string, ...args: any[]): Promise<T> {
  try {
    return await Call.ByName(`${SERVICE}.${method}`, ...args);
  } catch (e) {
    console.error(`call ${method} failed`, e);
    throw e;
  }
}

export const api = {
  status: () => call<Status>("Status"),
  connect: (serverId: string) => call<Status>("Connect", serverId),
  disconnect: () => call<void>("Disconnect"),
  listServers: () => call<ServerInfo[]>("ListServers"),
  importConfig: (name: string, confText: string) => call<void>("ImportConfig", name, confText),
  removeServer: (id: string) => call<void>("RemoveServer", id),
  toggleFavorite: (id: string) => call<void>("ToggleFavorite", id),
  getSettings: () => call<Settings>("GetSettings"),
  setSettings: (s: Settings) => call<Settings>("SetSettings", s),
  stats: () => call<Stats>("Stats"),
  logs: (since: string) => call<LogEntry[]>("Logs", since),
  subscribe: () => call<void>("Subscribe"),
};

export function onStatus(cb: (s: Status) => void) {
  Events.On("status", (e: any) => cb(e.data ?? e));
}
export function onStats(cb: (s: Stats) => void) {
  Events.On("stats", (e: any) => cb(e.data ?? e));
}

export { underWails };

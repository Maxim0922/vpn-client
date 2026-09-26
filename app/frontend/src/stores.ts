import { writable } from "svelte/store";
import type { Status, Stats, Settings, ServerInfo } from "./api";
import { api, onStatus, onStats } from "./api";

export const status = writable<Status>({ state: "disconnected", server: "" });
export const stats = writable<Stats>({ rxBytes: 0, txBytes: 0, rxRate: 0, txRate: 0, lastHandshake: 0, latencyMs: -1 });
export const servers = writable<ServerInfo[]>([]);
export const settings = writable<Settings>({
  killSwitch: false, allowLAN: false, customDNS: [], connectOnLaunch: false,
  lastServer: "", favorites: [], publicIPService: "https://api.ipify.org",
});

export async function initStores() {
  onStatus((s) => status.set(s));
  onStats((s) => stats.set(s));
  try {
    status.set(await api.status());
    servers.set((await api.listServers()) ?? []);
    settings.set(await api.getSettings());
    await api.subscribe();
  } catch (e) {
    console.warn("initStores partial (daemon offline?)", e);
  }
}

export async function refreshServers() { servers.set((await api.listServers()) ?? []); }
export async function refreshSettings() { settings.set(await api.getSettings()); }

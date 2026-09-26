<script lang="ts">
  import { status, stats, settings, servers } from "../stores";
  import { api } from "../api";
  import { bytes, rate, sinceDuration, ping } from "../format";
  import { lookup, type Geo, type Marker } from "../geo";
  import Toggle from "./Toggle.svelte";
  import DotMap from "./DotMap.svelte";
  import { rise } from "../motion";

  let { onChangeLocation = () => {} }: { onChangeLocation?: () => void } = $props();

  let busy = $state(false);
  let now = $state(Date.now());
  setInterval(() => (now = Date.now()), 1000);

  const connected = $derived($status.state === "connected");
  const city = $derived($status.server || "—");
  const protocol = $derived(
    $servers.find((s) => s.id === ($status.server || $settings.lastServer))?.protocol === "vless" ? "VLESS" : "WireGuard",
  );

  function pickServer(): string {
    const ids = $servers.map((s) => s.id);
    if ($settings.lastServer && ids.includes($settings.lastServer)) return $settings.lastServer;
    return ($servers.find((s) => s.favorite) ?? $servers[0])?.id ?? "";
  }

  let connectError = $state("");
  async function connect() {
    const id = pickServer();
    if (!id) { onChangeLocation(); return; }
    busy = true;
    connectError = "";
    try { await api.connect(id); } catch (e) { connectError = String((e as any)?.message ?? e); }
    busy = false;
  }
  async function disconnect() { busy = true; try { await api.disconnect(); } finally { busy = false; } }
  function copyIP() { if ($status.publicIP) navigator.clipboard?.writeText($status.publicIP); }

  async function setKill(v: boolean) { const s = { ...$settings, killSwitch: v }; settings.set(await api.setSettings(s)); }

  let geo = $state<Geo | null>(null);
  $effect(() => {
    const st = $status.state;
    const ip = $status.publicIP;
    if (st === "connecting") return;
    if (st === "connected" && !ip) return;
    let live = true;
    lookup(st === "connected" ? ip : "").then((g) => { if (live) geo = g; });
    return () => { live = false; };
  });
  const marker = $derived<Marker | null>(geo && { lat: geo.lat, lon: geo.lon, tone: connected ? "accent" : "warn" });
</script>

<div class="mv-home">
<div class="mv-label">— STATUS</div>

<div class="mv-home__main">
  <div class="mv-home__left">
    {#key connected}
    <div class="mv-home__state" in:rise={{ y: 8, duration: 360 }}>
    {#if connected}
      <div class="mv-status-head">Protected<span class="dot">.</span></div>
      <div class="mv-city">{city}</div>
      <div class="text-muted">Your traffic is routed through the VPN tunnel.</div>

      <div class="mv-field" style="margin-top: var(--space-3); align-self: flex-start;">
        <span>{$status.publicIP ?? "resolving…"}</span>
        <button class="mv-btn mv-btn--ghost mv-label" onclick={copyIP}>COPY</button>
      </div>

      <div class="mv-row" style="margin-top: var(--space-4);">
        <button class="mv-btn mv-btn--secondary mv-btn--lg" disabled={busy} onclick={disconnect}>Disconnect</button>
        <button class="mv-btn mv-btn--ghost" onclick={onChangeLocation}>Change location</button>
      </div>
    {:else}
      <div class="mv-status-head">Not protected<span class="dot">.</span></div>
      <div class="text-muted">
        {#if $status.state === "connecting"}Connecting…{:else if $status.state === "error"}Error: {$status.error}{:else if connectError}Error: {connectError}{:else}Connect to route your traffic through the tunnel.{/if}
      </div>
      <div class="mv-row" style="margin-top: var(--space-4);">
        <button class="mv-btn mv-btn--primary mv-btn--lg" disabled={busy} onclick={connect}>Connect</button>
      </div>
    {/if}
    </div>
    {/key}

    <div class="mv-route">
      <span class="node">This Mac</span><span class="link"></span>
      <span class="node">{connected ? city : "VPN"}</span><span class="link"></span>
      <span class="node">Internet</span>
    </div>
  </div>

  <div class="mv-card mv-home__card">
    <div class="mv-card__map">
      <DotMap {marker} />
      <div class="mv-card__map-label" class:is-exposed={!connected}>
        <span class="mv-label">{connected ? "Exit" : "Exposed"}</span>
        <span>{geo ? `${geo.city}, ${geo.country}` : "locating…"}</span>
      </div>
    </div>
    <div class="mv-setting"><div><div class="mv-setting__label">Kill switch</div><div class="mv-setting__desc">Block all traffic if the tunnel drops</div></div><Toggle checked={$settings.killSwitch} onchange={setKill} /></div>
    <div class="mv-setting" style="border-bottom: none; padding-bottom: 0;"><div class="mv-setting__label">Protocol</div><span class="mv-host">{protocol}</span></div>
  </div>
</div>

<div class="mv-stats">
  <div><div class="mv-label">Download</div><div class="mv-stats__v">{rate($stats.rxRate)}</div><div class="text-faint mono">{bytes($stats.rxBytes)}</div></div>
  <div><div class="mv-label">Upload</div><div class="mv-stats__v">{rate($stats.txRate)}</div><div class="text-faint mono">{bytes($stats.txBytes)}</div></div>
  <div><div class="mv-label">Session</div><div class="mv-stats__v">{sinceDuration($status.since, now)}</div></div>
  <div><div class="mv-label">Latency</div><div class="mv-stats__v">{ping($stats.latencyMs)}</div></div>
</div>
</div>

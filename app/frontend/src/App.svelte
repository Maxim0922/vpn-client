<script lang="ts">
  import Sidebar from "./components/Sidebar.svelte";
  import Connect from "./components/Connect.svelte";
  import Locations from "./components/Locations.svelte";
  import Settings from "./components/Settings.svelte";
  import Log from "./components/Log.svelte";
  import { status } from "./stores";
  import { rise } from "./motion";

  let screen = $state("connect");

  function statusLine() {
    const s = $status;
    if (s.state === "connected")
      return { cls: "ok", text: `connected to ${s.server} … IP ${s.publicIP ?? "…"} · IPv6 blocked` };
    if (s.state === "connecting") return { cls: "warn", text: "connecting…" };
    if (s.state === "error") return { cls: "danger", text: `error: ${s.error}` };
    return { cls: "", text: "disconnected" };
  }
  const line = $derived(statusLine());

  function onKey(e: KeyboardEvent) {
    if (e.metaKey && e.key === ",") { e.preventDefault(); screen = "settings"; }
    if (e.metaKey && e.key === "f") { e.preventDefault(); screen = "locations"; }
  }
</script>

<svelte:window on:keydown={onKey} />

<div class="mv-titlebar"></div>
<div class="mv-app">
  <Sidebar active={screen} onNav={(v) => (screen = v)} />
  <div class="mv-content">
    <div class="mv-statusline" style="margin-bottom: var(--space-5);">
      <span class="mono">status:</span>
      <span class="{line.cls} mono"><span class="mv-statusdot" class:is-pulsing={$status.state === "connecting"}>●</span> {line.text}</span>
    </div>

    <div class="mv-screen">
    {#key screen}
    <div class="mv-screen__in" in:rise>
    {#if screen === "connect"}
      <Connect onChangeLocation={() => (screen = "locations")} />
    {:else if screen === "locations"}
      <Locations />
    {:else if screen === "settings"}
      <Settings />
    {:else if screen === "log"}
      <Log />
    {/if}
    </div>
    {/key}
    </div>
  </div>
</div>

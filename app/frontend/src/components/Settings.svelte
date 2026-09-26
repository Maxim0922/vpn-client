<script lang="ts">
  import { indicator } from "../motion";
  import { settings, refreshSettings } from "../stores";
  import { api } from "../api";
  import { applyTheme, loadTheme, type ThemeMode } from "../theme";
  import Toggle from "./Toggle.svelte";

  let tab = $state<"general" | "connection" | "privacy" | "account">("general");
  let theme = $state<ThemeMode>(loadTheme());
  let dnsText = $state("");

  $effect(() => { dnsText = ($settings.customDNS ?? []).join(", "); });

  async function patch(p: Partial<typeof $settings>) {
    settings.set(await api.setSettings({ ...$settings, ...p }));
  }
  function setTheme(m: ThemeMode) { theme = m; applyTheme(m); }

  async function saveDNS() {
    const list = dnsText.split(/[, ]+/).map((s) => s.trim()).filter(Boolean);
    await patch({ customDNS: list });
  }
  async function resetDNS() { dnsText = ""; await patch({ customDNS: [] }); }
</script>

<div class="mv-tabs" use:indicator>
  <div class="mv-ind"></div>
  {#each ["general","connection","privacy","account"] as t}
    <div class="mv-tab {tab===t?'is-active':''}" role="button" tabindex="0" onclick={() => (tab = t as any)} onkeydown={() => {}}>{t}</div>
  {/each}
</div>

{#if tab === "general"}
  <div class="mv-setting">
    <div><div class="mv-setting__label">Appearance</div><div class="mv-setting__desc">Ink / Paper / System</div></div>
    <div class="mv-segment" use:indicator>
      <div class="mv-ind"></div>
      <div class="mv-segment__opt {theme==='dark'?'is-active':''}" role="button" tabindex="0" onclick={() => setTheme('dark')} onkeydown={() => {}}>Ink</div>
      <div class="mv-segment__opt {theme==='light'?'is-active':''}" role="button" tabindex="0" onclick={() => setTheme('light')} onkeydown={() => {}}>Paper</div>
      <div class="mv-segment__opt {theme==='system'?'is-active':''}" role="button" tabindex="0" onclick={() => setTheme('system')} onkeydown={() => {}}>System</div>
    </div>
  </div>
  <div class="mv-setting">
    <div><div class="mv-setting__label">Connect on launch</div><div class="mv-setting__desc">Reconnect to the last server automatically</div></div>
    <Toggle checked={$settings.connectOnLaunch} onchange={(v) => patch({ connectOnLaunch: v })} />
  </div>
{:else if tab === "connection"}
  <div class="mv-setting">
    <div><div class="mv-setting__label">Kill switch</div><div class="mv-setting__desc">Block all traffic if the tunnel drops</div></div>
    <Toggle checked={$settings.killSwitch} onchange={(v) => patch({ killSwitch: v })} />
  </div>
  <div class="mv-setting">
    <div><div class="mv-setting__label">Allow LAN</div><div class="mv-setting__desc">Permit local network access with kill switch on</div></div>
    <Toggle checked={$settings.allowLAN} onchange={(v) => patch({ allowLAN: v })} />
  </div>
{:else if tab === "privacy"}
  <div class="mv-col">
    <div class="mv-setting__label">Custom DNS</div>
    <input class="mv-search" bind:value={dnsText} placeholder="1.1.1.1, 1.0.0.1" />
    <div class="mv-row">
      <button class="mv-btn mv-btn--ghost" onclick={resetDNS}>Reset</button>
      <button class="mv-btn mv-btn--primary" onclick={saveDNS}>Save</button>
    </div>
  </div>
{:else}
  <div class="text-muted">Account features are not part of the local build.</div>
{/if}

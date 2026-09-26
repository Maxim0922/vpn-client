<script lang="ts">
  import { servers } from "../stores";
  import { indicator } from "../motion";

  let { active = "connect", onNav = (_: string) => {} }:
    { active?: string; onNav?: (v: string) => void } = $props();

  const items = [
    { id: "connect", label: "Connect" },
    { id: "locations", label: "Locations" },
    { id: "settings", label: "Settings" },
    { id: "log", label: "Log" },
  ];
  const recent = $derived($servers.slice(0, 3));
</script>

<div class="mv-side" use:indicator>
  <div class="mv-ind"></div>
  {#each items as it}
    <div class="mv-side__item {active === it.id ? 'is-active' : ''}" role="button" tabindex="0"
         onclick={() => onNav(it.id)} onkeydown={(e) => e.key === 'Enter' && onNav(it.id)}>
      {it.label}
    </div>
  {/each}

  <div class="mv-side__spacer"></div>

  <div class="mv-label" style="padding: var(--space-2) var(--space-3);">Recent</div>
  {#each recent as s}
    <div class="mv-side__item" role="button" tabindex="0" onclick={() => onNav('locations')} onkeydown={() => {}}>{s.id}</div>
  {/each}
</div>

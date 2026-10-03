<script lang="ts">
  import { servers, status, refreshServers } from "../stores";
  import { api, type ServerInfo } from "../api";

  let query = $state("");
  let fileInput: HTMLInputElement;

  const filtered = $derived($servers.filter((s) => s.id.toLowerCase().includes(query.toLowerCase())));
  const favs = $derived(filtered.filter((s) => s.favorite));
  const all = $derived(filtered);

  async function connect(id: string) { try { await api.connect(id); } catch (e) { console.error(e); } }
  async function toggleFav(id: string) { await api.toggleFavorite(id); await refreshServers(); }
  async function remove(id: string) { await api.removeServer(id); await refreshServers(); }

  let link = $state("");
  let importError = $state("");

  function linkName(text: string): string {
    const raw = text.trim();
    let name = "";
    try {
      const u = new URL(raw);
      name = decodeURIComponent(u.hash.slice(1)) || u.hostname;
    } catch {}
    name = name.replace(/[^\p{L}\p{N} ._-]+/gu, " ").replace(/\s+/g, " ").trim().replace(/^\.+/, "");
    return name.slice(0, 64) || "vless";
  }
  const isLink = (text: string) => text.trim().startsWith("vless://");

  async function importText(fallbackName: string, text: string) {
    importError = "";
    try {
      await api.importConfig(isLink(text) ? linkName(text) : fallbackName, text);
      await refreshServers();
      return true;
    } catch (e) {
      importError = String((e as any)?.message ?? e);
      return false;
    }
  }

  async function importFile(f: File) {
    await importText(f.name.replace(/\.(conf|ovpn|txt)$/i, ""), await f.text());
  }
  async function onFile(e: Event) {
    const f = (e.target as HTMLInputElement).files?.[0];
    if (f) await importFile(f);
  }
  async function onDrop(e: DragEvent) {
    e.preventDefault();
    const f = e.dataTransfer?.files?.[0];
    if (f) await importFile(f);
  }
  async function addLink() {
    if (await importText("", link)) link = "";
  }

  const cc = (id: string) => id.slice(0, 2).toUpperCase();
</script>

<div class="mv-row mv-spread">
  <input class="mv-search" placeholder="Search locations (⌘F)" bind:value={query} />
  <button class="mv-btn mv-btn--secondary" style="margin-left: var(--space-3);" onclick={() => fileInput.click()}>Import file</button>
  <input type="file" accept=".conf,.ovpn,.txt" bind:this={fileInput} onchange={onFile} style="display:none" />
</div>

<div role="region" aria-label="Drop a config file or paste a vless link" ondragover={(e) => e.preventDefault()} ondrop={onDrop} style="margin-top: var(--space-4); border: 1px dashed var(--line); border-radius: var(--radius-md); padding: var(--space-2);">
  <div class="mv-label" style="padding: var(--space-2);">Drop a WireGuard .conf, OpenVPN .ovpn, or paste a vless:// link</div>
  <form class="mv-row" style="padding: 0 var(--space-2) var(--space-2);" onsubmit={(e) => { e.preventDefault(); addLink(); }}>
    <input class="mv-search" style="flex: 1;" placeholder="vless://…" bind:value={link} />
    <button class="mv-btn mv-btn--secondary" type="submit" disabled={!isLink(link)}>Add</button>
  </form>
  {#if importError}<div class="mono mv-fade" style="color: var(--danger); padding: 0 var(--space-2) var(--space-2); font-size: 12px;">{importError}</div>{/if}
</div>

{#if favs.length}
  <div class="mv-label" style="margin: var(--space-4) 0 var(--space-2);">Favourites</div>
  {#each favs as s, i (s.id)}
    {@render row(s, i)}
  {/each}
{/if}

<div class="mv-label" style="margin: var(--space-4) 0 var(--space-2);">All locations</div>
{#if !all.length}<div class="text-faint">No servers. Import a WireGuard .conf, OpenVPN .ovpn, or a vless:// link to begin.</div>{/if}
{#each all as s, i (s.id)}
  {@render row(s, i)}
{/each}

{#snippet row(s: ServerInfo, i: number)}
  <div class="mv-server mv-stagger {$status.server === s.id ? 'is-selected' : ''}" style="--i: {Math.min(i, 12)}">
    <span class="mv-cc">{cc(s.id)}</span>
    <button class="mv-btn mv-btn--ghost" style="justify-self:start;padding:0" onclick={() => connect(s.id)}>{s.id}</button>
    <span class="mv-host">{s.protocol === "vless" ? "VLESS" : s.protocol === "openvpn" ? "OpenVPN" : "WireGuard"}</span>
    <div class="mv-load"><div class="mv-load__fill" style="width: 40%"></div></div>
    <span class="mv-ping">—</span>
    <span class="mv-star {s.favorite ? 'is-on' : ''}" role="button" tabindex="0" onclick={() => toggleFav(s.id)} onkeydown={() => {}}>★</span>
  </div>
{/snippet}

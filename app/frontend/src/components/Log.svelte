<script lang="ts">
  import { api, type LogEntry } from "../api";

  let entries = $state<LogEntry[]>([]);

  async function load() {
    try { entries = (await api.logs("")) ?? []; } catch (e) { console.error(e); }
  }
  load();
  const timer = setInterval(load, 2000);
  $effect(() => () => clearInterval(timer));

  function copy() { navigator.clipboard?.writeText(entries.map((e) => `${e.ts} ${e.level} ${e.msg}`).join("\n")); }

  function markup(msg: string): string {
    return msg
      .replace(/\b(connected|reconnected|up)\b/g, '<span class="w-ok">$1</span>')
      .replace(/\b(stale|rolling back|retry|reconnecting)\b/g, '<span class="w-warn">$1</span>')
      .replace(/\b(error|failed|denied|rejected)\b/gi, '<span class="w-danger">$1</span>');
  }
  const t = (iso: string) => new Date(iso).toLocaleTimeString();
</script>

<div class="mv-row mv-spread" style="margin-bottom: var(--space-3);">
  <div class="mv-label">— LOG</div>
  <button class="mv-btn mv-btn--ghost" onclick={copy}>Copy</button>
</div>

<div class="mv-log" style="height: 420px;">
  {#each entries as e}
    <div class="mv-log__row"><span class="mv-log__t">{t(e.ts)}</span> {@html markup(e.msg)}</div>
  {/each}
  {#if !entries.length}<div class="text-faint">No log entries.</div>{/if}
</div>

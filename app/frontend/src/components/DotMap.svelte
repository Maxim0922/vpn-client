<script lang="ts">
  import { MAP } from "../map";
  import type { Marker } from "../geo";
  import { Tween } from "svelte/motion";
  import { cubicInOut } from "svelte/easing";
  import { reducedMotion } from "../motion";
  let { marker = null }: { marker?: Marker | null } = $props();

  const HALO = 6;

  const dots: { x: number; y: number }[] = [];
  MAP.land.forEach((row, y) => {
    for (let x = 0; x < row.length; x++) if (row[x] === "#") dots.push({ x: x + 0.5, y: y + 0.5 });
  });

  const clamp = (v: number, max: number) => Math.min(Math.max(v, 0.5), max - 0.5);
  const target = $derived(
    marker
      ? {
          x: clamp((marker.lon - MAP.lonMin) / MAP.lonStep, MAP.cols),
          y: clamp((MAP.latMax - marker.lat) / MAP.latStep, MAP.rows),
        }
      : null,
  );

  const tween = new Tween({ x: 0, y: 0 }, { duration: 900, easing: cubicInOut });
  let shown = false;
  $effect(() => {
    if (!target) return;
    tween.set(target, { duration: shown && !reducedMotion() ? 900 : 0 });
    shown = true;
  });
  const pos = $derived(target ? tween.current : null);
  const halo = $derived(
    pos
      ? dots
          .map((d) => ({ ...d, k: 1 - Math.hypot(d.x - pos.x, d.y - pos.y) / HALO }))
          .filter((d) => d.k > 0)
      : [],
  );
</script>

<svg class="mv-map" viewBox="0 0 {MAP.cols} {MAP.rows}" preserveAspectRatio="xMidYMid meet" aria-hidden="true">
  {#each dots as d}
    <circle cx={d.x} cy={d.y} r="0.34" class="mv-map__dot" />
  {/each}
  {#if pos && marker}
    <g class="mv-map__tone--{marker.tone}">
      {#each halo as d}
        <circle cx={d.x} cy={d.y} r="0.34" class="mv-map__hot" fill-opacity={0.2 + 0.8 * d.k} />
      {/each}
      <circle cx={pos.x} cy={pos.y} r="1" class="mv-map__pulse" />
      <circle cx={pos.x} cy={pos.y} r="0.9" class="mv-map__pin" />
      <circle cx={pos.x} cy={pos.y} r="0.4" class="mv-map__core" />
    </g>
  {/if}
</svg>

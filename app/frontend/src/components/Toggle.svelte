<script lang="ts">
  let { checked = false, size = "md", onchange = (_: boolean) => {} }:
    { checked?: boolean; size?: "md" | "lg"; onchange?: (v: boolean) => void } = $props();
</script>

<div
  class="mv-toggle {size === 'lg' ? 'mv-toggle--lg' : ''} {checked ? 'is-on' : ''}"
  role="switch"
  aria-checked={checked}
  tabindex="0"
  onclick={() => onchange(!checked)}
  onkeydown={(e) => { if (e.key === " " || e.key === "Enter") { e.preventDefault(); onchange(!checked); } }}
>
  <div class="mv-toggle__knob"></div>
</div>

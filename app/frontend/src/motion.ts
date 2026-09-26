import { cubicOut } from "svelte/easing";

export const reducedMotion = () => window.matchMedia("(prefers-reduced-motion: reduce)").matches;

export function rise(_: Element, { y = 6, duration = 280, delay = 0 } = {}) {
  if (reducedMotion()) return { duration: 0 };
  return {
    delay,
    duration,
    easing: cubicOut,
    css: (t: number) =>
      `opacity: ${t}; transform: translateY(${(1 - t) * y}px) scale(${0.99 + 0.01 * t});`,
  };
}

export function indicator(node: HTMLElement) {
  const ind = node.querySelector<HTMLElement>(":scope > .mv-ind");
  if (!ind) return {};
  let placed = false;

  const place = () => {
    const a = node.querySelector<HTMLElement>(":scope > .is-active");
    if (!a) {
      ind.style.opacity = "0";
      return;
    }
    if (!placed) ind.style.transition = "none";
    ind.style.opacity = "1";
    ind.style.transform = `translate(${a.offsetLeft}px, ${a.offsetTop}px)`;
    ind.style.width = `${a.offsetWidth}px`;
    ind.style.height = `${a.offsetHeight}px`;
    if (!placed) {
      void ind.offsetWidth;
      ind.style.transition = "";
      placed = true;
    }
  };

  const mo = new MutationObserver(place);
  mo.observe(node, { subtree: true, childList: true, attributes: true, attributeFilter: ["class"] });
  const ro = new ResizeObserver(place);
  ro.observe(node);
  place();
  return {
    destroy() {
      mo.disconnect();
      ro.disconnect();
    },
  };
}

export type ThemeMode = "dark" | "light" | "system";

const KEY = "maxvpn-theme";

export function applyTheme(mode: ThemeMode) {
  const root = document.documentElement;
  root.classList.add("mv-theme-fade");
  setTimeout(() => root.classList.remove("mv-theme-fade"), 400);
  if (mode === "system") {
    const dark = window.matchMedia("(prefers-color-scheme: dark)").matches;
    root.setAttribute("data-theme", dark ? "dark" : "light");
  } else {
    root.setAttribute("data-theme", mode);
  }
  localStorage.setItem(KEY, mode);
}

export function loadTheme(): ThemeMode {
  return (localStorage.getItem(KEY) as ThemeMode) || "system";
}

export function initTheme() {
  const mode = loadTheme();
  applyTheme(mode);
  window.matchMedia("(prefers-color-scheme: dark)").addEventListener("change", () => {
    if (loadTheme() === "system") applyTheme("system");
  });
}

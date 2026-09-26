import { readFileSync, writeFileSync, mkdirSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, resolve } from "node:path";

const __dirname = dirname(fileURLToPath(import.meta.url));
const tokensPath = resolve(__dirname, "../../../design/tokens.json");
const outPath = resolve(__dirname, "../src/tokens.css");

const t = JSON.parse(readFileSync(tokensPath, "utf8"));

const scalarVars = () => {
  const lines = [];
  lines.push(`  --font-sans: ${t.font.sans};`);
  lines.push(`  --font-mono: ${t.font.mono};`);
  for (const [k, v] of Object.entries(t.radius)) lines.push(`  --radius-${k}: ${v};`);
  for (const [k, v] of Object.entries(t.space)) lines.push(`  --space-${k}: ${v};`);
  for (const [k, v] of Object.entries(t.duration)) lines.push(`  --duration-${k}: ${v};`);
  return lines.join("\n");
};

const themeVars = (name) => {
  return Object.entries(t.themes[name])
    .map(([k, v]) => `  --${k}: ${v};`)
    .join("\n");
};

const css = `:root {
${scalarVars()}
${themeVars("ink")}
}

[data-theme="dark"] {
${themeVars("ink")}
}

[data-theme="light"] {
${themeVars("paper")}
}
`;

mkdirSync(dirname(outPath), { recursive: true });
writeFileSync(outPath, css);
console.log("wrote", outPath);

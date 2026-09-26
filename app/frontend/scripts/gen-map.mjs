import { readFileSync, writeFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const outPath = resolve(__dirname, "../src/map.ts");
const SRC =
  "https://raw.githubusercontent.com/nvkelso/natural-earth-vector/master/geojson/ne_50m_land.geojson";

const COLS = 64;
const LON_MIN = -12, LON_MAX = 40;
const LAT_MAX = 61, LAT_MIN = 35;
const LON_STEP = (LON_MAX - LON_MIN) / COLS;
const LAT_STEP = LON_STEP * Math.cos((48 * Math.PI) / 180);
const ROWS = Math.round((LAT_MAX - LAT_MIN) / LAT_STEP);

const geo = process.argv[2]
  ? JSON.parse(readFileSync(process.argv[2], "utf8"))
  : await (await fetch(SRC)).json();

const polys = [];
for (const f of geo.features) {
  const g = f.geometry;
  if (g.type === "Polygon") polys.push(g.coordinates);
  else if (g.type === "MultiPolygon") polys.push(...g.coordinates);
}

function inRing(x, y, ring) {
  let inside = false;
  for (let i = 0, j = ring.length - 1; i < ring.length; j = i++) {
    const [xi, yi] = ring[i], [xj, yj] = ring[j];
    if (yi > y !== yj > y && x < ((xj - xi) * (y - yi)) / (yj - yi) + xi) inside = !inside;
  }
  return inside;
}
function isLand(lon, lat) {
  for (const rings of polys) {
    if (inRing(lon, lat, rings[0]) && !rings.slice(1).some((h) => inRing(lon, lat, h))) return true;
  }
  return false;
}

const rows = [];
for (let r = 0; r < ROWS; r++) {
  let line = "";
  for (let c = 0; c < COLS; c++) {
    let hits = 0;
    for (let sy = 0; sy < 3; sy++)
      for (let sx = 0; sx < 3; sx++) {
        const lon = LON_MIN + (c + (sx + 0.5) / 3) * LON_STEP;
        const lat = LAT_MAX - (r + (sy + 0.5) / 3) * LAT_STEP;
        if (isLand(lon, lat)) hits++;
      }
    line += hits >= 3 ? "#" : ".";
  }
  rows.push(line);
}

const ts = `export const MAP = {
  cols: ${COLS},
  rows: ${ROWS},
  lonMin: ${LON_MIN},
  latMax: ${LAT_MAX},
  lonStep: ${LON_STEP},
  latStep: ${LAT_STEP},
  land: [
${rows.map((r) => `    "${r}",`).join("\n")}
  ],
};
`;
writeFileSync(outPath, ts);
console.log(`wrote ${outPath} (${COLS}x${ROWS}, ${rows.join("").split("#").length - 1} dots)`);

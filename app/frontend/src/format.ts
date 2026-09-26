export function bytes(n: number): string {
  if (n < 1024) return `${n} B`;
  const u = ["KB", "MB", "GB", "TB"];
  let i = -1;
  do { n /= 1024; i++; } while (n >= 1024 && i < u.length - 1);
  return `${n.toFixed(1)} ${u[i]}`;
}

export function rate(n: number): string {
  return `${bytes(n)}/s`;
}

export function sinceDuration(iso?: string, now = Date.now()): string {
  if (!iso) return "—";
  const start = new Date(iso).getTime();
  if (isNaN(start)) return "—";
  let s = Math.max(0, Math.floor((now - start) / 1000));
  const h = Math.floor(s / 3600); s -= h * 3600;
  const m = Math.floor(s / 60); s -= m * 60;
  const pad = (x: number) => String(x).padStart(2, "0");
  return `${pad(h)}:${pad(m)}:${pad(s)}`;
}

export function ping(ms: number): string {
  return ms < 0 ? "—" : `${ms} ms`;
}

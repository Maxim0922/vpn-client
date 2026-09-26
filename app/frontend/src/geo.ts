export interface Marker { lat: number; lon: number; tone: "accent" | "warn"; }
export interface Geo { ip: string; lat: number; lon: number; city: string; country: string; }

const cache = new Map<string, Promise<Geo | null>>();

export function lookup(ip = ""): Promise<Geo | null> {
  let p = cache.get(ip);
  if (!p) {
    p = fetch(`https://ipwho.is/${ip}`)
      .then((r) => r.json())
      .then((j) =>
        j.success
          ? { ip: j.ip, lat: j.latitude, lon: j.longitude, city: j.city, country: j.country_code }
          : null,
      )
      .catch(() => null);
    if (ip) cache.set(ip, p);
  }
  return p;
}

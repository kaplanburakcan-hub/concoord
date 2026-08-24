import { useEffect, useState } from "react";
import type { ReactNode } from "react";
import { useProjects } from "../projects/ProjectContext";

const AYLAR = [
  "Ocak", "Şubat", "Mart", "Nisan", "Mayıs", "Haziran",
  "Temmuz", "Ağustos", "Eylül", "Ekim", "Kasım", "Aralık",
];
const GUNLER = ["Pazar", "Pazartesi", "Salı", "Çarşamba", "Perşembe", "Cuma", "Cumartesi"];

function fmt(d: Date) {
  const gun = d.getDate();
  const ay = AYLAR[d.getMonth()];
  const yil = d.getFullYear();
  const haftaGunu = GUNLER[d.getDay()];
  const ss = String(d.getHours()).padStart(2, "0");
  const dd = String(d.getMinutes()).padStart(2, "0");
  return { date: `${gun} ${ay} ${yil} ${haftaGunu}`, time: `${ss}:${dd}` };
}

type Weather = { tempC: number; code: number; humidity: number; windKmh: number } | null;

// Konum → koordinat ve hava durumu, aynı konum için sık sık yeniden
// sorgulanmasın diye modül seviyesinde (bileşen ömrünü aşan) önbelleklenir.
const weatherCache = new Map<string, { data: Weather; at: number }>();
const CACHE_MS = 20 * 60 * 1000;

async function geocode(query: string): Promise<{ lat: number; lon: number } | null> {
  try {
    const res = await fetch(
      `https://geocoding-api.open-meteo.com/v1/search?name=${encodeURIComponent(query)}&count=1&language=tr&format=json`
    );
    const j = await res.json();
    const r = j?.results?.[0];
    return r ? { lat: r.latitude, lon: r.longitude } : null;
  } catch {
    return null;
  }
}

async function fetchWeather(lat: number, lon: number): Promise<Weather> {
  try {
    const res = await fetch(
      `https://api.open-meteo.com/v1/forecast?latitude=${lat}&longitude=${lon}&current=temperature_2m,relative_humidity_2m,weather_code,wind_speed_10m`
    );
    const j = await res.json();
    const c = j?.current;
    if (!c) return null;
    return {
      tempC: Math.round(c.temperature_2m),
      code: c.weather_code,
      humidity: Math.round(c.relative_humidity_2m),
      windKmh: Math.round(c.wind_speed_10m),
    };
  } catch {
    return null;
  }
}

function IconSun() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" width="100%" height="100%">
      <circle cx="12" cy="12" r="4.5" />
      <path d="M12 2.5v2.5M12 19v2.5M4.2 4.2l1.8 1.8M18 18l1.8 1.8M2.5 12H5M19 12h2.5M4.2 19.8 6 18M18 6l1.8-1.8" />
    </svg>
  );
}
function IconCloudSun() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" width="100%" height="100%">
      <path d="M9 3.5v2M4.4 6l1.4 1.4M3 11h2" />
      <circle cx="9" cy="9" r="3" />
      <path d="M6.5 19h10.8a3.7 3.7 0 0 0 .5-7.36A5 5 0 0 0 8.3 13" />
    </svg>
  );
}
function IconCloud() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" width="100%" height="100%">
      <path d="M6.5 19h10.8a3.7 3.7 0 0 0 .5-7.36A6 6 0 0 0 6.2 13.1 4 4 0 0 0 6.5 19z" />
    </svg>
  );
}
function IconFog() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" width="100%" height="100%">
      <path d="M4 9.5h10.5a3.6 3.6 0 1 0-.5-7.1A5 5 0 0 0 5.3 5.6" />
      <path d="M3 14h18M3 18h18" />
    </svg>
  );
}
function IconRain() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" width="100%" height="100%">
      <path d="M6.5 15.5h10.8a3.7 3.7 0 0 0 .5-7.36 6 6 0 0 0-11.6-1.4A4 4 0 0 0 6.5 15.5z" />
      <path d="M8 18.5 7 21M12.5 18.5l-1 2.5M17 18.5l-1 2.5" />
    </svg>
  );
}
function IconSnow() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" width="100%" height="100%">
      <path d="M6.5 14.5h10.8a3.7 3.7 0 0 0 .5-7.36 6 6 0 0 0-11.6-1.4A4 4 0 0 0 6.5 14.5z" />
      <path d="M9 18v3M9 18.5l-1.5 1M9 18.5l1.5 1M15 18v3M15 18.5l-1.5 1M15 18.5l1.5 1" />
    </svg>
  );
}
function IconStorm() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" width="100%" height="100%">
      <path d="M6.5 13.5h10.8a3.7 3.7 0 0 0 .5-7.36 6 6 0 0 0-11.6-1.4A4 4 0 0 0 6.5 13.5z" />
      <path d="M13 14.5 10 19h3l-2 4" />
    </svg>
  );
}
function IconDroplet() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" width="100%" height="100%">
      <path d="M12 2.5S5.5 10 5.5 14.5a6.5 6.5 0 0 0 13 0C18.5 10 12 2.5 12 2.5z" />
    </svg>
  );
}
function IconWind() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" width="100%" height="100%">
      <path d="M3 8h11a2.5 2.5 0 1 0-2.3-3.5" />
      <path d="M3 16h14a2.5 2.5 0 1 1-2.3 3.5" />
      <path d="M3 12h8" />
    </svg>
  );
}

function weatherIcon(code: number): { icon: ReactNode; label: string } {
  if (code === 0) return { icon: <IconSun />, label: "Açık" };
  if (code === 1 || code === 2) return { icon: <IconCloudSun />, label: "Az bulutlu" };
  if (code === 3) return { icon: <IconCloud />, label: "Bulutlu" };
  if (code === 45 || code === 48) return { icon: <IconFog />, label: "Sisli" };
  if ([51, 53, 55, 56, 57, 61, 63, 65, 66, 67, 80, 81, 82].includes(code)) return { icon: <IconRain />, label: "Yağmurlu" };
  if ([71, 73, 75, 77, 85, 86].includes(code)) return { icon: <IconSnow />, label: "Karlı" };
  if ([95, 96, 99].includes(code)) return { icon: <IconStorm />, label: "Fırtınalı" };
  return { icon: <IconCloud />, label: "Bulutlu" };
}

export default function HeaderClock() {
  const { current } = useProjects();
  const [now, setNow] = useState(() => new Date());
  const [weather, setWeather] = useState<Weather>(null);
  const location = current?.location?.trim() || "";

  useEffect(() => {
    const id = setInterval(() => setNow(new Date()), 1000);
    return () => clearInterval(id);
  }, []);

  useEffect(() => {
    if (!location) {
      setWeather(null);
      return;
    }
    const cached = weatherCache.get(location);
    if (cached && Date.now() - cached.at < CACHE_MS) {
      setWeather(cached.data);
      return;
    }
    let cancelled = false;
    (async () => {
      const query = location.split("/")[0].trim() || location;
      const geo = (await geocode(query)) ?? (await geocode(location));
      if (!geo || cancelled) return;
      const w = await fetchWeather(geo.lat, geo.lon);
      if (cancelled) return;
      weatherCache.set(location, { data: w, at: Date.now() });
      setWeather(w);
    })();
    return () => {
      cancelled = true;
    };
  }, [location]);

  const { date, time } = fmt(now);
  const w = weather ? weatherIcon(weather.code) : null;

  return (
    <div
      className="hidden sm:flex flex-1 min-w-0 items-center justify-center gap-4 py-1.5 rounded-[10px] px-4 border"
      style={{ background: "rgba(255,255,255,.05)", borderColor: "var(--chrome-border)" }}
    >
      {weather && w && (
        <>
          <div className="flex items-center gap-2.5 shrink-0" title={`${current?.location ?? ""} · ${w.label}`}>
            <span style={{ color: "var(--group-accent)", width: 24, height: 24, display: "block" }}>{w.icon}</span>
            <div className="flex flex-col gap-0.5 leading-none">
              <div className="flex items-baseline gap-1.5">
                <span className="text-[19px] font-extrabold tabular-nums" style={{ color: "var(--group-accent)" }}>
                  {weather.tempC}°
                </span>
                <span className="text-[10.5px] font-medium" style={{ color: "var(--chrome-text-2)" }}>
                  {w.label}
                </span>
              </div>
              <div className="flex items-center gap-2.5 text-[10px]" style={{ color: "var(--chrome-text-3)" }}>
                <span className="flex items-center gap-1">
                  <span style={{ width: 10, height: 10, display: "block" }}><IconDroplet /></span>
                  {weather.humidity}%
                </span>
                <span className="flex items-center gap-1">
                  <span style={{ width: 10, height: 10, display: "block" }}><IconWind /></span>
                  {weather.windKmh} km/h
                </span>
              </div>
            </div>
          </div>
          <span className="w-px self-stretch shrink-0" style={{ background: "var(--chrome-border)" }} />
        </>
      )}
      <div className="flex flex-col gap-0.5 leading-none shrink-0">
        <span className="text-[10.5px] font-medium truncate" style={{ color: "var(--chrome-text-2)" }}>
          {date}
        </span>
        <span
          className="text-[21px] font-bold tabular-nums"
          style={{
            color: "var(--group-accent)",
            fontFamily: "'JetBrains Mono', ui-monospace, SFMono-Regular, Menlo, Consolas, monospace",
            letterSpacing: "-0.02em",
          }}
        >
          {time}
        </span>
      </div>
    </div>
  );
}

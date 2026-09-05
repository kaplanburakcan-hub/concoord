import { useEffect, useRef, useState } from "react";
import { useSearchParams } from "react-router-dom";
import QRCode from "qrcode";
import { api } from "../../api/client";
import { useProjects } from "../ProjectContext";

// QrPanoPage — şantiye girişindeki tablet/ekran için tam ekran, kimlik
// doğrulamalı (AppShell'siz — bkz. App.tsx route kaydı) görünüm. Kod
// STATİK DEĞİLDİR: her 60 saniyede bir yeni, tek kullanımlık token çekilir.

type Geofence = { id: string; name: string };
type QRToken = { token: string; geofence_id: string; issued_at: string; expires_at: string };

export default function QrPanoPage() {
  const { current } = useProjects();
  const [params] = useSearchParams();
  const initialGeofence = params.get("geofence");

  const [geofences, setGeofences] = useState<Geofence[]>([]);
  const [selected, setSelected] = useState<string | null>(initialGeofence);
  const [qrDataUrl, setQrDataUrl] = useState<string | null>(null);
  const [secondsLeft, setSecondsLeft] = useState(60);
  const [err, setErr] = useState<string | null>(null);
  const tickRef = useRef<ReturnType<typeof setInterval> | null>(null);

  useEffect(() => {
    if (!current?.id) return;
    api<{ geofences: Geofence[] }>(`/projects/${current.id}/geofences`, { projectId: current.id }).then((res) => {
      setGeofences(res.geofences);
      if (!selected && res.geofences.length > 0) setSelected(res.geofences[0].id);
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [current?.id]);

  useEffect(() => {
    if (!selected) return;
    let cancelled = false;

    async function fetchToken() {
      try {
        const res = await api<{ qr_token: QRToken }>(`/geofences/${selected}/qr-token`);
        if (cancelled) return;
        const url = `${window.location.origin}/pdks?token=${res.qr_token.token}`;
        const dataUrl = await QRCode.toDataURL(url, {
          width: 480, margin: 2, color: { dark: "#0f1115", light: "#ffffff" },
        });
        if (!cancelled) {
          setQrDataUrl(dataUrl);
          setSecondsLeft(60);
          setErr(null);
        }
      } catch {
        if (!cancelled) setErr("Kod alınamadı. Bağlantı kontrol ediliyor…");
      }
    }

    fetchToken();
    const interval = setInterval(fetchToken, 60000);
    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, [selected]);

  useEffect(() => {
    if (tickRef.current) clearInterval(tickRef.current);
    tickRef.current = setInterval(() => setSecondsLeft((s) => Math.max(0, s - 1)), 1000);
    return () => {
      if (tickRef.current) clearInterval(tickRef.current);
    };
  }, [qrDataUrl]);

  const geofenceName = geofences.find((g) => g.id === selected)?.name ?? "";

  return (
    <div className="min-h-screen flex flex-col items-center justify-center gap-6 p-8" style={{ background: "#0d0f14" }}>
      <div className="text-center">
        <p className="text-[11px] font-bold uppercase tracking-[.15em]" style={{ color: "var(--group-accent)" }}>
          PDKS Giriş-Çıkış
        </p>
        <h1 className="font-display text-2xl font-extrabold text-white mt-1">
          {geofenceName || (geofences.length === 0 ? "Şantiye sınırı tanımlanmamış" : "Yükleniyor…")}
        </h1>
      </div>

      {geofences.length > 1 && (
        <select
          value={selected ?? ""}
          onChange={(e) => setSelected(e.target.value)}
          className="rounded-md bg-beton-900 border border-beton-700 px-3 py-1.5 text-sm text-beton-200"
        >
          {geofences.map((g) => (
            <option key={g.id} value={g.id}>{g.name}</option>
          ))}
        </select>
      )}

      {qrDataUrl ? (
        <div className="bg-white p-6 rounded-2xl">
          <img src={qrDataUrl} alt="PDKS QR kodu" width={480} height={480} />
        </div>
      ) : (
        <p className="text-beton-400">{err ?? "Yükleniyor…"}</p>
      )}

      {qrDataUrl && (
        <p className="text-beton-400 text-sm">Kod {secondsLeft} saniye içinde yenilenecek — telefonunuzla okutun.</p>
      )}
    </div>
  );
}

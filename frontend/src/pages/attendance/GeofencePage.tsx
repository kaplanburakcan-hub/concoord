import { useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import L from "leaflet";
import "leaflet/dist/leaflet.css";
import markerIcon2x from "leaflet/dist/images/marker-icon-2x.png";
import markerIcon from "leaflet/dist/images/marker-icon.png";
import markerShadow from "leaflet/dist/images/marker-shadow.png";
import { api } from "../../api/client";
import { useProjects } from "../ProjectContext";

// Leaflet'in varsayılan marker ikon URL'leri paket-göreli yollara dayanır ve
// Vite gibi bundler'larda kırılır (bilinen bir Leaflet+bundler sorunu) —
// ikonları burada Vite'ın işlediği modül URL'leriyle elle ayarlıyoruz.
delete (L.Icon.Default.prototype as unknown as { _getIconUrl?: unknown })._getIconUrl;
L.Icon.Default.mergeOptions({ iconRetinaUrl: markerIcon2x, iconUrl: markerIcon, shadowUrl: markerShadow });

type Geofence = {
  id: string; project_id: string; name: string;
  center_lat: number; center_lng: number; radius_m: number; is_active: boolean; created_at: string;
};

const TURKIYE_CENTER: [number, number] = [39.0, 35.0];

export default function GeofencePage() {
  const { current } = useProjects();
  const [list, setList] = useState<Geofence[]>([]);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);

  useEffect(() => {
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [current?.id]);

  async function load() {
    if (!current?.id) return;
    setLoading(true);
    try {
      const res = await api<{ geofences: Geofence[] }>(`/projects/${current.id}/geofences`, { projectId: current.id });
      setList(res.geofences);
    } finally {
      setLoading(false);
    }
  }

  if (!current) return <div className="p-8 text-beton-400">Proje seçilmedi.</div>;

  return (
    <div>
      <div className="flex items-center gap-3">
        <h1 className="font-display text-2xl font-extrabold text-white">Şantiye Sınırları</h1>
        <button
          onClick={() => setShowForm((v) => !v)}
          className="ml-auto rounded-md bg-emniyet-500 hover:bg-emniyet-600 text-beton-950 font-semibold px-3 py-1.5 text-sm transition"
        >
          {showForm ? "Kapat" : "+ Yeni Sınır"}
        </button>
      </div>
      <p className="mt-1 text-sm text-beton-400 max-w-2xl">
        PDKS/GPS puantaj için giriş-çıkış kontrolü yapılacak şantiye sınırlarını tanımlayın. Bu sınırın dışından
        yapılan kayıtlar reddedilmez, yalnızca şef onayına düşecek şekilde işaretlenir.
      </p>

      {showForm && (
        <NewGeofenceForm projectId={current.id} onCreated={() => { setShowForm(false); load(); }} />
      )}

      <div className="mt-4 flex flex-col gap-2.5">
        {loading ? (
          <p className="text-beton-400 text-sm px-4 py-6 text-center">Yükleniyor…</p>
        ) : list.length === 0 ? (
          <p className="text-beton-400 text-sm px-4 py-6 text-center">Henüz şantiye sınırı tanımlanmamış.</p>
        ) : (
          list.map((g) => (
            <div key={g.id} className="rounded-lg border border-beton-800 bg-beton-900 px-4 py-3 flex items-center gap-4">
              <div className="flex-1 min-w-0">
                <p className="text-beton-100 font-medium truncate">{g.name}</p>
                <p className="text-xs text-beton-400 font-mono">
                  {g.center_lat.toFixed(5)}, {g.center_lng.toFixed(5)} · {g.radius_m} m yarıçap
                </p>
              </div>
              <span
                className={`text-xs font-medium px-2 py-0.5 rounded-full border shrink-0 ${
                  g.is_active ? "bg-green-500/15 text-green-300 border-green-500/40" : "bg-beton-800 text-beton-400 border-beton-700"
                }`}
              >
                {g.is_active ? "Aktif" : "Pasif"}
              </span>
              <Link
                to={`/proje/pdks-pano?geofence=${g.id}`}
                className="text-xs font-semibold shrink-0 whitespace-nowrap hover:underline"
                style={{ color: "var(--group-accent)" }}
              >
                QR Pano Aç →
              </Link>
            </div>
          ))
        )}
      </div>
    </div>
  );
}

function NewGeofenceForm({ projectId, onCreated }: { projectId: string; onCreated: () => void }) {
  const [name, setName] = useState("");
  const [radius, setRadius] = useState(300);
  const [center, setCenter] = useState<[number, number] | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const mapRef = useRef<HTMLDivElement>(null);
  const mapInstance = useRef<L.Map | null>(null);
  const markerInstance = useRef<L.Marker | null>(null);

  useEffect(() => {
    if (!mapRef.current || mapInstance.current) return;
    const map = L.map(mapRef.current).setView(TURKIYE_CENTER, 6);
    L.tileLayer("https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png", {
      attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> katkıda bulunanlar',
      maxZoom: 19,
    }).addTo(map);
    map.on("click", (e: L.LeafletMouseEvent) => {
      const { lat, lng } = e.latlng;
      setCenter([lat, lng]);
      if (markerInstance.current) markerInstance.current.setLatLng([lat, lng]);
      else markerInstance.current = L.marker([lat, lng]).addTo(map);
    });
    mapInstance.current = map;
    // Yalnızca haritayı yönetenin konumuna yaklaştırmak için — bir kayıt
    // OLUŞTURMAZ, salt bir başlangıç görünümü kolaylığıdır.
    if (navigator.geolocation) {
      navigator.geolocation.getCurrentPosition(
        (pos) => map.setView([pos.coords.latitude, pos.coords.longitude], 16),
        () => {},
        { timeout: 5000 }
      );
    }
    return () => {
      map.remove();
      mapInstance.current = null;
    };
  }, []);

  async function submit() {
    setErr(null);
    if (!name.trim()) {
      setErr("Sınır adı zorunlu.");
      return;
    }
    if (!center) {
      setErr("Haritada bir merkez seçin.");
      return;
    }
    setBusy(true);
    try {
      await api(`/projects/${projectId}/geofences`, {
        method: "POST", projectId,
        body: { name: name.trim(), center_lat: center[0], center_lng: center[1], radius_m: radius },
      });
      onCreated();
    } catch {
      setErr("Kaydedilemedi. Tekrar deneyin.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="mt-4 rounded-lg border border-beton-800 bg-beton-900 p-4">
      <div className="grid gap-3 sm:grid-cols-2 mb-3">
        <div>
          <label className="block text-xs text-beton-400 mb-1">Sınır Adı</label>
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="Örn: Ana Giriş"
            className="w-full rounded-md bg-beton-950 border border-beton-800 px-3 py-1.5 text-sm text-beton-200 outline-none focus:border-emniyet-500"
          />
        </div>
        <div>
          <label className="block text-xs text-beton-400 mb-1">Yarıçap (metre)</label>
          <input
            type="number" min={50} max={2000}
            value={radius}
            onChange={(e) => setRadius(Number(e.target.value))}
            className="w-full rounded-md bg-beton-950 border border-beton-800 px-3 py-1.5 text-sm text-beton-200 outline-none focus:border-emniyet-500"
          />
        </div>
      </div>
      <p className="text-xs text-beton-400 mb-2">Haritada şantiye merkezine tıklayın.</p>
      <div ref={mapRef} className="w-full h-72 rounded-lg overflow-hidden border border-beton-800" />
      {center && (
        <p className="mt-2 text-xs font-mono text-beton-400">
          Seçilen merkez: {center[0].toFixed(5)}, {center[1].toFixed(5)}
        </p>
      )}
      {err && <p className="mt-2 text-sm text-red-400">{err}</p>}
      <button
        onClick={submit}
        disabled={busy}
        className="mt-3 rounded-md bg-emniyet-500 hover:bg-emniyet-600 disabled:opacity-60 text-beton-950 font-semibold px-4 py-1.5 text-sm transition"
      >
        {busy ? "Kaydediliyor…" : "Sınırı Kaydet"}
      </button>
    </div>
  );
}

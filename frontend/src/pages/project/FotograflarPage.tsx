import { useCallback, useEffect, useRef, useState } from "react";
import { api, apiFetchBlob, apiUpload } from "../../api/client";
import { useAuth } from "../../auth/AuthContext";
import { useProjects } from "../../projects/ProjectContext";

// Fotoğraflar — önceden ComingSoon placeholder'ıydı. Yeni bir tablo yerine
// mevcut polimorfik documents motoru kullanılıyor: her fotoğraf bir
// documents satırı (entity_type="project_photo", entity_id=<project_id>),
// tip ayrımı (Saha/İmalat/Denetim) doc_category üzerinden yapılıyor.

type Tip = "SahaFotografi" | "ImalatFotografi" | "DenetimFotografi";

const TIP_LABEL: Record<Tip, string> = {
  SahaFotografi: "Saha",
  ImalatFotografi: "İmalat",
  DenetimFotografi: "Denetim",
};

const TIP_ICON: Record<Tip, string> = {
  SahaFotografi: "🏗️",
  ImalatFotografi: "🔨",
  DenetimFotografi: "🔍",
};

type DocItem = {
  id: string;
  title: string;
  doc_category: string;
  latest_version?: number;
  created_at: string;
};

type Foto = { id: string; ad: string; tip: string; url: string; createdAt: string };

export default function FotograflarPage() {
  const { current } = useProjects();
  const { can } = useAuth();
  const pid = current?.id;

  const [fotograflar, setFotograflar] = useState<Foto[]>([]);
  const [yukleniyor, setYukleniyor] = useState(true);
  const [filtre, setFiltre] = useState<Tip | "hepsi">("hepsi");
  const [err, setErr] = useState<string | null>(null);
  const [buyukFoto, setBuyukFoto] = useState<Foto | null>(null);

  const [modalAcik, setModalAcik] = useState(false);
  const [modalTip, setModalTip] = useState<Tip>("SahaFotografi");
  const [modalAciklama, setModalAciklama] = useState("");
  const [modalYukleniyor, setModalYukleniyor] = useState(false);
  const fileRef = useRef<HTMLInputElement>(null);

  const canUpload = can("documents.upload");
  const canDelete = can("documents.delete");

  const load = useCallback(async () => {
    if (!pid) return;
    setYukleniyor(true);
    setErr(null);
    try {
      const r = await api<{ documents: DocItem[] }>(
        `/projects/${pid}/documents?entity_type=project_photo&entity_id=${pid}`, { projectId: pid });
      const docs = (r.documents ?? []).filter((d) => d.latest_version);
      const withUrls = await Promise.all(
        docs.map(async (d) => {
          const url = await apiFetchBlob(`/projects/${pid}/documents/${d.id}/versions/${d.latest_version}/download`);
          return { id: d.id, ad: d.title, tip: d.doc_category, url, createdAt: d.created_at } as Foto;
        })
      );
      setFotograflar(withUrls);
    } catch {
      setErr("Fotoğraflar yüklenemedi.");
    } finally {
      setYukleniyor(false);
    }
  }, [pid]);

  useEffect(() => { load(); }, [load]);

  async function yukle(files: FileList | null) {
    if (!files || !pid) return;
    setModalYukleniyor(true);
    try {
      for (const file of Array.from(files)) {
        if (file.size > 10 * 1024 * 1024) { alert(`${file.name} 10MB sınırını aşıyor.`); continue; }
        const doc = await api<{ document: { id: string } }>(`/projects/${pid}/documents`, {
          method: "POST", projectId: pid,
          body: {
            title: modalAciklama.trim() || file.name,
            doc_category: modalTip,
            entity_type: "project_photo",
            entity_id: pid,
          },
        });
        const fd = new FormData();
        fd.append("file", file);
        await apiUpload(`/projects/${pid}/documents/${doc.document.id}/versions`, fd);
      }
      setModalAcik(false);
      setModalAciklama("");
      await load();
    } catch {
      setErr("Fotoğraf yüklenemedi.");
    } finally {
      setModalYukleniyor(false);
      if (fileRef.current) fileRef.current.value = "";
    }
  }

  async function sil(f: Foto) {
    if (!pid || !confirm("Bu fotoğrafı silmek istediğinize emin misiniz?")) return;
    try {
      await api(`/projects/${pid}/documents/${f.id}`, { method: "DELETE", projectId: pid });
      setBuyukFoto(null);
      await load();
    } catch {
      setErr("Fotoğraf silinemedi.");
    }
  }

  if (!current) return <p className="text-beton-400 text-sm">Önce üst bardan bir proje seçin.</p>;

  const filtreliler = filtre === "hepsi" ? fotograflar : fotograflar.filter((f) => f.tip === filtre);

  return (
    <div className="max-w-6xl mx-auto space-y-4">
      <div className="flex items-center justify-between gap-3 flex-wrap">
        <div>
          <h1 className="font-display font-extrabold text-xl text-white">Fotoğraflar</h1>
          <p className="text-sm text-beton-400 mt-1">{current.name} — proje sahası, imalat ve denetim fotoğrafları.</p>
        </div>
        {canUpload && (
          <button
            onClick={() => setModalAcik(true)}
            className="rounded-md bg-emniyet-500 px-3 py-2 text-sm font-medium text-beton-950 hover:brightness-110"
          >
            + Fotoğraf Yükle
          </button>
        )}
      </div>
      {err && <p className="text-sm text-red-400">{err}</p>}

      {/* Tip filtre pilleri */}
      <div className="flex gap-2 flex-wrap">
        {(["hepsi", "SahaFotografi", "ImalatFotografi", "DenetimFotografi"] as const).map((tip) => (
          <button key={tip} onClick={() => setFiltre(tip)}
            className={`rounded-full px-3 py-1 text-xs border transition ${
              filtre === tip ? "bg-emniyet-500 text-beton-950 border-emniyet-500 font-semibold"
              : "border-beton-700 text-beton-400 hover:border-beton-500"
            }`}
          >
            {tip === "hepsi" ? "Tümü" : `${TIP_ICON[tip as Tip]} ${TIP_LABEL[tip as Tip]}`}
          </button>
        ))}
      </div>

      {/* Izgara */}
      {yukleniyor ? (
        <p className="text-beton-400 text-sm">Yükleniyor…</p>
      ) : filtreliler.length === 0 ? (
        <p className="text-beton-400 text-sm">Henüz fotoğraf yüklenmemiş.</p>
      ) : (
        <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-6 gap-3">
          {filtreliler.map((f) => (
            <button key={f.id} onClick={() => setBuyukFoto(f)}
              className="relative rounded-md overflow-hidden border border-beton-800 hover:border-emniyet-500 group"
            >
              <img src={f.url} alt={f.ad} className="w-full h-28 object-cover" />
              <span className="absolute top-1 left-1 rounded bg-black/60 text-white text-[10px] px-1.5 py-0.5">
                {TIP_ICON[f.tip as Tip] ?? "📷"}
              </span>
            </button>
          ))}
        </div>
      )}

      {/* Yükleme Modalı */}
      {modalAcik && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60" onClick={() => setModalAcik(false)}>
          <div
            className="bg-beton-900 border border-beton-700 rounded-xl w-full max-w-md mx-4 p-6 space-y-4"
            onClick={(e) => e.stopPropagation()}
          >
            <h2 className="font-display font-bold text-white text-lg">Fotoğraf Yükle</h2>

            <div>
              <label className="block text-xs text-beton-400 mb-1">Tip</label>
              <div className="flex gap-2">
                {(["SahaFotografi", "ImalatFotografi", "DenetimFotografi"] as const).map((tip) => (
                  <button key={tip} onClick={() => setModalTip(tip)}
                    className={`flex-1 rounded-md border py-2 text-xs font-medium transition ${
                      modalTip === tip
                        ? "border-emniyet-500 bg-emniyet-500/10 text-emniyet-500"
                        : "border-beton-700 text-beton-400 hover:border-beton-500"
                    }`}
                  >
                    {TIP_ICON[tip]} {TIP_LABEL[tip]}
                  </button>
                ))}
              </div>
            </div>

            <div>
              <label className="block text-xs text-beton-400 mb-1">Açıklama (opsiyonel)</label>
              <input
                value={modalAciklama}
                onChange={(e) => setModalAciklama(e.target.value)}
                placeholder="Boş bırakılırsa dosya adı kullanılır"
                className="w-full rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500"
              />
            </div>

            <input
              ref={fileRef}
              type="file"
              accept="image/*"
              multiple
              onChange={(e) => yukle(e.target.files)}
              disabled={modalYukleniyor}
              className="w-full text-sm text-beton-300 file:mr-3 file:rounded-md file:border-0 file:bg-beton-800 file:px-3 file:py-1.5 file:text-beton-100 file:text-xs hover:file:bg-beton-700"
            />

            <div className="flex justify-end gap-3 pt-2">
              <button
                onClick={() => setModalAcik(false)}
                disabled={modalYukleniyor}
                className="rounded-md border border-beton-700 px-4 py-2 text-sm text-beton-300 hover:border-beton-500 disabled:opacity-50"
              >
                {modalYukleniyor ? "Yükleniyor…" : "Kapat"}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Büyük Fotoğraf Modalı */}
      {buyukFoto && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80"
          onClick={() => setBuyukFoto(null)}
        >
          <div className="relative max-w-3xl w-full mx-4" onClick={(e) => e.stopPropagation()}>
            <img src={buyukFoto.url} alt={buyukFoto.ad}
              className="w-full max-h-[80vh] object-contain rounded-lg"
            />
            <div className="flex items-center justify-between mt-2">
              <p className="text-xs text-beton-400">
                {TIP_ICON[buyukFoto.tip as Tip] ?? "📷"} {buyukFoto.ad}
              </p>
              {canDelete && (
                <button onClick={() => sil(buyukFoto)} className="text-xs text-red-400 hover:underline">
                  Sil
                </button>
              )}
            </div>
            <button onClick={() => setBuyukFoto(null)}
              className="absolute top-2 right-2 bg-black/60 text-white rounded-full w-8 h-8 flex items-center justify-center hover:bg-black"
            >✕</button>
          </div>
        </div>
      )}
    </div>
  );
}

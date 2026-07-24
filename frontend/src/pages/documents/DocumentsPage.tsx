import { useCallback, useEffect, useRef, useState } from "react";
import type { ReactNode } from "react";
import { api, apiUpload, apiDownload } from "../../api/client";
import { useAuth } from "../../auth/AuthContext";
import { useProjects } from "../ProjectContext";

type Folder = { id: string; parent_id?: string; name: string; module_scope?: string };
type DocVersion = {
  id: string; version_no: number; original_name: string; mime: string;
  size_bytes: number; sha256: string; note?: string; created_at: string;
};
type Doc = {
  id: string; folder_id?: string; title: string; doc_category: string;
  version_count: number; latest_version?: number; updated_at: string;
};

const CATS = ["Contract", "Addendum", "Submittal", "Drawing", "Delivery", "OHS", "Other"];
const CAT_LABEL: Record<string, string> = {
  Contract: "Sözleşme", Addendum: "Zeyilname", Submittal: "Malzeme", Drawing: "Çizim",
  Delivery: "İrsaliye", OHS: "İSG", Other: "Diğer",
};

export default function DocumentsPage() {
  const { current } = useProjects();
  const { can } = useAuth();
  const [folders, setFolders] = useState<Folder[]>([]);
  const [docs, setDocs] = useState<Doc[]>([]);
  const [sel, setSel] = useState<string | null>(null); // seçili klasör (null = tümü)
  const [catFilter, setCatFilter] = useState<string | null>(null); // kategori filtresi
  const [err, setErr] = useState<string | null>(null);

  const pid = current?.id;

  const load = useCallback(async (cat: string | null, folder: string | null) => {
    if (!pid) return;
    setErr(null);
    try {
      const f = await api<{ folders: Folder[] }>(`/projects/${pid}/folders`, { projectId: pid });
      setFolders(f.folders);
      const params = new URLSearchParams();
      if (folder) params.set("folder_id", folder);
      if (cat) params.set("category", cat);
      const query = params.toString() ? `?${params}` : "";
      const d = await api<{ documents: Doc[] }>(`/projects/${pid}/documents${query}`, { projectId: pid });
      setDocs(d.documents);
    } catch {
      setErr("Dokümanlar yüklenemedi ya da erişim yetkiniz yok.");
    }
  }, [pid]);

  useEffect(() => { load(catFilter, sel); }, [load, catFilter, sel]);

  if (!current) {
    return <p className="text-beton-400">Önce üst bardan bir proje seçin veya oluşturun.</p>;
  }

  return (
    <div>
      <h1 className="font-display text-2xl font-extrabold text-white">Dokümanlar</h1>
      <p className="text-sm text-beton-400 mt-1">{current.name} — sözleşme, zeyilname ve tüm proje belgeleri (versiyonlu).</p>
      {err && <p className="mt-3 text-sm text-red-400">{err}</p>}

      <div className="mt-4 grid gap-4 md:grid-cols-[240px_1fr]">
        <FolderPanel
          projectId={pid!}
          folders={folders}
          selected={sel}
          onSelect={(id) => {
            setSel(id);
            setCatFilter(null);
            load(null, id);
          }}
          catFilter={catFilter}
          onCatFilter={(c) => {
            setCatFilter(c);
            setSel(null);
            load(c, null); // state asenkron güncellenir; güncel değeri direkt geç
          }}
          canManage={can("documents.manage_folders")}
          onChanged={() => load(catFilter, sel)}
        />
        <DocPanel
          projectId={pid!}
          folderId={sel}
          docs={docs}
          catFilter={catFilter}
          canUpload={can("documents.upload")}
          canDelete={can("documents.delete")}
          canDownload={can("documents.download")}
          onChanged={() => load(catFilter, sel)}
        />
      </div>
    </div>
  );
}

function FolderPanel({ projectId, folders, selected, onSelect, catFilter, onCatFilter, canManage, onChanged }: {
  projectId: string; folders: Folder[]; selected: string | null;
  onSelect: (id: string | null) => void;
  catFilter: string | null; onCatFilter: (c: string | null) => void;
  canManage: boolean; onChanged: () => void;
}) {
  const [name, setName] = useState("");
  const roots = folders.filter((f) => !f.parent_id);
  const childrenOf = (id: string) => folders.filter((f) => f.parent_id === id);

  async function addFolder() {
    if (!name.trim()) return;
    await api(`/projects/${projectId}/folders`, {
      method: "POST", projectId, body: { name: name.trim(), parent_id: selected },
    });
    setName("");
    onChanged();
  }

  function render(list: Folder[], depth: number): ReactNode {
    return list.map((f) => (
      <div key={f.id}>
        <button
          onClick={() => onSelect(f.id)}
          style={{ paddingLeft: 8 + depth * 14 }}
          className={
            "w-full text-left px-2 py-1 rounded text-sm truncate " +
            (selected === f.id ? "bg-beton-800 text-white" : "text-beton-300 hover:bg-beton-900")
          }
        >
          <span className="text-emniyet-500 mr-1">▸</span>{f.name}
        </button>
        {render(childrenOf(f.id), depth + 1)}
      </div>
    ));
  }

  return (
    <div className="border border-beton-800 rounded-lg bg-beton-900 p-2 h-fit">
      <button
        onClick={() => onSelect(null)}
        className={"w-full text-left px-2 py-1 rounded text-sm " + (selected === null ? "bg-beton-800 text-white" : "text-beton-300 hover:bg-beton-900")}
      >
        Tüm dokümanlar
      </button>
      {/* Kategoriler sabit olarak sol panelde — dropdown yerine doğrudan tıklanabilir */}
      <div className="mt-2 border-t border-beton-800 pt-2">
        <p className="text-[10px] uppercase tracking-wider text-beton-500 px-2 mb-1">Kategoriler</p>
        {CATS.map((c) => (
          <button key={c}
            onClick={() => onCatFilter(catFilter === c ? null : c)}
            className={"w-full text-left px-2 py-1 rounded text-sm truncate " +
              (catFilter === c ? "bg-emniyet-500/20 text-emniyet-500" : "text-beton-300 hover:bg-beton-900")}>
            {CAT_LABEL[c]}
          </button>
        ))}
      </div>
      <div className="mt-2 border-t border-beton-800 pt-2">
        <p className="text-[10px] uppercase tracking-wider text-beton-500 px-2 mb-1">Klasörler</p>
        <div>{render(roots, 0)}</div>
      </div>
      {canManage && (
        <div className="mt-3 border-t border-beton-800 pt-2">
          <label className="block text-xs text-beton-400 mb-1">
            Klasör ekle {selected ? "(seçili altına)" : "(kök)"}
          </label>
          <div className="flex gap-1">
            <input
              value={name}
              onChange={(e) => setName(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && addFolder()}
              placeholder="Ad"
              className="flex-1 min-w-0 rounded bg-beton-950 border border-beton-800 px-2 py-1 text-xs text-beton-200 outline-none focus:border-emniyet-500"
            />
            <button onClick={addFolder} className="rounded bg-emniyet-500 hover:bg-emniyet-600 text-beton-950 font-semibold px-2 text-xs">+</button>
          </div>
        </div>
      )}
    </div>
  );
}

function DocPanel({ projectId, folderId, docs, catFilter, canUpload, canDelete, canDownload, onChanged }: {
  catFilter?: string | null;
  projectId: string; folderId: string | null; docs: Doc[];
  canUpload: boolean; canDelete: boolean; canDownload: boolean; onChanged: () => void;
}) {
  const [showNew, setShowNew] = useState(false);
  const [nf, setNf] = useState({ title: "", doc_category: catFilter ?? "Contract" });
  // Kategori filtresi değişince yeni doküman formunu güncelle
  useEffect(() => {
    setNf(f => ({ ...f, doc_category: catFilter ?? "Contract" }));
  }, [catFilter]);
  const [openId, setOpenId] = useState<string | null>(null);

  async function createDoc() {
    if (!nf.title.trim()) return;
    await api(`/projects/${projectId}/documents`, {
      method: "POST", projectId,
      body: { title: nf.title.trim(), doc_category: nf.doc_category, folder_id: folderId },
    });
    setNf({ title: "", doc_category: "Contract" });
    setShowNew(false);
    onChanged();
  }

  async function removeDoc(d: Doc) {
    await api(`/projects/${projectId}/documents/${d.id}`, { method: "DELETE", projectId });
    onChanged();
  }

  return (
    <div>
      {canUpload && (
        <div className="flex">
          <button
            onClick={() => setShowNew((v) => !v)}
            className="ml-auto rounded-md bg-emniyet-500 hover:bg-emniyet-600 text-beton-950 font-semibold px-3 py-1.5 text-sm"
          >
            {showNew ? "Kapat" : "Yeni doküman"}
          </button>
        </div>
      )}
      {showNew && (
        <div className="mt-3 rounded-lg border border-beton-800 bg-beton-900 p-3 flex flex-wrap gap-2 items-end">
          <div className="flex-1 min-w-[180px]">
            <label className="block text-xs text-beton-400 mb-1">Başlık</label>
            <input value={nf.title} onChange={(e) => setNf({ ...nf, title: e.target.value })}
              className="w-full rounded-md bg-beton-950 border border-beton-800 px-3 py-1.5 text-sm text-beton-200 outline-none focus:border-emniyet-500" />
          </div>
          <div>
            <label className="block text-xs text-beton-400 mb-1">Kategori</label>
            <select value={nf.doc_category} onChange={(e) => setNf({ ...nf, doc_category: e.target.value })}
              className="rounded-md bg-beton-950 border border-beton-800 px-3 py-1.5 text-sm text-beton-200 outline-none focus:border-emniyet-500">
              {CATS.map((c) => <option key={c} value={c}>{CAT_LABEL[c]}</option>)}
            </select>
          </div>
          <button onClick={createDoc} className="rounded-md bg-emniyet-500 hover:bg-emniyet-600 text-beton-950 font-semibold px-4 py-1.5 text-sm">Oluştur</button>
        </div>
      )}

      <div className="mt-3 border border-beton-800 rounded-lg divide-y divide-beton-800">
        {docs.length === 0 ? (
          <p className="px-4 py-6 text-center text-beton-400 text-sm">Bu kapsamda doküman yok.</p>
        ) : (
          docs.map((d) => (
            <DocRow
              key={d.id}
              projectId={projectId}
              doc={d}
              open={openId === d.id}
              onToggle={() => setOpenId(openId === d.id ? null : d.id)}
              canUpload={canUpload}
              canDelete={canDelete}
              canDownload={canDownload}
              onDelete={() => removeDoc(d)}
              onChanged={onChanged}
            />
          ))
        )}
      </div>
    </div>
  );
}

function DocRow({ projectId, doc, open, onToggle, canUpload, canDelete, canDownload, onDelete, onChanged }: {
  projectId: string; doc: Doc; open: boolean; onToggle: () => void;
  canUpload: boolean; canDelete: boolean; canDownload: boolean; onDelete: () => void; onChanged: () => void;
}) {
  const [versions, setVersions] = useState<DocVersion[]>([]);
  const [busy, setBusy] = useState(false);
  const [note, setNote] = useState("");
  const fileRef = useRef<HTMLInputElement>(null);

  const loadVersions = useCallback(async () => {
    const res = await api<{ versions: DocVersion[] }>(`/projects/${projectId}/documents/${doc.id}`, { projectId });
    setVersions(res.versions);
  }, [projectId, doc.id]);

  useEffect(() => { if (open) loadVersions(); }, [open, loadVersions]);

  async function upload() {
    const file = fileRef.current?.files?.[0];
    if (!file) return;
    setBusy(true);
    try {
      const fd = new FormData();
      fd.append("file", file);
      if (note.trim()) fd.append("note", note.trim());
      await apiUpload(`/projects/${projectId}/documents/${doc.id}/versions`, fd);
      setNote("");
      if (fileRef.current) fileRef.current.value = "";
      await loadVersions();
      onChanged();
    } finally {
      setBusy(false);
    }
  }

  async function download(v: DocVersion) {
    await apiDownload(
      `/projects/${projectId}/documents/${doc.id}/versions/${v.version_no}/download`,
      v.original_name
    );
  }

  return (
    <div>
      <div className="flex items-center gap-3 px-4 py-2">
        <button onClick={onToggle} className="text-beton-200 text-sm flex-1 text-left truncate">
          <span className="text-emniyet-500 mr-2">{open ? "▾" : "▸"}</span>
          {doc.title}
        </button>
        <span className="font-mono text-xs px-2 py-0.5 rounded bg-beton-800 text-beton-300">{CAT_LABEL[doc.doc_category] ?? doc.doc_category}</span>
        <span className="font-mono text-xs text-beton-400">{doc.latest_version ? `v${doc.latest_version}` : "boş"}</span>
        {canDelete && <button onClick={onDelete} className="text-beton-400 hover:text-red-400 text-xs">sil</button>}
      </div>

      {open && (
        <div className="px-4 pb-3 bg-beton-900/40">
          {versions.length === 0 ? (
            <p className="text-xs text-beton-400 py-2">Henüz versiyon yok — ilk dosyayı yükleyin.</p>
          ) : (
            <table className="w-full text-xs mt-1">
              <tbody>
                {versions.map((v) => (
                  <tr key={v.id} className="border-t border-beton-800/60">
                    <td className="py-1 font-mono text-emniyet-500 w-10">v{v.version_no}</td>
                    <td className="py-1 text-beton-200 truncate">{v.original_name}</td>
                    <td className="py-1 text-beton-400 w-24">{(v.size_bytes / 1024).toFixed(0)} KB</td>
                    <td className="py-1 font-mono text-beton-500 w-28 truncate" title={v.sha256}>{v.sha256.slice(0, 10)}…</td>
                    <td className="py-1 w-20 text-right">
                      {canDownload && <button onClick={() => download(v)} className="text-emniyet-500 hover:underline">indir</button>}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}

          {canUpload && (
            <div className="mt-2 flex flex-wrap items-center gap-2">
              <input ref={fileRef} type="file" className="text-xs text-beton-300 file:mr-2 file:rounded file:border-0 file:bg-beton-800 file:px-2 file:py-1 file:text-beton-200" />
              <input value={note} onChange={(e) => setNote(e.target.value)} placeholder="Not (ops.)"
                className="rounded bg-beton-950 border border-beton-800 px-2 py-1 text-xs text-beton-200 outline-none focus:border-emniyet-500" />
              <button onClick={upload} disabled={busy}
                className="rounded bg-emniyet-500 hover:bg-emniyet-600 disabled:opacity-60 text-beton-950 font-semibold px-3 py-1 text-xs">
                {busy ? "Yükleniyor…" : "Yeni versiyon yükle"}
              </button>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

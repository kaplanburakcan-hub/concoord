import type { ReactNode } from "react";
import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../../api/client";
import type { User } from "../../api/client";

type UserRow = User & { row_version: number; created_at: string };

export default function UsersPage() {
  const [users, setUsers] = useState<UserRow[]>([]);
  const [q, setQ] = useState("");
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState<string | null>(null);
  const [showNew, setShowNew] = useState(false);

  async function load() {
    setLoading(true);
    setErr(null);
    try {
      const res = await api<{ users: UserRow[] }>(`/admin/users${q ? `?query=${encodeURIComponent(q)}` : ""}`);
      setUsers(res.users);
    } catch {
      setErr("Kullanıcılar yüklenemedi.");
    } finally {
      setLoading(false);
    }
  }
  useEffect(() => {
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function toggleActive(u: UserRow) {
    await api(`/admin/users/${u.id}`, {
      method: "PATCH",
      body: { is_active: !u.is_active, row_version: u.row_version },
    });
    load();
  }

  return (
    <div>
      <div className="flex items-center gap-3">
        <h1 className="font-display text-2xl font-extrabold text-white">Kullanıcılar</h1>
        <button
          onClick={() => setShowNew((v) => !v)}
          className="ml-auto rounded-md bg-emniyet-500 hover:bg-emniyet-600 text-beton-950 font-semibold px-3 py-1.5 text-sm transition"
        >
          {showNew ? "Kapat" : "Yeni kullanıcı"}
        </button>
      </div>

      {showNew && <NewUserForm onCreated={() => { setShowNew(false); load(); }} />}

      <div className="mt-4 flex gap-2">
        <input
          value={q}
          onChange={(e) => setQ(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && load()}
          placeholder="Ara: e-posta, kullanıcı adı, ad"
          className="w-72 rounded-md bg-beton-900 border border-beton-800 px-3 py-1.5 text-sm text-beton-200 outline-none focus:border-emniyet-500"
        />
        <button onClick={load} className="rounded-md border border-beton-800 px-3 py-1.5 text-sm text-beton-200 hover:border-emniyet-500">
          Ara
        </button>
      </div>

      {err && <p className="mt-3 text-sm text-red-400">{err}</p>}

      <div className="mt-4 border border-beton-800 rounded-lg overflow-hidden">
        <table className="w-full text-sm">
          <thead className="bg-beton-900 text-beton-400">
            <tr>
              <Th>Ad</Th>
              <Th>E-posta</Th>
              <Th>Kullanıcı adı</Th>
              <Th>Durum</Th>
              <Th>İzinler</Th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr><td colSpan={5} className="px-4 py-6 text-center text-beton-400">Yükleniyor…</td></tr>
            ) : users.length === 0 ? (
              <tr><td colSpan={5} className="px-4 py-6 text-center text-beton-400">Kayıt yok.</td></tr>
            ) : (
              users.map((u) => (
                <tr key={u.id} className="border-t border-beton-800">
                  <Td className="text-beton-200">{u.full_name}</Td>
                  <Td>{u.email}</Td>
                  <Td className="font-mono text-xs">{u.username}</Td>
                  <Td>
                    <button
                      onClick={() => toggleActive(u)}
                      className={
                        "font-mono text-xs px-2 py-0.5 rounded " +
                        (u.is_active ? "bg-emniyet-500/15 text-emniyet-500" : "bg-beton-800 text-beton-400")
                      }
                    >
                      {u.is_active ? "aktif" : "pasif"}
                    </button>
                  </Td>
                  <Td>
                    <Link to={`/admin/permissions?user=${u.id}`} className="text-emniyet-500 hover:underline">
                      matris →
                    </Link>
                  </Td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function NewUserForm({ onCreated }: { onCreated: () => void }) {
  const [f, setF] = useState({ full_name: "", email: "", username: "", password: "" });
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function submit() {
    setErr(null);
    setBusy(true);
    try {
      await api("/admin/users", { method: "POST", body: f });
      onCreated();
    } catch (e: unknown) {
      setErr("Oluşturulamadı — alanları ve benzersizliği kontrol edin.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="mt-4 rounded-lg border border-beton-800 bg-beton-900 p-4 grid gap-3 sm:grid-cols-2">
      {(["full_name", "email", "username", "password"] as const).map((k) => (
        <div key={k}>
          <label className="block text-xs text-beton-400 mb-1">
            {{ full_name: "Ad Soyad", email: "E-posta", username: "Kullanıcı adı", password: "Parola (≥8)" }[k]}
          </label>
          <input
            type={k === "password" ? "password" : "text"}
            value={f[k]}
            onChange={(e) => setF({ ...f, [k]: e.target.value })}
            className="w-full rounded-md bg-beton-950 border border-beton-800 px-3 py-1.5 text-sm text-beton-200 outline-none focus:border-emniyet-500"
          />
        </div>
      ))}
      {err && <p className="sm:col-span-2 text-sm text-red-400">{err}</p>}
      <div className="sm:col-span-2">
        <button
          onClick={submit}
          disabled={busy}
          className="rounded-md bg-emniyet-500 hover:bg-emniyet-600 disabled:opacity-60 text-beton-950 font-semibold px-4 py-1.5 text-sm transition"
        >
          {busy ? "Kaydediliyor…" : "Oluştur"}
        </button>
      </div>
    </div>
  );
}

function Th({ children }: { children: ReactNode }) {
  return <th className="text-left font-medium px-4 py-2">{children}</th>;
}
function Td({ children, className = "" }: { children: ReactNode; className?: string }) {
  return <td className={"px-4 py-2 " + className}>{children}</td>;
}

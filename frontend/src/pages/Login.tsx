import type { FormEvent } from "react";
import { useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";
import { RequestError } from "../api/client";

export default function Login() {
  const { login } = useAuth();
  const nav = useNavigate();
  const loc = useLocation() as { state?: { from?: string } };
  const [identifier, setIdentifier] = useState("");
  const [password, setPassword] = useState("");
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setErr(null);
    setBusy(true);
    try {
      await login(identifier.trim(), password);
      nav(loc.state?.from || "/", { replace: true });
    } catch (e) {
      if (e instanceof RequestError && e.status === 401)
        setErr("E-posta/kullanıcı adı veya parola hatalı.");
      else setErr("Giriş yapılamadı. Lütfen tekrar deneyin.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="min-h-screen flex flex-col">
      <div
        className="h-2"
        style={{ background: "repeating-linear-gradient(-45deg,#f5b301 0 16px,#16181d 16px 32px)" }}
      />
      <div className="flex-1 flex items-center justify-center p-6">
        <form onSubmit={submit} className="w-full max-w-sm">
          <p className="font-mono text-xs tracking-[0.3em] text-emniyet-500 uppercase">İPKS</p>
          <h1 className="font-display text-3xl font-extrabold text-white mt-2">Giriş</h1>
          <p className="mt-1 text-beton-400 text-sm">
            İnşaat Proje Koordinasyon ve Saha Takip Platformu
          </p>

          <label className="block mt-8 text-sm text-beton-200">E-posta veya kullanıcı adı</label>
          <input
            autoFocus
            value={identifier}
            onChange={(e) => setIdentifier(e.target.value)}
            className="mt-1 w-full rounded-md bg-beton-900 border border-beton-800 px-3 py-2 text-beton-200 outline-none focus:border-emniyet-500"
          />

          <label className="block mt-4 text-sm text-beton-200">Parola</label>
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            className="mt-1 w-full rounded-md bg-beton-900 border border-beton-800 px-3 py-2 text-beton-200 outline-none focus:border-emniyet-500"
          />

          {err && <p className="mt-3 text-sm text-red-400">{err}</p>}

          <button
            type="submit"
            disabled={busy}
            className="mt-6 w-full rounded-md bg-emniyet-500 hover:bg-emniyet-600 disabled:opacity-60 text-beton-950 font-semibold py-2 transition"
          >
            {busy ? "Giriş yapılıyor…" : "Giriş yap"}
          </button>
        </form>
      </div>
    </main>
  );
}

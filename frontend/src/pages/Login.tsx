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
    <main className="min-h-screen app-canvas flex items-center justify-center p-6">
      <form
        onSubmit={submit}
        className="w-full max-w-sm rounded-2xl border border-beton-800 bg-beton-900 p-8"
        style={{ boxShadow: "var(--shadow)" }}
      >
        <div className="flex items-center gap-3">
          <div
            className="w-11 h-11 rounded-xl grid place-items-center text-white font-medium"
            style={{ background: "linear-gradient(135deg,var(--accent),var(--accent-sky))" }}
          >
            İP
          </div>
          <div className="leading-tight">
            <div className="font-display text-lg font-medium text-beton-100">İPKS</div>
            <div className="text-[11px] text-beton-500">Proje kontrol platformu</div>
          </div>
        </div>

        <h1 className="font-display text-2xl font-medium text-beton-100 mt-7 tracking-tight">Giriş</h1>
        <p className="mt-1 text-beton-400 text-sm">Hesabınızla oturum açın.</p>

        <label className="block mt-6 text-sm text-beton-300">E-posta veya kullanıcı adı</label>
        <input
          autoFocus
          value={identifier}
          onChange={(e) => setIdentifier(e.target.value)}
          className="mt-1.5 w-full rounded-lg bg-beton-950 border border-beton-800 px-3 py-2.5 text-beton-100 outline-none focus:border-emniyet-500 transition"
        />

        <label className="block mt-4 text-sm text-beton-300">Parola</label>
        <input
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          className="mt-1.5 w-full rounded-lg bg-beton-950 border border-beton-800 px-3 py-2.5 text-beton-100 outline-none focus:border-emniyet-500 transition"
        />

        {err && <p className="mt-3 text-sm text-red-400">{err}</p>}

        <button
          type="submit"
          disabled={busy}
          className="mt-6 w-full rounded-lg bg-emniyet-500 hover:bg-emniyet-600 disabled:opacity-60 text-beton-950 font-medium py-2.5 transition"
        >
          {busy ? "Giriş yapılıyor…" : "Giriş yap"}
        </button>
      </form>
    </main>
  );
}

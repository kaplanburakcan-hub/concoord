import type { ReactNode } from "react";
import { Navigate, useLocation } from "react-router-dom";
import { useAuth } from "./AuthContext";

// <Can perm="admin.manage_users"> — izin varsa çocukları gösterir.
// İzin yoksa `fallback` (varsayılan: hiçbir şey) render edilir.
export function Can({
  perm,
  children,
  fallback = null,
}: {
  perm: string;
  children: ReactNode;
  fallback?: ReactNode;
}) {
  const { can } = useAuth();
  return <>{can(perm) ? children : fallback}</>;
}

// RequireAuth — oturum yoksa /login'e yönlendirir.
export function RequireAuth({ children }: { children: ReactNode }) {
  const { user, loading } = useAuth();
  const loc = useLocation();
  if (loading) return <FullscreenNote text="Yükleniyor…" />;
  if (!user) return <Navigate to="/login" state={{ from: loc.pathname }} replace />;
  return <>{children}</>;
}

// RequirePerm — belirli bir izni olmayan kullanıcıya 403 ekranı gösterir.
export function RequirePerm({ perm, children }: { perm: string; children: ReactNode }) {
  const { can, loading } = useAuth();
  if (loading) return <FullscreenNote text="Yükleniyor…" />;
  if (!can(perm))
    return (
      <FullscreenNote
        text="Bu sayfa için yetkiniz yok."
        sub={`Gerekli izin: ${perm}`}
      />
    );
  return <>{children}</>;
}

function FullscreenNote({ text, sub }: { text: string; sub?: string }) {
  return (
    <div className="flex-1 flex items-center justify-center p-10 text-center">
      <div>
        <p className="text-beton-200">{text}</p>
        {sub && <p className="mt-1 font-mono text-xs text-beton-400">{sub}</p>}
      </div>
    </div>
  );
}

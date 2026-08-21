import { useEffect, useState } from "react";
import { api } from "../api/client";

// useKesinKabulTarihi — projenin Kesin Kabul tarihini (Yer Teslim + İşin
// Süresi + Geçici Kabul Sonrası gün) çeker; iş/teslimat tarihi giren
// formların <input type="date" max=…> değeri için kullanılır. Sunucu
// tarafı asıl uygulama noktasıdır (bkz. internal/validate.NotAfterKesinKabul)
// — bu sadece istemci tarafında erken geri bildirim verir. Ana sözleşme
// eksikse (ya da alanlardan biri girilmemişse) null döner — o durumda max
// uygulanmaz, kullanıcı haksız yere engellenmez.
export function useKesinKabulTarihi(projectId: string | undefined): string | undefined {
  const [tarih, setTarih] = useState<string | undefined>(undefined);

  useEffect(() => {
    if (!projectId) { setTarih(undefined); return; }
    let cancelled = false;
    api<{ kesin_kabul_tarihi: string | null }>(`/projects/${projectId}/kesin-kabul-tarihi`, { projectId })
      .then((r) => { if (!cancelled) setTarih(r.kesin_kabul_tarihi ?? undefined); })
      .catch(() => { if (!cancelled) setTarih(undefined); });
    return () => { cancelled = true; };
  }, [projectId]);

  return tarih;
}

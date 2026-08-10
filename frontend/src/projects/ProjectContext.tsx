import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import type { ReactNode } from "react";
import { api } from "../api/client";
import { useAuth } from "../auth/AuthContext";

export type Project = {
  id: string;
  code: string;
  name: string;
  location?: string;
  client_name?: string;
  budget_total?: number;
  currency: string;
  start_date?: string;
  end_date?: string;
  status: string;
  accent_color?: string;
  row_version: number;
  created_at: string;
};

// #RRGGBB → "R G B" (Tailwind'in rgb(var(--emniyet-500) / <alpha-value>) deseniyle uyumlu).
function hexToRgbTriplet(hex: string): string | null {
  const m = /^#([0-9a-f]{6})$/i.exec(hex.trim());
  if (!m) return null;
  const n = parseInt(m[1], 16);
  return `${(n >> 16) & 255} ${(n >> 8) & 255} ${n & 255}`;
}
function darken(hex: string, amount: number): string | null {
  const m = /^#([0-9a-f]{6})$/i.exec(hex.trim());
  if (!m) return null;
  const n = parseInt(m[1], 16);
  const r = Math.max(0, Math.round(((n >> 16) & 255) * (1 - amount)));
  const g = Math.max(0, Math.round(((n >> 8) & 255) * (1 - amount)));
  const b = Math.max(0, Math.round((n & 255) * (1 - amount)));
  return `${r} ${g} ${b}`;
}

// Proje vurgu rengini kök CSS değişkenlerine uygular; proje rengi yoksa
// (veya proje seçili değilse) tema varsayılanına dönmesi için satır içi
// override kaldırılır (CSS cascade [data-theme] kuralına düşer).
function applyProjectAccent(color?: string | null) {
  const root = document.documentElement.style;
  const rgb = color ? hexToRgbTriplet(color) : null;
  if (rgb) {
    root.setProperty("--emniyet-500", rgb);
    root.setProperty("--emniyet-600", darken(color!, 0.18) ?? rgb);
    // Sayfa zemini bu değere göre hafif tonlanır (bkz. index.css .app-canvas).
    root.setProperty("--project-accent-rgb", rgb);
  } else {
    root.removeProperty("--emniyet-500");
    root.removeProperty("--emniyet-600");
    root.removeProperty("--project-accent-rgb");
  }
}

type ProjectState = {
  projects: Project[];
  current: Project | null;
  loading: boolean;
  select: (id: string | null) => void;
  reload: () => Promise<void>;
};

const Ctx = createContext<ProjectState | null>(null);
const CUR_KEY = "ipks.project";

// Proje context'i: seçici için proje listesini yükler, seçili projeyi kalıcı
// tutar (localStorage) ve seçim değişince Auth katmanına bildirir → izinler
// o projenin kapsamında yeniden çözümlenir (proje bazlı RBAC).
export function ProjectProvider({ children }: { children: ReactNode }) {
  const { user, setProject } = useAuth();
  const [projects, setProjects] = useState<Project[]>([]);
  const [currentId, setCurrentId] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const reload = useCallback(async () => {
    const res = await api<{ projects: Project[] }>("/projects");
    setProjects(res.projects);
    setCurrentId((prev) => {
      const saved = prev ?? localStorage.getItem(CUR_KEY);
      const valid = res.projects.find((p) => p.id === saved);
      const next = valid ? valid.id : res.projects[0]?.id ?? null;
      if (next) localStorage.setItem(CUR_KEY, next);
      else localStorage.removeItem(CUR_KEY);
      return next;
    });
  }, []);

  useEffect(() => {
    if (!user) {
      setProjects([]);
      setCurrentId(null);
      setLoading(false);
      return;
    }
    (async () => {
      setLoading(true);
      try {
        await reload();
      } catch {
        /* liste yüklenemedi */
      }
      setLoading(false);
    })();
  }, [user, reload]);

  // Seçili proje değişince izinleri o kapsamda yeniden çöz.
  useEffect(() => {
    setProject(currentId);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [currentId]);

  const select = useCallback((id: string | null) => {
    if (id) localStorage.setItem(CUR_KEY, id);
    else localStorage.removeItem(CUR_KEY);
    setCurrentId(id);
  }, []);

  const current = useMemo(
    () => projects.find((p) => p.id === currentId) ?? null,
    [projects, currentId]
  );

  // Aktif projenin vurgu rengini uygula; proje değişince veya renk yoksa
  // tema varsayılanına dön.
  useEffect(() => {
    applyProjectAccent(current?.accent_color);
    return () => applyProjectAccent(null);
  }, [current?.accent_color]);

  const value = useMemo<ProjectState>(
    () => ({ projects, current, loading, select, reload }),
    [projects, current, loading, select, reload]
  );

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useProjects(): ProjectState {
  const ctx = useContext(Ctx);
  if (!ctx) throw new Error("useProjects ProjectProvider içinde kullanılmalı");
  return ctx;
}

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
  row_version: number;
  created_at: string;
};

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

import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import type { ReactNode } from "react";
import {
  fetchMe,
  hasSession,
  login as apiLogin,
  logout as apiLogout,
  setOnSessionLost,
  type User,
} from "../api/client";

type AuthState = {
  user: User | null;
  permissions: Set<string>;
  projectId: string | null;
  loading: boolean;
  can: (perm: string) => boolean;
  login: (identifier: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  setProject: (id: string | null) => void;
  refreshMe: () => Promise<void>;
};

const Ctx = createContext<AuthState | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [permissions, setPermissions] = useState<Set<string>>(new Set());
  const [projectId, setProjectId] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const loadMe = useCallback(async (pid: string | null) => {
    const me = await fetchMe(pid);
    setUser(me.user);
    setPermissions(new Set(me.permissions));
  }, []);

  const refreshMe = useCallback(async () => {
    await loadMe(projectId);
  }, [loadMe, projectId]);

  useEffect(() => {
    setOnSessionLost(() => {
      setUser(null);
      setPermissions(new Set());
    });
  }, []);

  // İlk yükleme: kalıcı refresh token varsa oturumu geri getir.
  useEffect(() => {
    (async () => {
      if (hasSession()) {
        try {
          await loadMe(projectId);
        } catch {
          /* oturum düştü */
        }
      }
      setLoading(false);
    })();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const login = useCallback(
    async (identifier: string, password: string) => {
      const u = await apiLogin(identifier, password);
      setUser(u);
      await loadMe(projectId);
    },
    [loadMe, projectId]
  );

  const logout = useCallback(async () => {
    await apiLogout();
    setUser(null);
    setPermissions(new Set());
  }, []);

  const setProject = useCallback(
    (id: string | null) => {
      setProjectId(id);
      if (user) loadMe(id).catch(() => {});
    },
    [user, loadMe]
  );

  const can = useCallback((perm: string) => permissions.has(perm), [permissions]);

  const value = useMemo<AuthState>(
    () => ({ user, permissions, projectId, loading, can, login, logout, setProject, refreshMe }),
    [user, permissions, projectId, loading, can, login, logout, setProject, refreshMe]
  );

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useAuth(): AuthState {
  const ctx = useContext(Ctx);
  if (!ctx) throw new Error("useAuth AuthProvider içinde kullanılmalı");
  return ctx;
}

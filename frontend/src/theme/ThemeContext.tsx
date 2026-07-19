import { createContext, useContext, useEffect, useState } from "react";
import type { ReactNode } from "react";

// İPKS tema sistemi — light (uçuk gökyüzü) / dark (derin lacivert).
// Tercih localStorage'da ipks-theme altında tutulur; kök <html data-theme>
// index.html'de boyamadan önce ayarlanır (bkz. inline script), burada senkron
// tutulur. Varsayılan: light.

type Theme = "light" | "dark";
type Ctx = { theme: Theme; toggle: () => void; setTheme: (t: Theme) => void };

const ThemeCtx = createContext<Ctx | null>(null);

function initial(): Theme {
  try {
    const t = localStorage.getItem("ipks-theme");
    if (t === "light" || t === "dark") return t;
  } catch {
    /* localStorage kapalı olabilir */
  }
  return "light";
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setThemeState] = useState<Theme>(initial);

  useEffect(() => {
    document.documentElement.setAttribute("data-theme", theme);
    try {
      localStorage.setItem("ipks-theme", theme);
    } catch {
      /* yoksay */
    }
    const meta = document.querySelector('meta[name="theme-color"]');
    if (meta) meta.setAttribute("content", theme === "dark" ? "#000f22" : "#00142e");
  }, [theme]);

  const setTheme = (t: Theme) => setThemeState(t);
  const toggle = () => setThemeState((p) => (p === "dark" ? "light" : "dark"));

  return <ThemeCtx.Provider value={{ theme, toggle, setTheme }}>{children}</ThemeCtx.Provider>;
}

export function useTheme(): Ctx {
  const c = useContext(ThemeCtx);
  if (!c) throw new Error("useTheme ThemeProvider içinde kullanılmalı");
  return c;
}

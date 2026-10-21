import { useCallback, useEffect, useRef, useState } from "react";

// useUndoable — form durumu için geri al / yinele geçmişi.
//
// set(updater, key?): her çağrı geçmişe bir adım ekler. Aynı `key` ile art
// arda (≈1 sn içinde) gelen çağrılar tek adımda birleşir; böylece bir alana
// yazarken her tuş vuruşu ayrı bir geri alma adımı olmaz. Satır silme/ekleme
// gibi ayrık eylemlerde key verilmez, her biri ayrı adımdır.
const COALESCE_MS = 1000;
const MAX_HISTORY = 100;

type Hist<T> = { past: T[]; present: T; future: T[] };

export function useUndoable<T>(initial: T) {
  const [h, setH] = useState<Hist<T>>({ past: [], present: initial, future: [] });
  const last = useRef<{ key?: string; at: number }>({ at: 0 });

  const set = useCallback((next: T | ((p: T) => T), key?: string) => {
    const now = Date.now();
    const merge = !!key && last.current.key === key && now - last.current.at < COALESCE_MS;
    last.current = { key, at: now };
    setH((s) => {
      const value = typeof next === "function" ? (next as (p: T) => T)(s.present) : next;
      if (value === s.present) return s;
      if (merge) return { ...s, present: value, future: [] };
      return { past: [...s.past, s.present].slice(-MAX_HISTORY), present: value, future: [] };
    });
  }, []);

  const undo = useCallback(() => {
    last.current = { at: 0 };
    setH((s) => {
      if (s.past.length === 0) return s;
      const prev = s.past[s.past.length - 1];
      return { past: s.past.slice(0, -1), present: prev, future: [s.present, ...s.future] };
    });
  }, []);

  const redo = useCallback(() => {
    last.current = { at: 0 };
    setH((s) => {
      if (s.future.length === 0) return s;
      const [nxt, ...rest] = s.future;
      return { past: [...s.past, s.present], present: nxt, future: rest };
    });
  }, []);

  // reset — geçmişi temizleyerek yeni bir başlangıç durumu koyar (kayıt sonrası vb.).
  const reset = useCallback((value: T) => {
    last.current = { at: 0 };
    setH({ past: [], present: value, future: [] });
  }, []);

  return {
    state: h.present, set, undo, redo, reset,
    canUndo: h.past.length > 0, canRedo: h.future.length > 0,
  };
}

// useUndoShortcuts — Ctrl/⌘+Z geri al, Ctrl/⌘+Y ya da Ctrl/⌘+Shift+Z yinele.
// Bir metin alanındayken tarayıcının kendi alan-içi geri almasına dokunmaz.
export function useUndoShortcuts(undo: () => void, redo: () => void, enabled = true) {
  useEffect(() => {
    if (!enabled) return;
    function onKey(e: KeyboardEvent) {
      if (!(e.ctrlKey || e.metaKey)) return;
      const t = e.target as HTMLElement | null;
      const tag = t?.tagName;
      if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT" || t?.isContentEditable) return;
      const k = e.key.toLowerCase();
      if (k === "z" && !e.shiftKey) { e.preventDefault(); undo(); }
      else if (k === "y" || (k === "z" && e.shiftKey)) { e.preventDefault(); redo(); }
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [undo, redo, enabled]);
}

// PanelKit — Panel v3'ün ("kesintisiz koyu levha", bkz. Dashboard.tsx)
// paylaşılan yerleşim/ikon bileşenleri. Panel'den ayıklandı ki Proje Özet
// gibi başka sayfalar da aynı --panel-* token diline sahip aynı görsel
// dili kullanabilsin — kopyala/yapıştır yerine tek doğruluk kaynağı.
import type { ReactNode } from "react";
import RadialRing, { type ColorStop } from "./RadialRing";

export function PanelRow({ cols, children }: { cols: string; children: ReactNode }) {
  return (
    <div
      className="grid border-b last:border-b-0"
      style={{ gridTemplateColumns: cols, borderColor: "rgb(var(--panel-hairline))" }}
    >
      {children}
    </div>
  );
}

export function PanelCell({ title, action, children, noPad }: { title: string; action?: ReactNode; children: ReactNode; noPad?: boolean }) {
  return (
    <div
      className={"flex flex-col border-r last:border-r-0 " + (noPad ? "" : "p-4")}
      style={{ borderColor: "rgb(var(--panel-hairline))" }}
    >
      <div className={"flex items-center justify-between mb-3" + (noPad ? " px-4 pt-4" : "")}>
        <h2 className="text-[11px] font-extrabold uppercase tracking-wide" style={{ color: "rgb(var(--panel-ink2))" }}>{title}</h2>
        {action}
      </div>
      <div className="flex-1 min-h-0">{children}</div>
    </div>
  );
}

export function PanelStat({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <p className="text-[10px] uppercase tracking-wider" style={{ color: "rgb(var(--panel-ink3))" }}>{label}</p>
      <p className="mt-0.5 font-display text-base font-bold" style={{ color: "rgb(var(--panel-ink))", fontVariantNumeric: "tabular-nums" }}>{value}</p>
    </div>
  );
}

export function PanelLine({ label, value, bad }: { label: string; value: number; bad?: boolean }) {
  return (
    <div className="flex items-center gap-2">
      <span className="flex-1" style={{ color: "rgb(var(--panel-ink2))" }}>{label}</span>
      <span className="font-mono font-bold" style={{ color: bad ? "#f87171" : "rgb(var(--panel-ink))" }}>{value}</span>
    </div>
  );
}

export function PanelBadge({ label, tone }: { label: string; tone: "red" | "amber" | "blue" | "muted" }) {
  const style =
    tone === "red" ? { background: "rgba(248,113,113,.15)", color: "#f87171" }
    : tone === "amber" ? { background: "rgba(245,168,0,.15)", color: "var(--group-accent)" }
    : tone === "blue" ? { background: "rgba(96,165,250,.15)", color: "#60a5fa" }
    : { background: "rgb(var(--panel-hairline) / .6)", color: "rgb(var(--panel-ink2))" };
  return <span className="font-mono text-xs px-2 py-1 rounded" style={style}>{label}</span>;
}

export function PanelKpi({ label, value, hint, bad, good, icon, action }: {
  label: string; value: string; hint?: string; bad?: boolean; good?: boolean; icon: ReactNode; action?: ReactNode;
}) {
  const iconTone = bad ? "#f87171" : good ? "#4ade80" : "rgb(180,186,196)";
  return (
    <div className="flex items-center gap-3 p-3.5 border-r last:border-r-0" style={{ borderColor: "rgb(var(--panel-hairline))" }}>
      <div className="w-9 h-9 rounded-lg grid place-items-center shrink-0" style={{ background: `${iconTone}24` }}>
        <span style={{ color: iconTone, width: 17, height: 17, display: "block" }}>{icon}</span>
      </div>
      <div className="min-w-0 flex-1">
        <p className="text-[9px] uppercase tracking-wide font-bold truncate" style={{ color: "rgb(var(--panel-ink3))" }}>{label}</p>
        <p className="text-[17px] font-extrabold leading-tight" style={{ color: "rgb(var(--panel-ink))", fontVariantNumeric: "tabular-nums" }}>{value}</p>
        {hint && <p className="text-[10px] mt-0.5" style={{ color: bad ? "#f87171" : good ? "#4ade80" : "rgb(var(--panel-ink2))" }}>{hint}</p>}
        {action}
      </div>
    </div>
  );
}

export function PanelKpiRing({ label, pct, colorStops, hint, bad }: {
  label: string; pct: number; colorStops: ColorStop[]; hint?: string; bad?: boolean;
}) {
  return (
    <div className="flex items-center gap-3 p-3.5 border-r last:border-r-0" style={{ borderColor: "rgb(var(--panel-hairline))" }}>
      <RadialRing pct={pct} colorStops={colorStops} size={44} />
      <div className="min-w-0 flex-1">
        <p className="text-[9px] uppercase tracking-wide font-bold truncate" style={{ color: "rgb(var(--panel-ink3))" }}>{label}</p>
        <p className="text-[16px] font-extrabold leading-tight" style={{ color: "rgb(var(--panel-ink))", fontVariantNumeric: "tabular-nums" }}>%{pct.toFixed(1)}</p>
        {hint && <p className="text-[10px] mt-0.5" style={{ color: bad ? "#f87171" : "rgb(var(--panel-ink2))" }}>{hint}</p>}
      </div>
    </div>
  );
}

// ── İkonlar (satır ikonları, emoji değil) ──
export function IconTrend() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" width="100%" height="100%">
      <path d="M3 17l6-6 4 4 8-9" /><path d="M15 6h6v6" />
    </svg>
  );
}
export function IconCheck() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" width="100%" height="100%">
      <circle cx="12" cy="12" r="9" /><path d="M9 12l2 2 4-4" />
    </svg>
  );
}
export function IconShield() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" width="100%" height="100%">
      <path d="M12 3l7 3v6c0 4.5-3 7.5-7 9-4-1.5-7-4.5-7-9V6l7-3z" />
    </svg>
  );
}
export function IconWarning() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" width="100%" height="100%">
      <path d="M12 3L2 20h20L12 3z" /><path d="M12 10v4" /><circle cx="12" cy="17" r=".6" fill="currentColor" />
    </svg>
  );
}
export function IconReceipt() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" width="100%" height="100%">
      <rect x="3" y="7" width="18" height="13" rx="2" /><path d="M8 7V5a2 2 0 012-2h4a2 2 0 012 2v2" />
    </svg>
  );
}
export function IconFlag() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" width="100%" height="100%">
      <path d="M4 21V4" /><path d="M4 4h13l-2.5 4L17 12H4" />
    </svg>
  );
}
export function IconList() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" width="100%" height="100%">
      <path d="M9 6h11M9 12h11M9 18h11" /><circle cx="4.5" cy="6" r="1" fill="currentColor" stroke="none" />
      <circle cx="4.5" cy="12" r="1" fill="currentColor" stroke="none" /><circle cx="4.5" cy="18" r="1" fill="currentColor" stroke="none" />
    </svg>
  );
}

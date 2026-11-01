// Yazı logosu — hakediş raporundaki ConCoord işaretiyle aynı: kalın, bitişik
// harfler (harf aralığı yok) ve vurgu renginde "C". Baretli görsel logo yalnızca
// giriş ekranında kullanılır (bkz. pages/Login.tsx).
export default function Wordmark({
  size = 20, subtitle = false, color = "currentColor",
}: { size?: number; subtitle?: boolean; color?: string }) {
  return (
    <span className="inline-flex flex-col leading-none select-none" aria-label="ConCoord">
      <span style={{ fontWeight: 700, letterSpacing: 0, fontSize: size, color }}>
        CON<span style={{ color: "#e0b400" }}>C</span>OORD
      </span>
      {subtitle && (
        <span
          style={{
            marginTop: Math.max(5, size * 0.3), fontSize: Math.max(8, Math.min(9, size * 0.42)),
            letterSpacing: ".16em", textTransform: "uppercase", opacity: 0.7,
          }}
        >
          İnşaat proje yönetim platformu
        </span>
      )}
    </span>
  );
}

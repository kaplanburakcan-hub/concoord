import ComingSoon from "../../components/ComingSoon";

export default function DepoPage() {
  return (
    <ComingSoon
      title="Depo Raporları"
      description="Malzeme giriş/çıkış takibi. Girişler satınalma teslimat kayıtlarıyla ilişkilendirilir."
      icon={<svg viewBox="0 0 24 24"><path d="M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/><polyline points="9 22 9 12 15 12 15 22"/></svg>}
    />
  );
}

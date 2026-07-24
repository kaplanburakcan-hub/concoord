import ComingSoon from "../../components/ComingSoon";

export default function AraclarPage() {
  return (
    <ComingSoon
      title="Tanımlı Araçlar"
      description="Projede kullanılan araçların tanımı, atanmış projeler ve günlük puantaj."
      icon={<svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="9"/><path d="M12 8v4l3 2"/></svg>}
    />
  );
}

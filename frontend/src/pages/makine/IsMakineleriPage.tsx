import MachinePage from "./MachinePage";

export default function IsMakineleriPage() {
  return (
    <MachinePage
      tip="is_makinesi"
      tipLabel="İş Makineleri"
      showSeriNo
      showBakim
      logBirimi="saat"
    />
  );
}

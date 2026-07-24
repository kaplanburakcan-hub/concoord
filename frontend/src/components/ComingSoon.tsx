import { ReactNode } from "react";

// Henüz geliştirilmemiş modüller için yer tutucu sayfa.
// Navigasyon yapısı hazır; içerik aşama aşama eklenecek.
export default function ComingSoon({ title, description, icon }: {
  title: string;
  description?: string;
  icon?: ReactNode;
}) {
  return (
    <div className="flex flex-col items-center justify-center min-h-[400px] text-center px-4">
      {icon && (
        <div className="w-16 h-16 rounded-2xl bg-emniyet-500/10 grid place-items-center mb-4
                        [&>svg]:w-8 [&>svg]:h-8 [&>svg]:stroke-emniyet-500 [&>svg]:fill-none [&>svg]:[stroke-width:1.5]">
          {icon}
        </div>
      )}
      <h1 className="font-display text-2xl font-medium text-beton-100">{title}</h1>
      <p className="mt-2 text-beton-400 max-w-md">
        {description ?? "Bu modül yakında kullanıma açılacak. Mevcut çalışmalar devam ediyor."}
      </p>
      <span className="mt-4 inline-flex items-center gap-2 rounded-full border border-emniyet-500/30
                       bg-emniyet-500/10 px-3 py-1 text-xs text-emniyet-500">
        Geliştirme aşamasında
      </span>
    </div>
  );
}

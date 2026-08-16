-- Migration 000047 — Makine/Ekipman/Araç Envanteri Faz A: firma çapında
-- kanonik envanter. Şimdiye kadar her makine kaydı tek bir projeye
-- bağlıydı (project_machines.project_id NOT NULL) — aynı fiziksel
-- makine birden fazla projede "yeniden" kaydedilebiliyordu, hangi
-- makinenin gerçekte nerede olduğunun tek bir doğruluk kaynağı yoktu.
-- company_equipment artık o kanonik kayıt; project_machines (bkz.
-- migration 000048) buna bağlı bir "proje ataması" kaydına dönüşecek.

CREATE TABLE company_equipment (
    id                   uuid          PRIMARY KEY DEFAULT gen_random_uuid(),
    tip                  text          NOT NULL CHECK (tip IN ('arac','is_makinesi','ekipman')),
    ad                   text          NOT NULL,
    plaka                text,
    seri_no              text,
    demirbas_no          text,
    marka                text,
    model                text,
    uretim_yili          integer,
    sahiplik             text          NOT NULL DEFAULT 'ozmal' CHECK (sahiplik IN ('ozmal','kiralik')),
    tedarikci            text,
    gunluk_ucret         numeric(15,2),
    durum                text          NOT NULL DEFAULT 'aktif' CHECK (durum IN ('aktif','bakim','devre_disi')),
    son_bakim_tarihi     date,
    sonraki_bakim_tarihi date,
    aciklama             text,
    current_project_id   uuid          REFERENCES projects(id), -- NULL = merkez havuzda, hiçbir projeye atanmamış
    last_rental_reminder_at timestamptz, -- Faz E: kiralama sözleşmesi hatırlatması
    created_at           timestamptz   NOT NULL DEFAULT now(),
    updated_at           timestamptz   NOT NULL DEFAULT now(),
    row_version          integer       NOT NULL DEFAULT 1
);

-- Eşleştirme anahtarı tipe göre değişir (kullanıcı kararı): araçlarda
-- plaka, iş makinelerinde seri no, ekipmanlarda seri no ya da demirbaş
-- no. Kısmi (partial) indeksler sadece dolu değerlerde benzersizliği
-- zorunlu kılar — boş alanlar çakışma sayılmaz.
CREATE UNIQUE INDEX idx_ce_plaka ON company_equipment (plaka)
    WHERE tip = 'arac' AND plaka IS NOT NULL;
CREATE UNIQUE INDEX idx_ce_seri_no ON company_equipment (seri_no)
    WHERE tip IN ('is_makinesi','ekipman') AND seri_no IS NOT NULL;
CREATE UNIQUE INDEX idx_ce_demirbas_no ON company_equipment (demirbas_no)
    WHERE tip = 'ekipman' AND demirbas_no IS NOT NULL;
CREATE INDEX idx_ce_current_project ON company_equipment (current_project_id);

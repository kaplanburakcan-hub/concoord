"""DEMO-05 operasyon verisi: keşif, risk, uygunsuzluk, saha tutanakları, ceza,
Kasım 2026 taslak hakediş ve makine kullanım kayıtları.

Tüm veri uygulamanın kendi API uçlarıyla yazılır (kurallar, onay zinciri ve
hesaplar uygulamada işler). Betik yeniden çalıştırılabilir: var olan kayıt
atlanır, kimliği kullanılır.

Yerel (varsayılan):
  python gen_demo05_ops.py
Canlı:
  IPKS_BASE=https://concoord-backend.onrender.com/api/v1 IPKS_ADMIN_EMAIL=... IPKS_ADMIN_PASSWORD=... python gen_demo05_ops.py
Yerelde giriş bilgileri .env dosyasından okunur.
"""
import json
import os
import time
import urllib.error
import urllib.request
from datetime import date, timedelta

BASE = os.environ.get("IPKS_BASE", "http://localhost:8080/api/v1")

KATEGORI = {"Y.16": "Hafriyat", "Y.17": "Betonarme", "Y.20": "Çelik",
            "Y.25": "Mekanik", "Y.26": "Elektrik"}

RISKS = [
    ("Kasım yağışları temel betonunu erteleyebilir",
     "Yağış sürerse C30/37 temel dökümleri gün gün kayabilir.", 4, 3),
    ("Vinç kiralama süresinde gecikme",
     "Çelik montajı için kiralık vincin teslim tarihi netleşmedi.", 3, 4),
    ("Betonarme taşeronunda kadro devamsızlığı",
     "Kalıpçı ve donatı ustası sayısı son iki haftada düştü.", 3, 3),
    ("Elektrik şebeke bağlantısı gecikmesi",
     "Enerji dağıtım firmasının sayaç bağlantısı planlanan tarihten geç olabilir.", 3, 4),
]

NCRS = [
    ("Donatı aralıklarında standart dışı sapma", "Y.17.300",
     "Nervürlü donatı aralıkları projeye göre ±20 mm sapmış; sökülüp yeniden bağlanacak.", "Karadeniz Beton Yapı Ltd."),
    ("Temel betonunda numune alma eksikliği", "Y.17.210",
     "C30/37 temel dökümünde küp numune alınmadan döküm yapıldı.", "Karadeniz Beton Yapı Ltd."),
    ("Çelik kolon-kiriş kaynağında muayene dışı işlem", "Y.20.110",
     "Üç birleşimde kaynak muayene raporu olmadan kapatıldı.", "Anadolu Çelik Konstrüksiyon A.Ş."),
    ("Çatı makas montajında tork değeri kaydı eksik", "Y.20.160",
     "Civata sıkma tork kayıtları tutulmamış.", "Anadolu Çelik Konstrüksiyon A.Ş."),
    ("Sıhhi tesisat basınç testi kaydı eksik", "Y.25.100",
     "Borulama tamamlandıktan sonra basınç testi tutanağı hazırlanmadı.", "Işık Mekanik-Elektrik Tes. Ltd."),
    ("HVAC devreye alma raporu eksik", "Y.25.300",
     "Klima sisteminin devreye alma ölçümleri teslim edilmedi.", "Işık Mekanik-Elektrik Tes. Ltd."),
    ("Kablo kanalında topraklama ölçümü yapılmadı", "Y.26.100",
     "Kanal hattı topraklama direnci ölçülmeden kapatıldı.", "Işık Mekanik-Elektrik Tes. Ltd."),
    ("Aydınlatma armatürü sertifika uyumsuzluğu", "Y.26.300",
     "Teslim edilen armatürlerin CE belgesi ile etiket bilgileri uyuşmuyor.", "Işık Mekanik-Elektrik Tes. Ltd."),
    ("Kazı şevinde güvenlik önlemi yetersiz", "Y.16.001",
     "Şev açısı ve iksa kontrolü günlük kayıtlara işlenmemiş.", "Güçlü Toprak ve Hafriyat A.Ş."),
    ("Dolgu tabakasında sıkışma testi eksik", "Y.16.080",
     "Granüler dolgunun her 500 m³ için sıkışma testi yapılmadı.", "Güçlü Toprak ve Hafriyat A.Ş."),
]

PENALTIES = [
    # (taşeron adı, ihlal, seviye, tutar, not)
    ("Anadolu Çelik Konstrüksiyon A.Ş.", "Baretsiz çalışma (yüksekte)", "Fine", 40000,
     "Kolon montajında baret takılmadan çalışan iki işçi tespit edildi."),
    ("Karadeniz Beton Yapı Ltd.", "Emniyet kemeri kullanmama", "Fine", 25000,
     "Kalıp işçiliğinde yüksekte emniyet kemeri takılmadı."),
    ("Işık Mekanik-Elektrik Tes. Ltd.", "Elektrik panosu kilitleme ihlali", "Fine", 15000,
     "Enerji açık pano başında çalışma yapıldı."),
    ("Güçlü Toprak ve Hafriyat A.Ş.", "Şev güvenliği uyarısı", "Warning", None,
     "Kazı şevinde ilk uyarı yazıldı; tekrarında ceza uygulanacak."),
    ("Karadeniz Beton Yapı Ltd.", "Saha temizliği uyarısı", "Warning", None,
     "Döküm sonrası malzeme artıkları toplanmadı."),
]

# Saha tutanakları (Kasım 2026 hakedişine girecek).
# (taşeron, tip, başlık, miktar, birim, tutar, açıklama)
TUTANAKLAR = [
    ("Güçlü Toprak ve Hafriyat A.Ş.", "yevmiyeli", "Yevmiye tutanağı – 2-6 Kasım",
     42, "adam-gün", 1650 * 42, "Kazı ve dolgu ekibi, 2-6 Kasım iş günleri."),
    ("Karadeniz Beton Yapı Ltd.", "yevmiyeli", "Yevmiye tutanağı – 2-6 Kasım",
     38, "adam-gün", 1800 * 38, "Kalıp ve donatı ekibi, 2-6 Kasım iş günleri."),
    ("Anadolu Çelik Konstrüksiyon A.Ş.", "ek_imalat", "Ek imalat – kolon başlığı güçlendirme",
     6, "ton", 26000 * 6, "Proje değişikliğiyle eklenen kolon başlığı güçlendirme imalatı."),
    ("Işık Mekanik-Elektrik Tes. Ltd.", "mesai", "Mesai tutanağı – devreye alma hazırlığı",
     120, "saat", 450 * 120, "Pano devreye alma hazırlığı için hafta sonu mesaisi."),
]

PERIOD_START, PERIOD_END = "2026-11-01", "2026-11-30"


def load_env(path):
    env = {}
    with open(path, encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if line and not line.startswith("#") and "=" in line:
                k, v = line.split("=", 1)
                env[k.strip()] = v.strip()
    return env


def call(method, path, token=None, body=None):
    data = json.dumps(body).encode() if body is not None else None
    for attempt in range(8):
        req = urllib.request.Request(BASE + path, data=data, method=method)
        req.add_header("Content-Type", "application/json")
        if token:
            req.add_header("Authorization", "Bearer " + token)
        try:
            with urllib.request.urlopen(req, timeout=120) as r:
                raw = r.read()
                return r.status, (json.loads(raw) if raw else None)
        except urllib.error.HTTPError as e:
            if e.code == 429:
                time.sleep(5 * (attempt + 1))
                continue
            return e.code, e.read().decode(errors="replace")
    return 429, "hız sınırı aşıldı"


def expect(label, res, ok=(200, 201)):
    s, b = res
    if s not in ok:
        raise SystemExit(f"{label} başarısız: {s} {str(b)[:300]}")
    return b


def main():
    here = os.path.dirname(os.path.abspath(__file__))
    if "IPKS_ADMIN_EMAIL" in os.environ:
        email, pw = os.environ["IPKS_ADMIN_EMAIL"], os.environ["IPKS_ADMIN_PASSWORD"]
    else:
        env = load_env(os.path.join(here, "..", "..", ".env"))
        email, pw = env["IPKS_BOOTSTRAP_ADMIN_EMAIL"], env["IPKS_BOOTSTRAP_ADMIN_PASSWORD"]
    token = expect("login", call("POST", "/auth/login", None, {"identifier": email, "password": pw}))["tokens"]["access_token"]
    pid = next(p["id"] for p in expect("projects", call("GET", "/projects", token))["projects"] if p["code"] == "DEMO-05")
    base = f"/projects/{pid}"

    subs = {s["company_name"]: s["id"] for s in expect("subs", call("GET", base + "/subcontractors", token))["subcontractors"]}
    wi_by_poz = {}
    for sid in subs.values():
        for w in expect("wi", call("GET", f"{base}/subcontractors/{sid}/work-items", token))["work_items"]:
            wi_by_poz[w["poz_no"]] = w

    # 1) Proje keşfi: sözleşme poz'ları, her kalem iş kalemine bağlı.
    have_kesif = {s["poz_no"] for s in expect("keşif", call("GET", base + "/survey-items", token))["items"]}
    n = 0
    for i, (poz, w) in enumerate(sorted(wi_by_poz.items()), start=1):
        if poz in have_kesif:
            continue
        expect("keşif " + poz, call("POST", base + "/survey-items", token, {
            "kategori": KATEGORI.get(poz[:4], "Diğer"), "poz_no": poz, "tanim": w["description"],
            "birim": w["unit"], "miktar": w["contract_qty"], "birim_fiyat": w["unit_price"],
            "para_birimi": "TRY", "aciklama": "Sözleşme pozu ile eşleşik keşif kalemi.",
            "sira": i * 10}))
        n += 1
    print("keşif yazıldı:", n, "/ toplam", len(wi_by_poz))

    # 2) Riskler.
    have_risk = {r["title"] for r in expect("risk", call("GET", base + "/risks", token))["risks"]}
    n = 0
    for title, desc, prob, imp in RISKS:
        if title in have_risk:
            continue
        expect("risk", call("POST", base + "/risks", token, {
            "title": title, "description": desc, "probability": prob, "impact": imp}))
        n += 1
    print("risk yazıldı:", n)

    # 3) Uygunsuzluklar (NCR), imalat kalemi poz'larına bağlı.
    have_ncr = {x["title"] for x in expect("ncr", call("GET", base + "/ncrs", token))["ncrs"]}
    n = 0
    for title, poz, desc, sub in NCRS:
        if title in have_ncr:
            continue
        expect("ncr " + poz, call("POST", base + "/ncrs", token, {
            "title": title, "poz_no": poz, "description": desc, "subcontractor_id": subs[sub]}))
        n += 1
    print("uygunsuzluk yazıldı:", n)

    # 4) Saha tutanakları: yoksa oluştur, onay zinciri bitene kadar onaya gönder.
    existing = expect("tutanak", call("GET", base + "/tutanaklar", token))["tutanaklar"]
    tut_ids = {}
    for sub, tip, baslik, miktar, birim, tutar, aciklama in TUTANAKLAR:
        ex = next((t for t in existing if t["taseron_id"] == subs[sub] and t["baslik"] == baslik), None)
        if ex:
            tid = ex["id"]
        else:
            t = expect("tutanak", call("POST", base + "/tutanaklar", token, {
                "tip": tip, "baslik": baslik, "tarih": "2026-11-06", "taseron_id": subs[sub],
                "aciklama": aciklama, "tutar": tutar, "birim": birim, "miktar": miktar,
                "kisim_sefi_var": True}))
            tid = t["id"] if "id" in t else t["tutanak"]["id"]
            expect("gönder", call("POST", f"{base}/tutanaklar/{tid}/submit", token, {}))
            for _ in range(10):
                cur = next(x for x in expect("tutanak liste", call("GET", base + "/tutanaklar", token))["tutanaklar"]
                           if x["id"] == tid)
                if cur["durum"] != "onay_sureci":
                    break
                expect("karar", call("POST", f"{base}/tutanaklar/{tid}/decide", token, {"karar": "onaylandi"}))
        tut_ids[sub] = tid
    print("tutanak hazır:", len(tut_ids))

    # 5) İSG cezaları. Fine'lar taşeronun taslak hakedişinde kesinti olur.
    have_pen = {(p["subcontractor_id"], p["violation_type"]): p
                for p in expect("ceza", call("GET", base + "/ohs/penalties", token))["penalties"]}
    pen_for_sub = {}
    n = 0
    for sub, ihlal, seviye, tutar, not_ in PENALTIES:
        p = have_pen.get((subs[sub], ihlal))
        if p is None:
            body = {"subcontractor_id": subs[sub], "violation_type": ihlal, "penalty_level": seviye, "note": not_}
            if tutar is not None:
                body["amount"] = tutar
            created = expect("ceza", call("POST", base + "/ohs/penalties", token, body))
            p = created.get("penalty") or created
            n += 1
        if p.get("penalty_level") == "Fine":
            pen_for_sub.setdefault(sub, []).append(p)
    print("ceza yazıldı:", n)

    # 6) Kasım 2026 taslak hakedişi (taşeron başına): tutanaklı imalat + ceza kesintisi.
    pays = expect("hakediş", call("GET", base + "/payments", token))["payments"]
    for sub, sid in subs.items():
        pay = next((p for p in pays if p["subcontractor_id"] == sid and p["period_start"].startswith(PERIOD_START)), None)
        if pay is None:
            created = expect("taslak", call("POST", base + "/payments", token, {
                "subcontractor_id": sid, "period_start": PERIOD_START, "period_end": PERIOD_END, "vat_pct": 20}))
            pay_id = created["payment"]["id"] if "payment" in created else created["id"]
        else:
            pay_id = pay["id"]
        det = expect("taslak detay", call("GET", f"{base}/payments/{pay_id}", token))
        if det["payment"]["status"] != "Draft":
            continue
        # Önceki kesinmiş dönemin kümülatif miktarları (poz başına).
        prev_cum = {}
        fin = [p for p in pays if p["subcontractor_id"] == sid and p["status"] == "Finalized"]
        if fin:
            last = max(fin, key=lambda p: p["period_end"])
            for it in expect("önceki dönem", call("GET", f"{base}/payments/{last['id']}", token))["items"]:
                prev_cum[it["work_item_id"]] = it["cum_qty"]
        # Taşeronun tüm iş kalemleri: önceki kümülatif + bu dönem (sözleşme miktarının %3'ü).
        sub_items = expect("iş kalemleri", call("GET", f"{base}/subcontractors/{sid}/work-items", token))["work_items"]
        items = []
        for k, w in enumerate(sub_items):
            entry = {"work_item_id": w["id"],
                     "cum_qty": round(prev_cum.get(w["id"], 0) + w["contract_qty"] * 0.03, 3)}
            if k == 0 and sub in tut_ids:
                entry["tutanak_id"] = tut_ids[sub]
            items.append(entry)
        deds = [{"type": "OHSPenalty", "description": "İSG para cezası " + p["penalty_no"],
                 "amount": p["amount"], "source_entity": "ohs_penalties", "source_id": p["id"]}
                for p in pen_for_sub.get(sub, [])]
        expect("taslak güncelle", call("PATCH", f"{base}/payments/{pay_id}", token, {
            "row_version": det["payment"]["row_version"], "items": items, "deductions": deds}))
    print("taslak hakediş hazır:", len(subs))

    # 7) Makine kullanım kayıtları: Ağustos-Ekim 2026, iki haftada bir (yoksa eklenir).
    machines = expect("makine", call("GET", base + "/machines", token))["machines"]
    n = 0
    for m in machines:
        logs = expect("log", call("GET", f"{base}/machines/{m['id']}/logs", token))
        have = {x["tarih"][:10] for x in logs.get("logs", logs if isinstance(logs, list) else [])}
        d = date(2026, 8, 1)
        while d <= date(2026, 10, 31):
            if d.isoformat() not in have:
                expect("makine log", call("POST", f"{base}/machines/{m['id']}/logs", token, {
                    "tarih": d.isoformat(), "calisma_miktari": 8, "calisma_birimi": "saat",
                    "operator": "Demo operatör", "aciklama": "Saha kullanım kaydı."}))
                n += 1
            d += timedelta(days=14)
    print("makine kaydı yazıldı:", n)


if __name__ == "__main__":
    main()

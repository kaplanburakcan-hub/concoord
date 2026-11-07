"""DEMO-05 (Sakarya Entegre Lojistik Depo) tohum verisi üretici.

Tüm sayılar burada tutarlı şekilde hesaplanır; çıktı bir SQL dosyasıdır.
Hedefler (yerel test sonrası ölçülür):
  - Fiziki ilerleme (EV/BAC)  ~ %60
  - Finansal ilerleme (idari hakediş / sözleşme bedeli) ~ %65
  - CPI ~ 0.92, SPI ~ 0.96
Çalıştırma: python gen_demo05.py > demo-05-sakarya-lojistik.sql
"""
import calendar
import json
import math
import random
import sys
import uuid
from datetime import date, datetime, timedelta

random.seed(505)

BAC = 180_000_000.0
MAIN_CONTRACT = 180_000_000.0
PROJECT_CODE = "DEMO-05"
START = date(2025, 6, 1)
END = date(2027, 12, 31)
NOW_MONTH_IDX = 16  # 2026-10 = son tamamlanmış hakediş ayı (0 = 2025-06)
MONTHS = []
y, m = 2025, 6
while (y, m) <= (2027, 12):
    MONTHS.append((y, m))
    m += 1
    if m > 12:
        y, m = y + 1, 1


def uid():
    return str(uuid.uuid4())


def q(s):
    if s is None:
        return "NULL"
    return "'" + str(s).replace("'", "''") + "'"


def n(x):
    return "NULL" if x is None else f"{round(float(x), 2):.2f}"


def month_end(y, m):
    return date(y, m, calendar.monthrange(y, m)[1])


def smooth(x):
    x = max(0.0, min(1.0, x))
    return 3 * x * x - 2 * x ** 3


# ---------------------------------------------------------------- taşeronlar
SUBS = [
    dict(key="toprak", name="Güçlü Toprak ve Hafriyat A.Ş.", tax="5101112233", contact="Hasan Çelik",
         phone="0537 100 1100", email="proje@guclutoprak.com", trade="Toprak İşleri",
         start=(2025, 7), ramp_end=(2026, 3), final=1.0, contract_no="DEMO05-SZL-01",
         items=[("Y.16.001", "Kazı ve hafriyat taşınması", "m3", 150000, 90.0, "Hafriyat"),
                ("Y.16.080", "Granüler dolgu serimi", "m3", 60000, 140.0, "Hafriyat")]),
    dict(key="kaba", name="Karadeniz Beton Yapı Ltd.", tax="5102223344", contact="Fatma Arslan",
         phone="0532 200 2200", email="info@karadenizbeton.com", trade="Betonarme",
         start=(2025, 10), ramp_end=(2026, 10), final=None, contract_no="DEMO05-SZL-02",
         items=[("Y.17.210", "Temel betonu (C30/37)", "m3", 6500, 2600.0, "Betonarme"),
                ("Y.17.240", "Kolon-kiriş-perde betonu (C35/45)", "m3", 4200, 3200.0, "Betonarme"),
                ("Y.17.260", "Döşeme betonu (C30/37)", "m3", 3800, 2900.0, "Betonarme"),
                ("Y.17.300", "Nervürlü betonarme donatı", "ton", 780, 24000.0, "Betonarme")]),
    dict(key="celik", name="Anadolu Çelik Konstrüksiyon A.Ş.", tax="5103334455", contact="Kemal Şahin",
         phone="0533 300 3300", email="proje@anadolucelik.com", trade="Çelik Konstrüksiyon",
         start=(2026, 3), ramp_end=(2026, 10), final=0.35, contract_no="DEMO05-SZL-03",
         items=[("Y.20.110", "Çelik kolon-kiriş imalat ve montajı", "ton", 950, 26000.0, "Çelik"),
                ("Y.20.160", "Çatı makas ve gerdirme", "ton", 420, 22000.0, "Çelik")]),
    dict(key="mep", name="Işık Mekanik-Elektrik Tes. Ltd.", tax="5104445566", contact="Sibel Güneş",
         phone="0534 400 4400", email="info@isikmep.com", trade="Mekanik-Elektrik",
         start=(2026, 6), ramp_end=(2026, 10), final=0.20, contract_no="DEMO05-SZL-04",
         items=[("Y.25.100", "Sıhhi tesisat borulama", "m", 9500, 780.0, "Mekanik"),
                ("Y.26.100", "Elektrik kablo ve kanal", "m", 18000, 420.0, "Elektrik"),
                ("Y.26.300", "Aydınlatma armatürü", "adet", 1200, 1850.0, "Elektrik"),
                ("Y.25.300", "HVAC klima sistemi", "ad", 1, 6500000.0, "Mekanik")]),
]
for s in SUBS:
    s["id"] = uid()
    s["contract_id"] = uid()
    s["amount"] = round(sum(q_ * p for _, _, _, q_, p, _ in s["items"]), 2)
    s["work_ids"] = [uid() for _ in s["items"]]

CONTRACTS_TOTAL = sum(s["amount"] for s in SUBS)

# Kaba inşaat fiziki tamamlanma oranı: toplam hakediş brütü = %60 × sözleşme toplamı
def sub_final_total(s):
    return s["amount"] * (s["final"] if s["final"] is not None else 1.0)

target_gross = 0.60 * CONTRACTS_TOTAL
others = sum(sub_final_total(s) for s in SUBS if s["key"] != "kaba")
kaba = next(s for s in SUBS if s["key"] == "kaba")
kaba["final"] = (target_gross - others) / kaba["amount"]


def sub_fraction(s, mi):
    """Taşeronun ay (index) sonundaki kümülatif tamamlanma oranı."""
    a = (s["start"][0] - 2025) * 12 + (s["start"][1] - 6)
    e = (s["ramp_end"][0] - 2025) * 12 + (s["ramp_end"][1] - 6)
    if mi < a:
        return 0.0
    ramp = max(1, e - a + 1)
    frac = smooth(min(1.0, (mi - a + 1) / ramp))
    return s["final"] * frac


# ------------------------------------------------------------ hakediş hesabı
progress_rows = []  # her sub-ay için dict
month_gross = {}    # mi -> toplam gross_this
for s in SUBS:
    prev_cum = 0.0
    period_no = 0
    for mi in range(NOW_MONTH_IDX + 1):
        f = sub_fraction(s, mi)
        if f <= 0:
            continue
        period_no += 1
        y, m = MONTHS[mi]
        items = []
        gross_cum = 0.0
        for (poz, desc, unit, qty, price, _), wid in zip(s["items"], s["work_ids"]):
            cq = round(qty * f, 2)
            ca = round(cq * price, 2)
            gross_cum += ca
            items.append(dict(wid=wid, poz=poz, unit=unit, price=price, cum_qty=cq, cum_amount=ca))
        gross_cum = round(gross_cum, 2)
        gross_this = round(gross_cum - prev_cum, 2)
        for it in items:
            it["prev_qty"] = 0.0
        progress_rows.append(dict(sub=s, mi=mi, period_no=period_no, y=y, m=m,
                                  gross_cum=gross_cum, gross_prev=round(prev_cum, 2),
                                  gross_this=gross_this, items=items, id=uid()))
        month_gross[mi] = month_gross.get(mi, 0.0) + gross_this
        prev_cum = gross_cum

# item prev/this qty hesapları (her sub için ardışık ay)
by_sub = {}
for r in progress_rows:
    by_sub.setdefault(r["sub"]["key"], []).append(r)
for rows in by_sub.values():
    prev = {}
    for r in rows:
        for it in r["items"]:
            it["prev_qty"] = prev.get(it["wid"], 0.0)
            it["this_qty"] = round(it["cum_qty"] - it["prev_qty"], 2)
            prev[it["wid"]] = it["cum_qty"]

GROSS_TOTAL = round(sum(month_gross.values()), 2)
ACTUAL_HAK = 0.0
for r in progress_rows:
    r["actual_cost"] = round(r["gross_this"] * 0.97, 2)
    ACTUAL_HAK += r["actual_cost"]
ACTUAL_HAK = round(ACTUAL_HAK, 2)

# ------------------------------------------------------------- idari hakediş
IDARI_TARGET = round(0.65 * MAIN_CONTRACT, 2)
mult = IDARI_TARGET / GROSS_TOTAL
idari = []
running = 0.0
mis = sorted(month_gross)
for k, mi in enumerate(mis):
    if k == len(mis) - 1:
        tutar = round(IDARI_TARGET - running, 2)
    else:
        tutar = round(month_gross[mi] * mult, 2)
    running += tutar
    y, m = MONTHS[mi]
    idari.append(dict(no=k + 1, y=y, m=m, tutar=tutar, prev=round(running - tutar, 2),
                      paid=(mi < NOW_MONTH_IDX)))

# ------------------------------------------------------------------ PO / PR
PO_DELIVERED = [
    ("Çelik profil malzeme tedariki", "Mavi Çelik Tic. Ltd.", 12_000_000, "04 Çelik Yapı", date(2026, 2, 20)),
    ("Trafo ve ana pano", "Enerji Ekipman A.Ş.", 4_800_000, "06 Elektrik Tesisat", date(2026, 5, 15)),
    ("Jeneratör ve UPS sistemi", "Enerji Ekipman A.Ş.", 3_200_000, "06 Elektrik Tesisat", date(2026, 7, 2)),
    ("Yangın söndürme tesisatı malzemesi", "Yangın Sistemleri Ltd.", 2_600_000, "05 Mekanik Tesisat", date(2026, 6, 18)),
    ("Şantiye konteyneri ve altyapı", "Saha Lojistik Ltd.", 2_400_000, "01 Genel Giderler", date(2025, 8, 5)),
    ("Cephe panel kaplama malzemesi", "Yapı Kaplama A.Ş.", 6_000_000, "07 Dış Cephe", date(2026, 9, 12)),
    ("Hidrolik asansör", "Asansör Dünyası Ltd.", 4_000_000, "05 Mekanik Tesisat", date(2026, 10, 5)),
]
PO_OPEN = [
    ("Cephe alüminyum doğrama", "Alüm Doğrama Sanayi", 4_500_000, "07 Dış Cephe", date(2026, 12, 15)),
    ("Güvenlik ve CCTV sistemi", "Güven Güvenlik Sist.", 1_500_000, "06 Elektrik Tesisat", date(2026, 11, 30)),
]
DELIVERED_TOTAL = sum(p[2] for p in PO_DELIVERED)
AC_TOTAL = round(ACTUAL_HAK + DELIVERED_TOTAL, 2)

COST_CODES = [
    ("01", "Genel Giderler ve Şantiye", 14_000_000),
    ("02", "Hafriyat ve Altyapı", 22_000_000),
    ("03", "Kaba İnşaat (Betonarme)", 62_000_000),
    ("04", "Çelik Yapı", 30_000_000),
    ("05", "Mekanik Tesisat", 18_000_000),
    ("06", "Elektrik Tesisat", 16_000_000),
    ("07", "Dış Cephe ve Kaplama", 12_000_000),
    ("08", "Yol ve Saha", 6_000_000),
]
assert abs(sum(c[2] for c in COST_CODES) - BAC) < 1
CODE_IDS = {c[0] + " " + c[1]: uid() for c in COST_CODES}

def code_id(text):
    # PO tarafındaki kod metni numarayla başlar ("04 Çelik Yapı")
    return next(v for k, v in CODE_IDS.items() if k[:2] == text[:2])

# -------------------------------------------------------------------- PV
pv = []
T = len(MONTHS) - 1  # 30 aralık
prev_c = 0.0
for k in range(len(MONTHS)):
    c = 100.0 * (1 - math.cos(math.pi * (k + 1) / (T + 1))) / 2
    inc = round(c - prev_c, 4)
    prev_c = c
    pv.append((MONTHS[k], inc))
pv_total = round(sum(p for _, p in pv), 2)
pv_at_now = round(sum(p for (y, m), p in pv if (y, m) <= (2026, 11)), 2)

# ------------------------------------------------------------- raporlar
def sql_sub(s):
    return f"""INSERT INTO subcontractors (id, project_id, company_name, tax_no, contact_person, phone, email, trade)
VALUES ('{s['id']}', v_proj, {q(s['name'])}, {q(s['tax'])}, {q(s['contact'])}, {q(s['phone'])}, {q(s['email'])}, {q(s['trade'])});"""


def main():
    out = []
    w = out.append
    w("-- ============================================================================")
    w("-- DEMO-05 — Sakarya Entegre Lojistik Depo ve İdari Bina (gen_demo05.py çıktısı)")
    w("-- Fiziki ilerleme ~%60, finansal ilerleme ~%65, CPI ~0.92, SPI ~0.96 hedeflenir.")
    w("-- Önceden DEMO-05 varsa arşivlenir (idempotent).")
    w("-- ============================================================================")
    w("DO $$")
    w("DECLARE")
    w("  v_admin uuid; v_role uuid; v_proj uuid;")
    w("BEGIN")
    w("  SELECT id INTO v_admin FROM users WHERE deleted_at IS NULL ORDER BY created_at LIMIT 1;")
    w("  IF v_admin IS NULL THEN RAISE EXCEPTION 'Kullanıcı yok'; END IF;")
    w("  SELECT id INTO v_role FROM roles WHERE name = 'ProjectManager' LIMIT 1;")
    w("  IF v_role IS NULL THEN SELECT id INTO v_role FROM roles ORDER BY created_at LIMIT 1; END IF;")
    w(f"  UPDATE projects SET code = code || '-ARSIV-' || to_char(now(),'YYYYMMDDHH24MISS'), name = name || ' (arşiv)', status = 'Archived', deleted_at = now()")
    w(f"  WHERE code = '{PROJECT_CODE}' AND deleted_at IS NULL;")
    w("")
    w("  -- 1) PROJE")
    w(f"  INSERT INTO projects (code, name, location, client_name, budget_total, currency, start_date, end_date, status)")
    w(f"  VALUES ('{PROJECT_CODE}', 'Sakarya Entegre Lojistik Depo ve İdari Bina', 'Sakarya / Hendek',")
    w(f"          'Sakarya Lojistik Yatırım A.Ş.', {n(BAC)}, 'TRY', DATE '{START}', DATE '{END}', 'Active')")
    w("  RETURNING id INTO v_proj;")
    w("  INSERT INTO project_members (project_id, user_id, role_id) VALUES (v_proj, v_admin, v_role);")
    w("")
    w("  -- 2) ANA SÖZLEŞME (idari hakediş oranı bu bedele göre)")
    w(f"""  INSERT INTO project_main_contracts (project_id, sozlesme_turu, fiyat_farki_var, sozlesme_bedeli, sozlesme_para_birimi,
    sozlesme_tarihi, yer_teslim_tarihi, is_suresi_gun, is_turu, kdv_pct, isveren_adi, yuklenici_adi, yuklenici_proje_sorumlusu,
    created_by, updated_by, is_locked)
  VALUES (v_proj, 'goturu_bedel', false, {n(MAIN_CONTRACT)}, 'TRY', DATE '2025-05-15', DATE '2025-06-01', 915, 'yapim', 20,
    'Sakarya Lojistik Yatırım A.Ş.', 'Demo Yapı Müteahhitlik A.Ş.', 'Proje Müdürü (Demo)', v_admin, v_admin, false);""")
    w("")
    w("  -- 3) TAŞERONLAR + SÖZLEŞMELER + BİRİM FİYAT POZLARI")
    for s in SUBS:
        w(sql_sub(s))
        w(f"""INSERT INTO contracts (id, project_id, subcontractor_id, contract_no, type, amount, advance_amount, retention_pct,
    advance_rate_pct, sign_date, start_date, end_date, is_multi_year, withholding_pct)
VALUES ('{s['contract_id']}', v_proj, '{s['id']}', {q(s['contract_no'])}, 'Sub', {n(s['amount'])}, {n(round(s['amount']*0.15,2))}, 5.00, 15.00,
    DATE '{s['start'][0]}-{s['start'][1]:02d}-01', DATE '{s['start'][0]}-{s['start'][1]:02d}-01', DATE '2027-06-30', true, 0.00);""")
        for (poz, desc, unit, qty, price, _), wid in zip(s["items"], s["work_ids"]):
            w(f"""INSERT INTO work_items (id, project_id, subcontractor_id, contract_id, poz_no, description, unit, contract_qty, unit_price)
VALUES ('{wid}', v_proj, '{s['id']}', '{s['contract_id']}', {q(poz)}, {q(desc)}, {q(unit)}, {n(qty)}, {n(price)});""")
    w("")
    w("  -- 4) MALİYET KODLARI + BÜTÇE (toplam = BAC)")
    for code, name, budget in COST_CODES:
        cid = CODE_IDS[code + " " + name]
        w(f"INSERT INTO cost_codes (id, project_id, code, name, created_by) VALUES ('{cid}', v_proj, {q(code)}, {q(name)}, v_admin);")
        w(f"INSERT INTO cost_code_budgets (cost_code_id, project_id, amount, updated_by) VALUES ('{cid}', v_proj, {n(budget)}, v_admin);")
    w("")
    w("  -- 5) HAKEDİŞLER (kesinleşmiş; EV ve AC buradan türer)")
    for r in progress_rows:
        s = r["sub"]
        fin = date(r["y"] + (1 if r["m"] == 12 else 0), 1 if r["m"] == 12 else r["m"] + 1, 5)
        ps = date(r["y"], r["m"], 1)
        pe = month_end(r["y"], r["m"])
        vat = round(r["gross_this"] * 0.20, 2)
        ded = round(r["gross_this"] * 0.05, 2)
        net = round(r["gross_this"] - ded, 2)
        w(f"""INSERT INTO progress_payments (id, project_id, subcontractor_id, period_no, period_start, period_end, status,
    gross_cum, gross_prev, gross_this, total_deductions, vat_pct, net_payable, submitted_at, site_approved_at, finalized_at,
    finalized_by, created_by, withholding_applied, vat_amount, vat_collected, payable_gross, actual_cost, is_final)
VALUES ('{r['id']}', v_proj, '{s['id']}', {r['period_no']}, DATE '{ps}', DATE '{pe}', 'Draft',
    {n(r['gross_cum'])}, {n(r['gross_prev'])}, {n(r['gross_this'])}, {n(ded)}, 20.00, {n(net)},
    NULL, NULL, NULL,
    v_admin, v_admin, false, {n(vat)}, {n(vat)}, {n(r['gross_this'])}, {n(r['actual_cost'])}, false);""")
        for it in r["items"]:
            w(f"""INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('{r['id']}', '{it['wid']}', {n(it['prev_qty'])}, {n(it['this_qty'])}, {n(it['cum_qty'])}, {n(it['cum_amount'])}, {n(round(it['this_qty']*it['price'],2))});""")
        w(f"""UPDATE progress_payments SET status='Finalized', submitted_at=TIMESTAMPTZ '{fin} 09:00:00+03', site_approved_at=TIMESTAMPTZ '{fin} 10:00:00+03', finalized_at=TIMESTAMPTZ '{fin} 11:00:00+03' WHERE id='{r['id']}';""")
    w("")
    w("  -- 6) İDARİ HAKEDİŞ (işveren hakedişleri; finansal ilerleme)")
    for d in idari:
        hd = month_end(d["y"], d["m"])
        odeme = f"DATE '{hd + timedelta(days=30)}'" if d["paid"] else "NULL"
        aciklama = f"{d['y']}-{d['m']:02d} dönemi hakedişi"
        w(f"""INSERT INTO idari_hakedisler (project_id, donem_no, aciklama, tutar, kdv_pct, fatura_no, gelen_odeme_tarihi,
        created_by, hakedis_tarihi, onceki_hakedis_toplami, kesintiler)
    VALUES (v_proj, {d['no']}, {q(aciklama)}, {n(d['tutar'])}, 20.00,
        {q(f'FT-2026-{d["no"]:04d}')}, {odeme}, v_admin, DATE '{hd}', {n(d['prev'])}, '[]'::jsonb);""")
    w("")
    w("  -- 7) İŞ PROGRAMI KİLOMETRE TAŞLARI (PV kaynağı: milestone yedeği)")
    ms = [
        ("Şantiye kurulumu ve mobilizasyon", date(2025, 6, 30), date(2025, 7, 5), 5, "Completed"),
        ("Temel tamamlanması", date(2026, 1, 31), date(2026, 1, 20), 15, "Completed"),
        ("1. kat kaba inşaat tamamlanması", date(2026, 5, 31), date(2026, 6, 9), 20, "Completed"),
        ("Çelik çatı montajı", date(2026, 9, 30), None, 20, "Delayed"),
        ("Kaba inşaat tamamlanması", date(2026, 12, 31), None, 25, "Planned"),
        ("MEP ön kabul ve test", date(2027, 6, 30), None, 15, "Planned"),
    ]
    for i, (name, pd, ad, wgt, st) in enumerate(ms, 1):
        w(f"""INSERT INTO milestones (project_id, name, planned_date, actual_date, weight_pct, status, sort_order)
VALUES (v_proj, {q(name)}, DATE '{pd}', {('DATE ' + chr(39) + str(ad) + chr(39)) if ad else 'NULL'}, {n(wgt)}, {q(st)}, {i});""")
    w("")
    w("  -- 8) PV PLANI (aylık planlanan % — kümülatif toplam = %100)")
    for (y, m), p in pv:
        w(f"INSERT INTO pv_plan_entries (project_id, month, planned_pct) VALUES (v_proj, DATE '{y}-{m:02d}-01', {p:.4f});")
    w("")
    w("  -- 9) SATINALMA: PR + PO + TESLİMAT")
    po_no = 1
    for (desc, sup, amt, code, dd) in PO_DELIVERED:
        pr_id, po_id = uid(), uid()
        cid = code_id(code)
        w(f"""INSERT INTO purchase_requests (id, project_id, pr_no, requested_by, needed_by_date, status, submitted_at, decided_by, decided_at, decision_note, cost_code_id)
VALUES ('{pr_id}', v_proj, 'TLP-2025-{po_no:03d}', v_admin, DATE '{dd}', 'Draft', NULL, NULL, NULL, NULL, '{cid}');""")
        w(f"""INSERT INTO purchase_request_items (pr_id, material_name, qty, unit) VALUES ('{pr_id}', {q(desc)}, 1, 'ad');""")
        w(f"""UPDATE purchase_requests SET status='Approved', submitted_at=TIMESTAMPTZ '{dd} 09:00:00+03', decided_by=v_admin, decided_at=TIMESTAMPTZ '{dd} 10:00:00+03', decision_note='Onaylandı' WHERE id='{pr_id}';""")
        w(f"""INSERT INTO purchase_orders (id, project_id, pr_id, po_no, supplier_name, amount, currency, status, expected_date, created_by, cost_code_id)
VALUES ('{po_id}', v_proj, '{pr_id}', 'SIP-2025-{po_no:03d}', {q(sup)}, {n(amt)}, 'TRY', 'Ordered', DATE '{dd}', v_admin, '{cid}');""")
        w(f"""INSERT INTO deliveries (po_id, delivery_note_no, delivered_at, received_by, note) VALUES ('{po_id}', 'IRS-{po_no:04d}', TIMESTAMPTZ '{dd} 14:00:00+03', v_admin, {q(desc + ' teslim alındı')});""")
        w(f"""UPDATE purchase_orders SET status='Delivered' WHERE id='{po_id}';""")
        po_no += 1
    for (desc, sup, amt, code, dd) in PO_OPEN:
        pr_id, po_id = uid(), uid()
        cid = code_id(code)
        w(f"""INSERT INTO purchase_requests (id, project_id, pr_no, requested_by, needed_by_date, status, submitted_at, decided_by, decided_at, decision_note, cost_code_id)
VALUES ('{pr_id}', v_proj, 'TLP-2026-{po_no:03d}', v_admin, DATE '{dd}', 'Draft', NULL, NULL, NULL, NULL, '{cid}');""")
        w(f"""INSERT INTO purchase_request_items (pr_id, material_name, qty, unit) VALUES ('{pr_id}', {q(desc)}, 1, 'ad');""")
        w(f"""UPDATE purchase_requests SET status='Approved', submitted_at=TIMESTAMPTZ '2026-10-20 09:00:00+03', decided_by=v_admin, decided_at=TIMESTAMPTZ '2026-10-21 10:00:00+03', decision_note='Onaylandı' WHERE id='{pr_id}';""")
        w(f"""INSERT INTO purchase_orders (id, project_id, pr_id, po_no, supplier_name, amount, currency, status, expected_date, created_by, cost_code_id)
VALUES ('{po_id}', v_proj, '{pr_id}', 'SIP-2026-{po_no:03d}', {q(sup)}, {n(amt)}, 'TRY', 'Ordered', DATE '{dd}', v_admin, '{cid}');""")
        po_no += 1
    w("")
    w("  -- 10) KARARLAR (karar günlüğü)")
    decisions = [
        (date(2026, 2, 12), "Çelik çatı montaj sırası değiştirildi: önce kuzey açıklık tamamlanacak."),
        (date(2026, 6, 20), "Trafo odası konumu mimari projeye uygun hale getirildi (revizyon R2)."),
        (date(2026, 9, 25), "Cephe panel tedariki tek sözleşmeyle yapılacak; taşeron iş kapsamı dışında."),
    ]
    for d, text in decisions:
        w(f"INSERT INTO project_decisions (project_id, decision_text, decided_on, created_by) VALUES (v_proj, {q(text)}, DATE '{d}', v_admin);")

    # ---------------------------------------------------- 11) İŞ PROGRAMI (WBS)
    w("  -- 11) İŞ PROGRAMI (WBS kalemleri; baseline + gerçekleşen)")
    wbs = [
        ("1", "Şantiye kurulumu ve mobilizasyon", False, date(2025,6,1), date(2025,7,5), date(2025,6,1), date(2025,7,5), 100, 2),
        ("2", "Hafriyat ve altyapı", False, date(2025,7,1), date(2026,3,31), date(2025,7,1), date(2026,3,20), 100, 10),
        ("3", "Kaba inşaat", False, date(2025,10,1), date(2026,12,31), date(2025,10,1), None, 72, 40),
        ("3.1", "Temel betonu", False, date(2025,10,1), date(2026,1,31), date(2025,10,1), date(2026,1,20), 100, 10),
        ("3.2", "1. kat kaba inşaat", False, date(2026,2,1), date(2026,5,31), date(2026,2,1), date(2026,6,9), 100, 12),
        ("3.3", "2. kat ve üst kat kaba inşaat", False, date(2026,6,1), date(2026,12,31), date(2026,6,10), None, 45, 18),
        ("4", "Çelik konstrüksiyon", False, date(2026,3,1), date(2026,9,30), date(2026,3,15), None, 40, 20),
        ("4.1", "Kolon-kiriş montajı", False, date(2026,3,1), date(2026,8,31), date(2026,3,15), None, 70, 12),
        ("4.2", "Çatı makas ve gerdirme", False, date(2026,9,1), date(2026,9,30), None, None, 0, 8),
        ("5", "Mekanik ve elektrik tesisat", False, date(2026,6,1), date(2027,6,30), date(2026,6,15), None, 15, 25),
        ("6", "Cephe ve dış kaplama", False, date(2027,1,1), date(2027,5,31), None, None, 0, 15),
        ("7", "Peyzaj ve dış saha", False, date(2027,5,1), date(2027,6,30), None, None, 0, 5),
        ("8", "Geçici kabul", True, date(2027,7,1), date(2027,7,1), None, None, 0, 0),
    ]
    for wc, name, is_ms, bs, bf, a_s, a_f, prog, wt in wbs:
        w(f"""INSERT INTO schedule_items (project_id, wbs_code, name, sort_order, is_milestone, baseline_start, baseline_finish,
    actual_start, actual_finish, progress_source, manual_progress, weight)
VALUES (v_proj, {q(wc)}, {q(name)}, {int(float(wc.split('.')[0]))*10 + (int(wc.split('.')[1]) if '.' in wc else 0)}, {str(is_ms).lower()},
    DATE '{bs}', DATE '{bf}', {('DATE ' + chr(39) + str(a_s) + chr(39)) if a_s else 'NULL'}, {('DATE ' + chr(39) + str(a_f) + chr(39)) if a_f else 'NULL'},
    'manual', {n(prog)}, {n(wt)});""")

    # ---------------------------------------------------- 12) GÜNLÜK SAHA RAPORLARI
    w("  -- 12) GÜNLÜK SAHA RAPORLARI (son 9 iş günü; taslak -> gönder)")
    report_days = [date(2026,10,27), date(2026,10,28), date(2026,10,29), date(2026,10,30),
                   date(2026,11,2), date(2026,11,3), date(2026,11,4), date(2026,11,5), date(2026,11,6)]
    kaba = next(s for s in SUBS if s["key"] == "kaba")
    celik = next(s for s in SUBS if s["key"] == "celik")
    mep = next(s for s in SUBS if s["key"] == "mep")
    rng = random.Random(77)
    conditions = ["Güneşli", "Parçalı bulutlu", "Parçalı bulutlu", "Yağışlı", "Güneşli", "Bulutlu", "Güneşli", "Rüzgarlı", "Güneşli"]
    for idx, d in enumerate(report_days):
        rid = uid()
        delay = []
        if conditions[idx] == "Rüzgarlı":
            delay = [{"reason_code": "Hava", "hours": 2.0, "note": "Rüzgâr 50 km/s üzeri, vinç çalışması durduruldu"}]
        if conditions[idx] == "Yağışlı":
            delay = [{"reason_code": "Hava", "hours": 3.5, "note": "Yağış nedeniyle beton dökümü ertelendi"}]
        w(f"""INSERT INTO daily_reports (id, project_id, report_date, revision_no, weather, temperature_min, temperature_max,
    notes, status, author_id, delay_causes, issue_flagged)
VALUES ('{rid}', v_proj, DATE '{d}', 1, '{{"condition": "{conditions[idx]}"}}'::jsonb, {rng.randint(6,12)}, {rng.randint(15,22)},
    {q('Kaba inşaat ve çelik montajı planlandığı gibi ilerledi.' if not delay else 'Hava koşulları nedeniyle kısmi çalışma yapıldı.')},
    'Draft', v_admin, '{json.dumps(delay, ensure_ascii=False)}'::jsonb, {str(bool(delay)).lower()});""")
        for sub, mh in ((kaba, rng.randint(44, 58)), (celik, rng.randint(16, 22) if d >= date(2026,3,15) else 0),
                        (mep, rng.randint(10, 15) if d >= date(2026,6,15) else 0)):
            if mh <= 0:
                continue
            w(f"INSERT INTO daily_manpower (daily_report_id, subcontractor_id, trade, headcount) VALUES ('{rid}', '{sub['id']}', {q(sub['trade'])}, {mh});")
        w(f"INSERT INTO daily_equipment (daily_report_id, equipment_name, count, working_hours, idle_reason) VALUES ('{rid}', 'Kule vinç', 2, {n(rng.choice([0, 9.0, 10.0]))}, NULL);")
        w(f"INSERT INTO daily_equipment (daily_report_id, equipment_name, count, working_hours, idle_reason) VALUES ('{rid}', 'Beton pompası', 1, {n(rng.choice([0, 6.0, 7.5]))}, NULL);")
        w(f"INSERT INTO daily_equipment (daily_report_id, equipment_name, count, working_hours, idle_reason) VALUES ('{rid}', 'Hidrolik ekskavatör', 1, {n(rng.choice([0, 0, 8.0]))}, NULL);")
        dk_wid = kaba["work_ids"][3 if idx % 2 else 2]  # donatı ya da döşeme
        qty = rng.randint(120, 220) if idx % 2 == 0 else rng.randint(8, 22)
        w(f"""INSERT INTO daily_work_entries (daily_report_id, work_item_id, location, description, qty, unit, discipline)
VALUES ('{rid}', '{dk_wid}', 'B Blok 2. kat', {q('Döşeme betonu dökümü' if idx % 2 == 0 else 'Nervürlü donatı montajı')}, {qty}, '{'m3' if idx % 2 == 0 else 'ton'}', 'Kaba inşaat');""")
        if d >= date(2026,6,15):
            w(f"""INSERT INTO daily_work_entries (daily_report_id, work_item_id, location, description, qty, unit, discipline)
VALUES ('{rid}', '{mep['work_ids'][1]}', 'A Blok tavan arası', 'Elektrik kablo kanalı çekimi', {rng.randint(300, 900)}, 'm', 'Elektrik');""")
        if d >= date(2026,3,15):
            w(f"""INSERT INTO daily_work_entries (daily_report_id, work_item_id, location, description, qty, unit, discipline)
VALUES ('{rid}', '{celik['work_ids'][0]}', 'Çelik kolon montajı - A aks', 'Kolon-kiriş montajı', {rng.randint(4, 9)}, 'ton', 'Çelik');""")
        # günlük rapor gönderildi (kilitlenmeden önce alt kayıtlar yazılır)
        w(f"""UPDATE daily_reports SET status='Submitted', submitted_at=TIMESTAMPTZ '{d} 18:00:00+03', submitted_by=v_admin WHERE id='{rid}';""")

    w("END $$;")

    # kontrol özeti stderr'e
    ev_fin = GROSS_TOTAL / CONTRACTS_TOTAL
    print(f"sözleşme toplamı (taşeron): {CONTRACTS_TOTAL:,.2f}", file=sys.stderr)
    print(f"fiziki (hakediş brütü / sözleşme): %{ev_fin*100:.2f}  -> EV = {GROSS_TOTAL*BAC/CONTRACTS_TOTAL:,.0f}", file=sys.stderr)
    print(f"idari hakediş toplamı: {IDARI_TARGET:,.0f}  -> finansal %{IDARI_TARGET/MAIN_CONTRACT*100:.2f}", file=sys.stderr)
    print(f"AC: hakediş gerçekleşen {ACTUAL_HAK:,.0f} + PO teslim {DELIVERED_TOTAL:,.0f} = {AC_TOTAL:,.0f}", file=sys.stderr)
    ev = GROSS_TOTAL * BAC / CONTRACTS_TOTAL
    print(f"CPI = EV/AC = {ev/AC_TOTAL:.3f}", file=sys.stderr)
    print(f"PV (2026-11 kümülatif) = %{pv_at_now} -> PV = {BAC*pv_at_now/100:,.0f}; SPI = {ev/(BAC*pv_at_now/100):.3f}", file=sys.stderr)
    print("\n".join(out))


if __name__ == "__main__":
    main()

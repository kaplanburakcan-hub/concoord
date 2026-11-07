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
for s_ in SUBS:
    s_["start_date"] = date(s_["start"][0], s_["start"][1], 1)

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
    idari.append(dict(id=uid(), no=k + 1, y=y, m=m, tutar=tutar, prev=round(running - tutar, 2),
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
PO_REF = []  # (po_id, desc, sup, amt, teslim_tarihi) — ödeme ve stok kayıtları için
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
    w("  v_admin uuid; v_role uuid; v_proj uuid; v_ce uuid;")
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
        w(f"""INSERT INTO idari_hakedisler (id, project_id, donem_no, aciklama, tutar, kdv_pct, fatura_no, gelen_odeme_tarihi,
        created_by, hakedis_tarihi, onceki_hakedis_toplami, kesintiler)
    VALUES ('{d['id']}', v_proj, {d['no']}, {q(aciklama)}, {n(d['tutar'])}, 20.00,
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
        PO_REF.append((po_id, desc, sup, amt, dd))
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

    # ---------------------------------------------------- 12) GÜNLÜK SAHA RAPORLARI (tüm hafta içi iş günleri)
    w("  -- 12) GÜNLÜK SAHA RAPORLARI (2025-06-02 .. 2026-11-06, Pzt-Cum)")
    rng = random.Random(77)
    kaba = next(s for s in SUBS if s["key"] == "kaba")
    celik = next(s for s in SUBS if s["key"] == "celik")
    mep = next(s for s in SUBS if s["key"] == "mep")
    toprak = next(s for s in SUBS if s["key"] == "toprak")

    work_days = []
    d0 = date(2025, 6, 2)
    while d0 <= date(2026, 11, 6):
        if d0.weekday() < 5:
            work_days.append(d0)
        d0 += timedelta(days=1)

    def mi_of(day):
        return (day.year - 2025) * 12 + (day.month - 6)

    # Ay bazında iş kalemi miktarı (hakediş ayından; hakediş yoksa önceki ayın hızı)
    month_qty = {}
    for r in progress_rows:
        for it in r["items"]:
            month_qty[(r["sub"]["key"], r["mi"], it["wid"])] = it["this_qty"]

    month_days = {}
    for day in work_days:
        month_days.setdefault(mi_of(day), []).append(day)

    # Her ay-iş kalemi miktarını o ayın iş günlerine rastgele böl
    day_entries = {}
    for s in SUBS:
        for wi, wid in enumerate(s["work_ids"]):
            for mi, days in month_days.items():
                src_mi = mi if (s["key"], mi, wid) in month_qty else mi - 1
                q_total = month_qty.get((s["key"], src_mi, wid), 0.0)
                if q_total <= 0:
                    continue
                active = [x for x in days if x >= s["start_date"]]
                if not active:
                    continue
                weights = [rng.uniform(0.6, 1.4) for _ in active]
                wsum = sum(weights)
                left = round(q_total, 2)
                for k, day in enumerate(active):
                    part = round(left * weights[k] / wsum, 2) if k < len(active) - 1 else round(left, 2)
                    left = round(left - part, 2) if k < len(active) - 1 else 0.0
                    if part > 0:
                        day_entries.setdefault(day, []).append((s, wi, wid, part))

    CONCRETE_IDX = {"kaba"}
    for idx, day in enumerate(work_days):
        winter = day.month in (12, 1, 2, 3)
        summer = day.month in (6, 7, 8, 9)
        roll = rng.random()
        rain = roll < (0.12 if winter else 0.06)
        wind = (not rain) and rng.random() < 0.04
        if rain:
            condition = "Yağışlı"
        elif wind:
            condition = "Rüzgarlı"
        else:
            condition = rng.choice(["Güneşli", "Güneşli", "Parçalı bulutlu", "Bulutlu"])
        t_min = rng.randint(-1, 5) if winter else (rng.randint(17, 23) if summer else rng.randint(7, 14))
        t_max = rng.randint(6, 12) if winter else (rng.randint(28, 35) if summer else rng.randint(15, 22))
        delay = []
        if rain:
            delay = [{"reason_code": "Hava", "hours": 3.5, "note": "Yağış nedeniyle beton dökümü ve yüksekte çalışma durduruldu"}]
        elif wind:
            delay = [{"reason_code": "Hava", "hours": 2.0, "note": "Rüzgâr 50 km/s üzeri, vinç çalışması durduruldu"}]
        rid = uid()
        notes = "Hava koşulları nedeniyle kısmi çalışma yapıldı." if delay else rng.choice(
            ["Kaba inşaat ve montaj planlandığı gibi ilerledi.", "İmalatlar planlanan sırada sürdü.",
             "Taşeron ekipleri sahada, kalıp ve donatı işleri devam ediyor."])
        w(f"""INSERT INTO daily_reports (id, project_id, report_date, revision_no, weather, temperature_min, temperature_max,
    notes, status, author_id, delay_causes, issue_flagged)
VALUES ('{rid}', v_proj, DATE '{day}', 1, '{{"condition": "{condition}"}}'::jsonb, {t_min}, {t_max},
    {q(notes)}, 'Draft', v_admin, '{json.dumps(delay, ensure_ascii=False)}'::jsonb, {str(bool(delay)).lower()});""")

        # Personel (taşeron ekipleri, aktif olduğu günlerde)
        for sub, lo, hi, start in ((toprak, 24, 32, date(2025, 7, 1)), (kaba, 20, 46, date(2025, 10, 1)),
                                   (celik, 12, 18, date(2026, 3, 1)), (mep, 8, 13, date(2026, 6, 1))):
            if day < start or (sub is toprak and day > date(2026, 3, 31)):
                continue
            if rain and sub is not toprak:
                hc = max(4, rng.randint(lo // 2, hi // 2))
            else:
                hc = rng.randint(lo, hi)
            w(f"INSERT INTO daily_manpower (daily_report_id, subcontractor_id, trade, headcount) VALUES ('{rid}', '{sub['id']}', {q(sub['trade'])}, {hc});")

        # Ekipman
        if day >= date(2025, 10, 1):
            w(f"INSERT INTO daily_equipment (daily_report_id, equipment_name, count, working_hours, idle_reason) VALUES ('{rid}', 'Kule vinç', 2, {0 if (rain or wind) else rng.choice([9.0, 10.0])}, {q('Hava (rüzgâr)') if wind else 'NULL'});")
        if day >= date(2025, 10, 1) and day.weekday() in (0, 2, 4):
            w(f"INSERT INTO daily_equipment (daily_report_id, equipment_name, count, working_hours, idle_reason) VALUES ('{rid}', 'Beton pompası', 1, {0 if rain else rng.choice([6.5, 7.5])}, {q('Yağış') if rain else 'NULL'});")
        if day <= date(2026, 3, 31) and day >= date(2025, 7, 1):
            w(f"INSERT INTO daily_equipment (daily_report_id, equipment_name, count, working_hours, idle_reason) VALUES ('{rid}', 'Hidrolik ekskavatör', 1, {0 if rain else 8.0}, NULL);")
            w(f"INSERT INTO daily_equipment (daily_report_id, equipment_name, count, working_hours, idle_reason) VALUES ('{rid}', 'Damper kamyon', 2, {0 if rain else 9.0}, NULL);")

        # Personel devamı
        durum_burak = "izinli" if rng.random() < 0.08 else "mevcut"
        durum_demir = "izinli" if idx % 7 == 6 else "mevcut"
        for grup, ad, unvan, durum in (
            ("yuklenici", "Emre Kılıç", "Şantiye Şefi", "mevcut"),
            ("yuklenici", "Deniz Aydın", "İSG Uzmanı", "mevcut"),
            ("yuklenici", "Burak Yılmaz", "Saha Mühendisi", durum_burak),
            ("musavir", "S. Demir", "Kontrol Mühendisi", durum_demir),
        ):
            w(f"INSERT INTO daily_staff_attendance (daily_report_id, grup, ad_soyad, unvan, durum) VALUES ('{rid}', '{grup}', {q(ad)}, {q(unvan)}, '{durum}');")

        # İmalat kayıtları (aylık miktarın günlere bölünmüş hali; yağışlı günlerde beton ve yükseklik işi yok)
        for s, wi, wid, part in day_entries.get(day, []):
            if rain and s["key"] in CONCRETE_IDX:
                continue
            poz, desc, unit, _, _, _ = s["items"][wi]
            loc = {"toprak": "Hafriyat sahası", "kaba": "B Blok " + str(1 + (idx // 40) % 3) + ". kat",
                   "celik": "Çelik kolon montajı - A aks", "mep": "A Blok tavan arası"}[s["key"]]
            w(f"""INSERT INTO daily_work_entries (daily_report_id, work_item_id, location, description, qty, unit, discipline)
VALUES ('{rid}', '{wid}', {q(loc)}, {q(desc)}, {n(part)}, {q(unit)}, {q(s['trade'])});""")

        # Küçük nakit harcamaları (günlerin ~%30'u)
        if rng.random() < 0.3:
            for k in range(rng.randint(1, 2)):
                cat = rng.choice(["Yakıt", "Küçük malzeme", "Yemek", "Ulaşım", "Ofis"])
                desc = {"Yakıt": "Mazot ikmali (jeneratör/ekskavatör)", "Küçük malzeme": "Bağlama teli ve sarf malzeme",
                        "Yemek": "Ekip öğle yemeği", "Ulaşım": "Personel servis ücreti", "Ofis": "Ofis kırtasiye ve sarf"}[cat]
                amt = {"Yakıt": rng.randint(2500, 6500), "Küçük malzeme": rng.randint(400, 2200),
                       "Yemek": rng.randint(900, 2400), "Ulaşım": rng.randint(600, 1500), "Ofis": rng.randint(150, 600)}[cat]
                w(f"""INSERT INTO daily_cash_expenses (daily_report_id, description, category, amount, receipt_no)
VALUES ('{rid}', {q(desc)}, {q(cat)}, {amt}, {q(f'FIS-{idx:03d}-{k + 1}')});""")

        w(f"""UPDATE daily_reports SET status='Submitted', submitted_at=TIMESTAMPTZ '{day} 18:00:00+03', submitted_by=v_admin WHERE id='{rid}';""")

    # ---------------------------------------------------- 13) RİSKLER
    w("  -- 13) RİSK KAYDI (olasılık x etki 1-5)")
    risks = [
        ("Çelik çatı montajında hava koşulları", "Rüzgâr ve yağış nedeniyle vinç çalışma süresi kısalabilir.", 4, 4, "İzleniyor"),
        ("Hazır beton tedarik gecikmesi", "Santral bakımı nedeniyle döşeme dökümlerinde plan dışı bekleme.", 3, 3, "Açık"),
        ("Trafo ve ana pano devreye alma", "Enerji dağıtım şirketinin bağlantı onayı gecikebilir.", 3, 5, "Açık"),
        ("Kur artışı (çelik ve ekipman)", "İthal ekipman ve çelik fiyatlarında artış maliyeti etkileyebilir.", 3, 4, "İzleniyor"),
        ("Kazı sırasında yeraltı suyu", "Hafriyat tamamlandı; ana risk kapandı.", 2, 3, "Kapandı"),
        ("Cephe panel montajında vinç kapasitesi", "Dış cephe için ek vinç ihtiyacı olabilir.", 2, 3, "Açık"),
    ]
    for i, (title, desc, pr, im, st) in enumerate(risks, 1):
        w(f"""INSERT INTO project_risks (project_id, risk_no, title, description, probability, impact, status, created_by)
VALUES (v_proj, {i}, {q(title)}, {q(desc)}, {pr}, {im}, {q(st)}, v_admin);""")

    # ---------------------------------------------------- 14) İSG BULGULARI
    w("  -- 14) İSG BULGULARI")
    findings = [
        (kaba, "Major", "Kolon kalıp iskelesinde korkuluk eksik", "B Blok 1. kat", "Closed", date(2026,9,20), date(2026,9,22), "Korkuluk tamamlandı, fotoğraf arşivlendi."),
        (celik, "Major", "Çelik montajında emniyet kemeri kullanılmıyor", "Çelik kolon montajı - A aks", "InProgress", date(2026,11,15), None, None),
        (mep, "Minor", "Kablo kanalı önünde malzeme yığını", "A Blok tavan arası", "Open", date(2026,11,20), None, None),
        (kaba, "Observation", "Beton pompası hortumu için bağlantı noktası işaretlenmeli", "B Blok dış cephe", "Open", date(2026,11,30), None, None),
    ]
    for sub, sev, desc, loc, st, due, closed, note in findings:
        cl_by = "v_admin" if closed else "NULL"
        cl_at = ("TIMESTAMPTZ '" + str(closed) + " 15:00:00+03'") if closed else "NULL"
        w(f"""INSERT INTO ohs_findings (project_id, subcontractor_id, severity, description, location, due_date, status, reported_by, closed_by, closed_at, close_note)
VALUES (v_proj, '{sub['id']}', '{sev}', {q(desc)}, {q(loc)}, DATE '{due}', '{st}', v_admin, {cl_by}, {cl_at}, {q(note)});""")

    # ---------------------------------------------------- 15) GÖREVLER
    w("  -- 15) GÖREVLER (kanban)")
    tasks = [
        ("Çelik montaj planı revizyonu", "Kuzey açıklık için montaj sırası güncellenecek.", "Done", "High", date(2026,10,25), "toplanti"),
        ("Trafo bağlantı başvurusunu tamamla", "Enerji dağıtım şirketine eksik evrakları gönder.", "InProgress", "Urgent", date(2026,11,10), "elle"),
        ("Cephe panel teknik çizimlerini onayla", "Mimari onay bekleniyor.", "Review", "Normal", date(2026,11,14), "elle"),
        ("İSG: çelik montajı için emniyet kemeri eğitimi", "Çelik ekibine toplu eğitim.", "Todo", "Urgent", date(2026,11,12), "isg"),
        ("Kablo kanalı önü malzeme düzenlemesi", "Mep taşeronu ile koordinasyon.", "Todo", "Normal", date(2026,11,20), "isg"),
        ("Hazır beton tedarik sözleşmesi güncellemesi", "Yedek santral için ek teklif iste.", "Backlog", "Normal", date(2026,11,30), "elle"),
        ("Haftalık ilerleme raporunu paylaş", "Müşteriye PDF gönder.", "Todo", "Low", date(2026,11,8), "elle"),
        ("Kaba inşaat 3. kat kalıp planı", "Kalıp ekibi için iş planı.", "Backlog", "Normal", date(2026,12,5), "elle"),
    ]
    for i, (title, desc, st, pr, due, src) in enumerate(tasks, 1):
        tid = uid()
        w(f"""INSERT INTO tasks (id, project_id, title, description, status, priority, due_date, created_by, kanban_order, source_module)
VALUES ('{tid}', v_proj, {q(title)}, {q(desc)}, '{st}', '{pr}', DATE '{due}', v_admin, {i * 1000}, '{src}');""")
        w(f"INSERT INTO task_assignees (task_id, user_id) VALUES ('{tid}', v_admin);")

    # ---------------------------------------------------- 16) TOPLANTILAR
    w("  -- 16) TOPLANTILAR")
    meetings = [
        ("TOP-001", "Proje kick-off toplantısı", "kickoff", date(2025,6,5), "gerceklesti", "Proje kapsamı, takvim ve sorumluluk matrisi görüşüldü.", "Kapsam onaylandı; haftalık ilerleme toplantısı kararlaştırıldı."),
        ("TOP-014", "Haftalık ilerleme toplantısı", "ilerleme", date(2026,10,27), "gerceklesti", "Çelik montaj durumu, trafo başvurusu, cephe onayları.", "Çelik montaj planı revize edilecek; trafo başvurusu 10 Kasım'a kadar tamamlanacak."),
        ("TOP-015", "Çelik montaj saha toplantısı", "saha", date(2026,11,12), "planli", "İSG önlemleri ve kuzey açıklık montaj sırası.", None),
        ("TOP-016", "Taşeron koordinasyon toplantısı", "taseron", date(2026,11,20), "planli", "Kablo kanalı ve kalıp iş planı koordinasyonu.", None),
    ]
    for no, title, tur, dt_, dur, gund, karar in meetings:
        w(f"""INSERT INTO project_meetings (project_id, toplanti_no, baslik, toplanti_turu, tarih, lokasyon, katilimcilar, gundem, kararlar, aksiyon_maddeleri, durum)
VALUES (v_proj, {q(no)}, {q(title)}, '{tur}', DATE '{dt_}', 'Şantiye ofisi', ARRAY['Emre Kılıç (Yüklenici)', 'S. Demir (Müşavir)'], {q(gund)}, {q(karar)}, '[]'::jsonb, '{dur}');""")

    # ---------------------------------------------------- 17) MAKİNE / EKİPMAN
    w("  -- 17) MAKİNE VE EKİPMAN (şirket envanteri + projeye atama + çalışma kaydı)")
    machines = [
        ("is_makinesi", "Kule vinç (50 m)", "ozmal", "aktif", 12500, "KVN-01", "Liebherr", "100 EC-H", 2019),
        ("is_makinesi", "Mobil vinç 60 ton", "kiralik", "aktif", 9800, "MVN-02", "Tadano", "GR-600", 2021),
        ("is_makinesi", "Hidrolik ekskavatör 22 ton", "ozmal", "aktif", 5200, "EKS-03", "Komatsu", "PC220", 2018),
        ("is_makinesi", "Beton pompası 37 m", "kiralik", "bakim", 7800, "PMP-04", "Putzmeister", "M37", 2020),
        ("arac", "Kamyonet (Pickup)", "kiralik", "aktif", 2100, "34 DM 505", "Ford", "Ranger", 2023),
    ]
    rng2 = random.Random(9)
    for i, (tip, ad, sahiplik, durum, ucret, dno, marka, model, yil) in enumerate(machines, 1):
        ceid = uid()
        atama = date(2026, 1, 15) if i < 5 else date(2026, 3, 1)
        plaka = q(dno) if tip == 'arac' else 'NULL'
        tedarikci = q('Demo Makine Kiralama Ltd.') if sahiplik == 'kiralik' else 'NULL'
        w(f"""SELECT id INTO v_ce FROM company_equipment WHERE demirbas_no = {q(dno)} LIMIT 1;
  IF v_ce IS NULL THEN
    INSERT INTO company_equipment (id, tip, ad, plaka, demirbas_no, marka, model, uretim_yili, sahiplik, tedarikci, gunluk_ucret, durum, aciklama, current_project_id)
    VALUES ('{ceid}', '{tip}', {q(ad)}, {plaka}, {q(dno)}, {q(marka)}, {q(model)}, {yil}, '{sahiplik}', {tedarikci}, {ucret}, '{durum}', 'Demo kaydı', v_proj)
    RETURNING id INTO v_ce;
  END IF;""")
        mid = uid()
        w(f"""INSERT INTO project_machines (id, project_id, company_equipment_id, atanma_tarihi, is_basi_tarihi, teslim_alindi_tarihi)
VALUES ('{mid}', v_proj, v_ce, DATE '{atama}', DATE '{atama}', DATE '{atama}');""")
        for dd in [date(2026,10,27), date(2026,10,29), date(2026,11,3), date(2026,11,5)]:
            if dd < atama:
                continue
            hrs = rng2.choice([6.5, 8.0, 9.0]) if tip != 'arac' else rng2.choice([40.0, 65.0, 80.0])
            birim = 'km' if tip == 'arac' else 'saat'
            w(f"""INSERT INTO project_machine_logs (project_id, machine_id, tarih, calisma_miktari, calisma_birimi, operator, aciklama)
VALUES (v_proj, '{mid}', DATE '{dd}', {hrs}, '{birim}', 'Demo operatör', 'Günlük çalışma kaydı');""")

    # ---------------------------------------------------- 18) İDARİ HAKEDİŞ TAHSİLATLARI (nakit girişi)
    w("  -- 18) İDARİ HAKEDİŞ TAHSİLATLARI")
    for dd_ in idari:
        if not dd_["paid"]:
            continue
        hd_ = month_end(dd_["y"], dd_["m"])
        w(f"""INSERT INTO cash_events (project_id, direction, source_entity, source_id, description, amount, event_date, created_by)
VALUES (v_proj, 'in', 'idari_hakedis', '{dd_['id']}', {q('İdari Hakediş — ' + str(dd_['y']) + '-' + f"{dd_['m']:02d}")}, {n(dd_['tutar'])}, DATE '{hd_ + timedelta(days=30)}', v_admin);""")

    # ---------------------------------------------------- 19) TAŞERON ÖDEMELERİ (havale + nakit çıkışı)
    w("  -- 19) TAŞERON ÖDEMELERİ (kesinleşen hakediş net tutarı, +20 gün)")
    for r in progress_rows:
        fin_d = date(r["y"] + (1 if r["m"] == 12 else 0), 1 if r["m"] == 12 else r["m"] + 1, 5)
        pay_d = fin_d + timedelta(days=20)
        if pay_d > date(2026, 11, 6):
            continue
        net = round(r["gross_this"] - round(r["gross_this"] * 0.05, 2), 2)
        did = uid()
        desc = r["sub"]["name"] + " hakedişi " + str(r["y"]) + "-" + f"{r['m']:02d}"
        w(f"""INSERT INTO progress_payment_disbursements (id, progress_payment_id, amount, payment_method, event_date, created_by)
VALUES ('{did}', '{r['id']}', {n(net)}, 'havale', DATE '{pay_d}', v_admin);""")
        w(f"""INSERT INTO cash_events (project_id, direction, source_entity, source_id, description, amount, event_date, payment_method, created_by)
VALUES (v_proj, 'out', 'progress_payment_disbursement', '{did}', {q(desc)}, {n(net)}, DATE '{pay_d}', 'havale', v_admin);""")

    # ---------------------------------------------------- 20) TEDARİKÇİ ÖDEMELERİ (teslim + 30 gün)
    w("  -- 20) TEDARİKÇİ ÖDEMELERİ (teslim tarihi + 30 gün)")
    for po_id, desc, sup, amt, dd_ in PO_REF:
        pay_d = dd_ + timedelta(days=30)
        pyid = uid()
        w(f"""INSERT INTO purchase_order_payments (id, po_id, amount, payment_method, event_date, created_by)
VALUES ('{pyid}', '{po_id}', {n(amt)}, 'havale', DATE '{pay_d}', v_admin);""")
        w(f"""INSERT INTO cash_events (project_id, direction, source_entity, source_id, description, amount, event_date, payment_method, created_by)
VALUES (v_proj, 'out', 'po_payment', '{pyid}', {q(desc + ' — ' + sup)}, {n(amt)}, DATE '{pay_d}', 'havale', v_admin);""")

    # ---------------------------------------------------- 21) TEDARİKÇİ KAYITLARI VE EKSTRELER
    w("  -- 21) TEDARİKÇİLER VE EKSTRELER")
    suppliers = {}
    for po_id, desc, sup, amt, dd_ in PO_REF:
        suppliers.setdefault(sup, {"toplam": 0.0, "odenen": 0.0})
        suppliers[sup]["toplam"] += amt
        suppliers[sup]["odenen"] += amt
    for desc, sup, amt, code, dd_ in PO_OPEN:
        suppliers.setdefault(sup, {"toplam": 0.0, "odenen": 0.0})
        suppliers[sup]["toplam"] += amt
    for i, (sup, vals) in enumerate(suppliers.items(), 1):
        tid_ = uid()
        w(f"""INSERT INTO tedarikciler (id, project_id, company_name, tax_no, contact_person, phone, email, trade, default_payment_method)
VALUES ('{tid_}', v_proj, {q(sup)}, {q(str(6100000000 + i * 137))}, 'Satış Müdürü (Demo)', '0212 555 0{i:02d} {i:02d}', 'satis@{sup.split()[0].lower()}.demo', 'Tedarik', 'havale');""")
        durum = "odendi" if abs(vals["odenen"] - vals["toplam"]) < 0.01 and vals["toplam"] > 0 else (
            "kismi_odendi" if vals["odenen"] > 0 else "bekliyor")
        w(f"""INSERT INTO supplier_statements (project_id, tedarikci_adi, ekstre_no, ekstre_tarihi, vade_tarihi, toplam_tutar, odenen_tutar, para_birimi, odeme_durumu, aciklama)
VALUES (v_proj, {q(sup)}, 'EKS-2026-{i:03d}', DATE '2026-10-31', DATE '2026-11-30', {n(vals['toplam'])}, {n(vals['odenen'])}, 'TRY', '{durum}', 'Ekim 2026 dönemi ekstresi (demo)');""")

    # ---------------------------------------------------- 22) SABİT GİDERLER
    w("  -- 22) SABİT GİDERLER (her ay tekrarlanan; okuma anında üretilir)")
    fixed = [
        ("Şantiye ofis kirası", 45000, "kira", 5),
        ("Güvenlik hizmeti", 180000, "hizmet", 1),
        ("Temizlik hizmeti", 25000, "hizmet", 10),
        ("Ofis internet ve telefon", 8500, "iletisim", 15),
    ]
    for label, amt, cat, dom in fixed:
        w(f"""INSERT INTO fixed_expenses (project_id, label, amount, category, expense_day_of_month, start_date, active, created_by)
VALUES (v_proj, {q(label)}, {amt}, {q(cat)}, {dom}, DATE '2025-06-01', true, v_admin);""")

    # ---------------------------------------------------- 23) TEDARİK PLANI
    w("  -- 23) SATINALMA PLANI (kalem bazlı)")
    plan = [
        ("04.110", "Çelik profil (HEA/IPE)", "Yapısal çelik", 980, "ton", "Mavi Çelik Tic. Ltd.", date(2026,1,20), date(2026,2,20), "Kritik", "TeslimAlindi"),
        ("05.210", "Trafo ve ana pano", "Elektrik", 1, "ad", "Enerji Ekipman A.Ş.", date(2026,4,20), date(2026,5,15), "Kritik", "TeslimAlindi"),
        ("06.400", "Jeneratör ve UPS", "Elektrik", 1, "ad", "Enerji Ekipman A.Ş.", date(2026,6,1), date(2026,7,2), "Normal", "TeslimAlindi"),
        ("07.120", "Cephe panel kaplama", "Dış cephe", 6200, "m2", "Yapı Kaplama A.Ş.", date(2026,8,10), date(2026,9,12), "Kritik", "TeslimAlindi"),
        ("07.310", "Cephe alüminyum doğrama", "Dış cephe", 1, "takım", "Alüm Doğrama Sanayi", date(2026,11,1), date(2026,12,15), "Normal", "SiparisVerildi"),
        ("05.500", "Hidrolik asansör", "Mekanik", 2, "adet", "Asansör Dünyası Ltd.", date(2026,8,20), date(2026,10,5), "Kritik", "TeslimAlindi"),
        ("06.520", "Güvenlik ve CCTV", "Elektrik", 1, "sistem", "Güven Güvenlik Sist.", date(2026,10,25), date(2026,11,30), "Normal", "SiparisVerildi"),
        ("03.300", "Hazır beton C30/37 (yedek santral)", "Beton", 1200, "m3", "Akdeniz Hazır Beton", date(2026,11,10), date(2026,11,25), "Kritik", "Planlandi"),
        ("08.210", "Asfalt karışımı (yol üst tabaka)", "Yol", 480, "ton", "Sakarya Asfalt A.Ş.", date(2027,4,1), date(2027,4,20), "Normal", "Planlandi"),
        ("05.120", "Yangın söndürme sprinkler", "Mekanik", 1, "sistem", "Yangın Sistemleri Ltd.", date(2026,5,1), date(2026,6,18), "Kritik", "TeslimAlindi"),
        ("06.110", "Aydınlatma armatürü LED", "Elektrik", 1200, "adet", "Işık Aydınlatma Tic.", date(2026,9,1), date(2026,10,30), "Normal", "Gecikti"),
        ("01.050", "Şantiye konteyneri", "Genel", 4, "adet", "Saha Lojistik Ltd.", date(2025,7,20), date(2025,8,5), "Normal", "TeslimAlindi"),
    ]
    for code_, desc_, cat_, qty_, unit_, sup_, od, dd_, crit, st in plan:
        w(f"""INSERT INTO procurement_plan_items (project_id, poz_no, description, category, quantity, unit, supplier_name, planned_order_date, planned_delivery_date, criticality, status, created_by)
VALUES (v_proj, {q(code_)}, {q(desc_)}, {q(cat_)}, {qty_}, {q(unit_)}, {q(sup_)}, DATE '{od}', DATE '{dd_}', {q(crit)}, '{st}', v_admin);""")

    # ---------------------------------------------------- 24) MALZEME ONAYLARI
    w("  -- 24) MALZEME ONAYLARI (taşeron talebi, müşavir kararı)")
    approvals = [
        (kaba, "MAR-001", "C30/37 hazır beton (reçete)", "Akdeniz Hazır Beton", "Approved", "Reçete uygun, numune testi geçti.", 3, date(2025,10,20)),
        (kaba, "MAR-004", "B500C nervürlü donatı", "Kardemir (demo)", "Approved", "TS 708 belgesi uygun.", 7, date(2026,1,10)),
        (celik, "MAR-007", "Yüksek mukavemetli cıvata 10.9", "Bulonsan (demo)", "ConditionallyApproved", "Sertlik raporu eklenmesi şartıyla onaylandı.", 14, date(2026,3,5)),
        (mep, "MAR-011", "PPR boru sistemi", "Tesisat Malzeme Ltd. (demo)", "Rejected", "Basınç sınıfı talep edilenin altında, yeniden teklif istendi.", 10, date(2026,6,20)),
        (mep, "MAR-013", "LED armatür 40W", "Işık Aydınlatma Tic.", "UnderReview", None, 21, date(2026,10,28)),
    ]
    for sub_, mar, mname, manu, st, note, lead, need in approvals:
        dec_by = "NULL" if st in ("Submitted", "UnderReview") else "v_admin"
        dec_at = "NULL" if st in ("Submitted", "UnderReview") else f"TIMESTAMPTZ '{need} 16:00:00+03'"
        w(f"""INSERT INTO material_approvals (project_id, subcontractor_id, mar_no, material_name, manufacturer, status, decision_note, decided_by, decided_at, created_by, lead_time_days, needed_by_date)
VALUES (v_proj, '{sub_['id']}', {q(mar)}, {q(mname)}, {q(manu)}, '{st}', {q(note) if note else 'NULL'}, {dec_by}, {dec_at}, v_admin, {lead}, DATE '{need}');""")

    # ---------------------------------------------------- 25) ŞANTİYE DEPOSU
    w("  -- 25) ŞANTİYE DEPOSU (stok kalemleri ve hareketler)")
    wh = [
        ("Donatı B500C (çap 12-20)", "Donatı", "ton", 42.5, 10),
        ("CEM I 42.5 çimento", "Bağlayıcı", "ton", 18.0, 6),
        ("Kaynak teli", "Sarf", "kg", 240, 50),
        ("Mazot", "Yakıt", "litre", 1850, 500),
        ("Bağlama teli", "Sarf", "kg", 65, 20),
        ("Çelik profil (HEA)", "Yapısal", "ton", 0, 2),
    ]
    wh_ids = {}
    for i, (nm, cat_, un, cur, mn) in enumerate(wh, 1):
        wid_ = uid()
        wh_ids[nm] = wid_
        w(f"""INSERT INTO site_warehouse_items (id, project_id, malzeme_adi, kategori, birim, mevcut_miktar, min_stok, sira)
VALUES ('{wid_}', v_proj, {q(nm)}, {q(cat_)}, {q(un)}, {cur}, {mn}, {i});""")
    celik_po = next((po_id for po_id, desc, sup, amt, dd_ in PO_REF if desc.startswith("Çelik profil")), None)
    if celik_po:
        w(f"""INSERT INTO site_warehouse_movements (project_id, item_id, hareket_turu, miktar, tarih, belge_no, aciklama, purchase_order_id)
VALUES (v_proj, '{wh_ids['Çelik profil (HEA)']}', 'giris', 980, DATE '2026-02-20', 'IRS-0001', 'Çelik profil teslim girişi', '{celik_po}');""")
        w(f"""INSERT INTO site_warehouse_movements (project_id, item_id, hareket_turu, miktar, tarih, belge_no, aciklama)
VALUES (v_proj, '{wh_ids['Çelik profil (HEA)']}', 'cikis', 980, DATE '2026-03-01', 'CKS-0001', 'Kolon montajı için sahaya çıkış');""")
    for k, day in enumerate([d for d in work_days if d.day in (5, 20) and d >= date(2025, 8, 1)][:24]):
        w(f"""INSERT INTO site_warehouse_movements (project_id, item_id, hareket_turu, miktar, tarih, belge_no, aciklama)
VALUES (v_proj, '{wh_ids['Mazot']}', 'cikis', {rng.randint(180, 420)}, DATE '{day}', 'CKS-M{k + 1:03d}', 'Jeneratör ve ekskavatör ikmali');""")
    w(f"""INSERT INTO site_warehouse_movements (project_id, item_id, hareket_turu, miktar, tarih, belge_no, aciklama)
VALUES (v_proj, '{wh_ids['Donatı B500C (çap 12-20)']}', 'giris', 60, DATE '2026-01-08', 'IRS-D01', 'Donatı teslim girişi (Kardemir)');""")
    w(f"""INSERT INTO site_warehouse_movements (project_id, item_id, hareket_turu, miktar, tarih, belge_no, aciklama)
VALUES (v_proj, '{wh_ids['CEM I 42.5 çimento']}', 'giris', 40, DATE '2026-02-10', 'IRS-C01', 'Çimento teslim girişi');""")

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

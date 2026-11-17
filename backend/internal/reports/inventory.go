package reports

// Malzeme - Envanter raporları: Depo-Stok ve Makine-Ekipman. Veriler canlı
// tablolardan okunur (site_warehouse_*, saha_tutanaklari, company_equipment,
// project_machines, project_machine_logs, equipment_transfer_requests).
// İzin: reports.view. ?download=1 verilirse PDF ek olarak indirilir.

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/ipks/ipks/backend/internal/httpx"
	"github.com/ipks/ipks/backend/internal/reportpdf"
)

// periodParams — ?from=&to= (YYYY-MM-DD). Varsayılan: ayın başı – bugün.
func periodParams(w http.ResponseWriter, r *http.Request, today time.Time) (time.Time, time.Time, bool) {
	from := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)
	to := today
	q := r.URL.Query()
	if s := q.Get("from"); s != "" {
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			httpx.ValidationFailed(w, r, map[string]string{"from": "YYYY-AA-GG biçiminde olmalı"})
			return from, to, false
		}
		from = t
	}
	if s := q.Get("to"); s != "" {
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			httpx.ValidationFailed(w, r, map[string]string{"to": "YYYY-AA-GG biçiminde olmalı"})
			return from, to, false
		}
		to = t
	}
	if from.After(to) {
		httpx.ValidationFailed(w, r, map[string]string{"from": "başlangıç bitişten sonra olamaz"})
		return from, to, false
	}
	return from, to, true
}

func (h *Handler) projectInfo(w http.ResponseWriter, r *http.Request, pid uuid.UUID) (string, string, bool) {
	var ad, kod string
	if err := h.pool.QueryRow(r.Context(),
		`SELECT name, code FROM projects WHERE id=$1 AND deleted_at IS NULL`, pid).Scan(&ad, &kod); err != nil {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Proje bulunamadı.", nil)
		return "", "", false
	}
	return ad, kod, true
}

func writeInventoryPDF(w http.ResponseWriter, r *http.Request, pdf []byte, name string) {
	disp := "inline"
	if r.URL.Query().Get("download") == "1" {
		disp = "attachment"
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`%s; filename="%s.pdf"`, disp, name))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pdf)
}

var hareketTurEtiket = map[string]string{
	"giris": "Giriş", "cikis": "Çıkış", "iade": "İade", "sayim": "Sayım",
}

// InventoryDepoStokPDF — GET /projects/{projectID}/inventory/depo-stok.pdf
func (h *Handler) InventoryDepoStokPDF(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	ad, kod, ok := h.projectInfo(w, r, pid)
	if !ok {
		return
	}
	today := truncDay(time.Now())
	from, to, ok := periodParams(w, r, today)
	if !ok {
		return
	}
	data, err := h.depoStokData(r.Context(), pid, from, to, today)
	if err != nil {
		h.log.Error("depo-stok raporu", "err", err)
		httpx.Internal(w, r)
		return
	}
	data.ProjeAd, data.ProjeKod = ad, kod
	writeInventoryPDF(w, r, reportpdf.BuildDepoStok(data), "depo-stok-"+kod)
}

// InventoryMakineEkipmanPDF — GET /projects/{projectID}/inventory/makine-ekipman.pdf
func (h *Handler) InventoryMakineEkipmanPDF(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	ad, kod, ok := h.projectInfo(w, r, pid)
	if !ok {
		return
	}
	today := truncDay(time.Now())
	from, to, ok := periodParams(w, r, today)
	if !ok {
		return
	}
	data, err := h.makineEkipmanData(r.Context(), pid, from, to, today)
	if err != nil {
		h.log.Error("makine-ekipman raporu", "err", err)
		httpx.Internal(w, r)
		return
	}
	data.ProjeAd, data.ProjeKod = ad, kod
	writeInventoryPDF(w, r, reportpdf.BuildMakineEkipman(data), "makine-ekipman-"+kod)
}

func truncDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// depoStokData — stok bakiyesi, kapsama, dönem hareketleri ve açık zimmetler.
func (h *Handler) depoStokData(ctx context.Context, pid uuid.UUID, from, to, today time.Time) (reportpdf.DepoStokData, error) {
	out := reportpdf.DepoStokData{Baslangic: from, Bitis: to, RaporTarihi: today, TurSayisi: map[string]int{}}

	rows, err := h.pool.Query(ctx, `
		SELECT i.malzeme_adi, i.kategori, i.birim, i.mevcut_miktar::float8, i.min_stok::float8,
		       COALESCE(c.cikis30, 0)::float8, COALESCE(to_char(s.son, 'YYYY-MM-DD'), '')
		FROM site_warehouse_items i
		LEFT JOIN (
			SELECT item_id, SUM(miktar) AS cikis30 FROM site_warehouse_movements
			WHERE project_id=$1 AND hareket_turu='cikis' AND tarih > CURRENT_DATE - 30
			GROUP BY item_id) c ON c.item_id = i.id
		LEFT JOIN (
			SELECT item_id, MAX(tarih) AS son FROM site_warehouse_movements
			WHERE project_id=$1 GROUP BY item_id) s ON s.item_id = i.id
		WHERE i.project_id=$1
		ORDER BY i.kategori, i.sira, i.malzeme_adi`, pid)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var k reportpdf.StokKalem
		var cikis30 float64
		if err := rows.Scan(&k.Ad, &k.Kategori, &k.Birim, &k.Mevcut, &k.Min, &cikis30, &k.SonHareket); err != nil {
			rows.Close()
			return out, err
		}
		if cikis30 > 0 {
			gun := k.Mevcut / (cikis30 / 30)
			k.Kapsama = &gun
		}
		if k.Min > k.Mevcut {
			k.Eksik = k.Min - k.Mevcut
		}
		k.Kritik = k.Min > 0 && k.Mevcut < k.Min
		out.Kalemler = append(out.Kalemler, k)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, err
	}

	rows, err = h.pool.Query(ctx, `
		SELECT hareket_turu, COUNT(*) FROM site_warehouse_movements
		WHERE project_id=$1 AND tarih BETWEEN $2 AND $3 GROUP BY hareket_turu`, pid, from, to)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var tur string
		var n int
		if err := rows.Scan(&tur, &n); err != nil {
			rows.Close()
			return out, err
		}
		out.TurSayisi[tur] = n
	}
	rows.Close()

	rows, err = h.pool.Query(ctx, `
		SELECT to_char(m.tarih, 'YYYY-MM-DD'), COALESCE(m.belge_no, ''), i.malzeme_adi, i.birim,
		       m.hareket_turu, m.miktar::float8, COALESCE(m.aciklama, '')
		FROM site_warehouse_movements m
		JOIN site_warehouse_items i ON i.id = m.item_id
		WHERE m.project_id=$1 AND m.tarih BETWEEN $2 AND $3
		ORDER BY m.tarih DESC, m.created_at DESC
		LIMIT 60`, pid, from, to)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var s reportpdf.HareketSatir
		var tur string
		if err := rows.Scan(&s.Tarih, &s.Belge, &s.Malzeme, &s.Birim, &tur, &s.Miktar, &s.Aciklama); err != nil {
			rows.Close()
			return out, err
		}
		s.Tur = hareketTurEtiket[tur]
		out.Hareketler = append(out.Hareketler, s)
	}
	rows.Close()

	// Açık zimmet: onaylı ve henüz iade edilmemiş zimmet kayıtları (dönemden bağımsız).
	rows, err = h.pool.Query(ctx, `
		SELECT to_char(t.tarih, 'YYYY-MM-DD'), COALESCE(p.ad_soyad, ''), COALESCE(p.firma, ''),
		       t.baslik, COALESCE(t.miktar, 0)::float8, COALESCE(t.birim, '')
		FROM saha_tutanaklari t
		LEFT JOIN project_personnel p ON p.id = t.personel_id
		WHERE t.project_id=$1 AND t.tip='zimmet' AND t.deleted_at IS NULL
		  AND t.durum='onaylandi' AND t.iade_tarihi IS NULL
		ORDER BY t.tarih`, pid)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var z reportpdf.ZimmetSatir
		if err := rows.Scan(&z.Tarih, &z.Personel, &z.Firma, &z.Kalem, &z.Miktar, &z.Birim); err != nil {
			rows.Close()
			return out, err
		}
		out.AcikZimmet = append(out.AcikZimmet, z)
	}
	rows.Close()
	out.ToplamAcikZimmet = len(out.AcikZimmet)
	return out, rows.Err()
}

var tipEtiket = map[string]string{"arac": "Araç", "is_makinesi": "İş makinesi", "ekipman": "Ekipman"}
var durumEtiket = map[string]string{"aktif": "Aktif", "bakim": "Bakımda", "devre_disi": "Devre dışı"}
var sahiplikEtiket = map[string]string{"ozmal": "Özmal", "kiralik": "Kiralık"}
var transferEtiket = map[string]string{"pending": "Bekliyor", "approved": "Onaylandı", "rejected": "Reddedildi"}

// makineEkipmanData — projeye atanmış ekipman, kullanım ve boşta maliyet, bakım, transfer.
func (h *Handler) makineEkipmanData(ctx context.Context, pid uuid.UUID, from, to, today time.Time) (reportpdf.MakineEkipmanData, error) {
	out := reportpdf.MakineEkipmanData{Baslangic: from, Bitis: to, RaporTarihi: today}

	type ekip struct {
		id      uuid.UUID
		row     reportpdf.EkipmanSatir
		gunluk  float64
		kiralik bool
		sonraki *time.Time
	}
	var list []*ekip
	byID := map[uuid.UUID]*ekip{}

	rows, err := h.pool.Query(ctx, `
		SELECT ce.id, ce.ad, ce.tip, COALESCE(NULLIF(ce.plaka, ''), NULLIF(ce.seri_no, ''), NULLIF(ce.demirbas_no, ''), '—'),
		       ce.sahiplik, ce.durum, COALESCE(ce.gunluk_ucret, 0)::float8,
		       COALESCE(to_char(ce.son_bakim_tarihi, 'YYYY-MM-DD'), ''),
		       ce.sonraki_bakim_tarihi
		FROM company_equipment ce
		WHERE ce.current_project_id=$1
		ORDER BY ce.tip, ce.ad`, pid)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		e := &ekip{}
		var tip, sahiplik, durum string
		var sonraki *time.Time
		if err := rows.Scan(&e.id, &e.row.Ad, &tip, &e.row.Kimlik, &sahiplik, &durum, &e.gunluk,
			&e.row.SonBakim, &sonraki); err != nil {
			rows.Close()
			return out, err
		}
		e.row.Tip = tipEtiket[tip]
		e.row.Sahiplik = sahiplikEtiket[sahiplik]
		e.row.Durum = durumEtiket[durum]
		e.kiralik = sahiplik == "kiralik"
		e.sonraki = sonraki
		if sonraki != nil {
			e.row.SonrakiBakim = sonraki.Format("2006-01-02")
			kalan := int(sonraki.Sub(today).Hours() / 24)
			e.row.KalanGun = &kalan
		}
		list = append(list, e)
		byID[e.id] = e
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, err
	}

	// Atanmış gün: projeye atama aralıklarının dönemle kesişimi.
	rows, err = h.pool.Query(ctx, `
		SELECT pm.company_equipment_id, pm.atanma_tarihi, COALESCE(pm.ayrilma_tarihi, $2::date)
		FROM project_machines pm
		WHERE pm.project_id=$1 AND pm.company_equipment_id IS NOT NULL`, pid, to)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var ceID uuid.UUID
		var a, b time.Time
		if err := rows.Scan(&ceID, &a, &b); err != nil {
			rows.Close()
			return out, err
		}
		if e := byID[ceID]; e != nil {
			e.row.AtanmisGun += overlapDays(a, b, from, to)
		}
	}
	rows.Close()

	// Çalışılan gün ve toplam çalışma (dönem içinde, pozitif kayıtlar).
	rows, err = h.pool.Query(ctx, `
		SELECT pm.company_equipment_id, COUNT(DISTINCT pml.tarih),
		       COALESCE(SUM(pml.calisma_miktari), 0)::float8, COALESCE(MAX(pml.calisma_birimi), 'saat')
		FROM project_machine_logs pml
		JOIN project_machines pm ON pm.id = pml.machine_id
		WHERE pml.project_id=$1 AND pml.tarih BETWEEN $2 AND $3
		  AND pml.calisma_miktari > 0 AND pm.company_equipment_id IS NOT NULL
		GROUP BY pm.company_equipment_id`, pid, from, to)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var ceID uuid.UUID
		var gun int
		var toplam float64
		var birim string
		if err := rows.Scan(&ceID, &gun, &toplam, &birim); err != nil {
			rows.Close()
			return out, err
		}
		if e := byID[ceID]; e != nil {
			e.row.CalismaGun = gun
			e.row.Calisma = toplam
			e.row.CalismaBirim = map[string]string{"saat": "saat", "km": "km"}[birim]
		}
	}
	rows.Close()

	for _, e := range list {
		r := &e.row
		if r.CalismaGun > r.AtanmisGun {
			r.CalismaGun = r.AtanmisGun
		}
		r.BostaGun = r.AtanmisGun - r.CalismaGun
		if r.AtanmisGun > 0 {
			r.Kullanim = float64(r.CalismaGun) / float64(r.AtanmisGun)
		}
		if e.kiralik && e.gunluk > 0 {
			bm := float64(r.BostaGun) * e.gunluk
			r.BostaMaliyet = &bm
		}
		out.Ekipmanlar = append(out.Ekipmanlar, *r)
	}

	rows, err = h.pool.Query(ctx, `
		SELECT to_char(etr.created_at, 'YYYY-MM-DD'), ce.ad, pf.name, pt.name, etr.status
		FROM equipment_transfer_requests etr
		JOIN company_equipment ce ON ce.id = etr.company_equipment_id
		JOIN projects pf ON pf.id = etr.from_project_id
		JOIN projects pt ON pt.id = etr.to_project_id
		WHERE etr.from_project_id=$1 OR etr.to_project_id=$1
		ORDER BY etr.created_at DESC
		LIMIT 50`, pid)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var t reportpdf.TransferSatir
		var durum string
		if err := rows.Scan(&t.Tarih, &t.Ekipman, &t.Kaynak, &t.Hedef, &durum); err != nil {
			rows.Close()
			return out, err
		}
		t.Durum = transferEtiket[durum]
		out.Transferler = append(out.Transferler, t)
	}
	rows.Close()
	return out, rows.Err()
}

// overlapDays — [a,b] ile [from,to] aralıklarının kesişimindeki gün sayısı (uçlar dahil).
func overlapDays(a, b, from, to time.Time) int {
	start, end := a, b
	if from.After(start) {
		start = from
	}
	if to.Before(end) {
		end = to
	}
	if end.Before(start) {
		return 0
	}
	return int(end.Sub(start).Hours()/24) + 1
}

package reports

// Hava durumu ön doldurma (Plan Faz 6 — OPSİYONEL özellik).
//
// Varsayılan sağlayıcı Open-Meteo'dur (anahtar gerektirmez); IPKS_WEATHER_API_URL
// ile farklı bir uyumlu uç nokta verilebilir. IPKS_WEATHER_ENABLED=false ise uç
// 404 benzeri kibar bir yanıt döner ve istemci formu elle doldurur — özellik
// kapalıyken hiçbir dış çağrı yapılmaz.
//
// Backend proxy'lemesinin nedeni: PWA saha cihazları kısıtlı ağlarda dış API'ye
// doğrudan çıkamayabilir; ayrıca sağlayıcı değişimi tek dosyada kalır (Plan §2
// adaptör ilkesiyle tutarlı).

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ipks/ipks/backend/internal/httpx"
)

type weatherPrefill struct {
	Condition       string   `json:"condition"`
	TempMin         *float64 `json:"temperature_min,omitempty"`
	TempMax         *float64 `json:"temperature_max,omitempty"`
	WindKph         *float64 `json:"wind_kph,omitempty"`
	PrecipitationMM *float64 `json:"precipitation_mm,omitempty"`
	Source          string   `json:"source"`
}

// WeatherPrefill — GET ?date=YYYY-MM-DD&lat=..&lng=..
// Konum istemciden gelir (cihaz GPS'i ya da proje künyesinden seçim);
// projects.location serbest metin olduğundan koordinat backend'de türetilmez.
func (h *Handler) WeatherPrefill(w http.ResponseWriter, r *http.Request) {
	if !h.weatherEnabled {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound,
			"Hava durumu ön doldurma bu kurulumda kapalı (IPKS_WEATHER_ENABLED).", nil)
		return
	}
	q := r.URL.Query()
	date := strings.TrimSpace(q.Get("date"))
	if _, err := time.Parse("2006-01-02", date); err != nil {
		httpx.ValidationFailed(w, r, map[string]string{"date": "geçerli bir tarih (YYYY-MM-DD) girin"})
		return
	}
	lat, err1 := strconv.ParseFloat(q.Get("lat"), 64)
	lng, err2 := strconv.ParseFloat(q.Get("lng"), 64)
	if err1 != nil || err2 != nil || lat < -90 || lat > 90 || lng < -180 || lng > 180 {
		httpx.ValidationFailed(w, r, map[string]string{"lat": "geçerli koordinat girin", "lng": "geçerli koordinat girin"})
		return
	}

	base := h.weatherAPIURL
	if base == "" {
		base = "https://api.open-meteo.com/v1/forecast"
	}
	url := fmt.Sprintf("%s?latitude=%.4f&longitude=%.4f"+
		"&daily=weather_code,temperature_2m_min,temperature_2m_max,wind_speed_10m_max,precipitation_sum"+
		"&timezone=auto&start_date=%s&end_date=%s", base, lat, lng, date, date)

	ctx := r.Context()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	client := &http.Client{Timeout: 6 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		httpx.Error(w, r, http.StatusBadGateway, httpx.CodeInternal,
			"Hava durumu servisine ulaşılamadı; alanları elle doldurabilirsiniz.", nil)
		return
	}
	defer res.Body.Close()
	if res.StatusCode/100 != 2 {
		httpx.Error(w, r, http.StatusBadGateway, httpx.CodeInternal,
			"Hava durumu servisi hata döndü; alanları elle doldurabilirsiniz.", nil)
		return
	}

	var om struct {
		Daily struct {
			WeatherCode      []int     `json:"weather_code"`
			TempMin          []float64 `json:"temperature_2m_min"`
			TempMax          []float64 `json:"temperature_2m_max"`
			WindMax          []float64 `json:"wind_speed_10m_max"`
			PrecipitationSum []float64 `json:"precipitation_sum"`
		} `json:"daily"`
	}
	if err := json.NewDecoder(res.Body).Decode(&om); err != nil || len(om.Daily.WeatherCode) == 0 {
		httpx.Error(w, r, http.StatusBadGateway, httpx.CodeInternal,
			"Hava durumu yanıtı çözümlenemedi.", nil)
		return
	}

	out := weatherPrefill{
		Condition: wmoConditionTR(om.Daily.WeatherCode[0]),
		Source:    "open-meteo",
	}
	if len(om.Daily.TempMin) > 0 {
		out.TempMin = &om.Daily.TempMin[0]
	}
	if len(om.Daily.TempMax) > 0 {
		out.TempMax = &om.Daily.TempMax[0]
	}
	if len(om.Daily.WindMax) > 0 {
		out.WindKph = &om.Daily.WindMax[0]
	}
	if len(om.Daily.PrecipitationSum) > 0 {
		out.PrecipitationMM = &om.Daily.PrecipitationSum[0]
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"weather": out})
}

// wmoConditionTR — WMO hava kodu → kısa Türkçe açıklama.
func wmoConditionTR(code int) string {
	switch {
	case code == 0:
		return "Açık"
	case code <= 3:
		return "Parçalı bulutlu"
	case code == 45 || code == 48:
		return "Sisli"
	case code >= 51 && code <= 57:
		return "Çisenti"
	case code >= 61 && code <= 67:
		return "Yağmurlu"
	case code >= 71 && code <= 77:
		return "Karlı"
	case code >= 80 && code <= 82:
		return "Sağanak yağışlı"
	case code == 85 || code == 86:
		return "Kar sağanağı"
	case code >= 95:
		return "Gök gürültülü fırtına"
	default:
		return "Bulutlu"
	}
}

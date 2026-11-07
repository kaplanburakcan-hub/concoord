"""DEMO-05 için haftalık, aylık, çeyrek ve özel dönem raporlarını UYGULAMANIN
kendi üretim ucuyla oluşturur (rakamlar uygulamanın hesaplarıyla tutarlı olur).

Yerel API'de çalışır: önce yerel API'yi başlatın, sonra:
  python gen_reports_demo05.py
Giriş bilgileri yerel .env dosyasından okunur.
"""
import json
import os
import time
import urllib.error
import urllib.request
from datetime import date, timedelta

BASE = "http://localhost:8080/api/v1"


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
    req = urllib.request.Request(BASE + path, data=data, method=method)
    req.add_header("Content-Type", "application/json")
    if token:
        req.add_header("Authorization", "Bearer " + token)
    try:
        with urllib.request.urlopen(req, timeout=120) as r:
            raw = r.read()
            return r.status, (json.loads(raw) if raw else None)
    except urllib.error.HTTPError as e:
        return e.code, e.read().decode(errors="replace")


def main():
    here = os.path.dirname(os.path.abspath(__file__))
    env = load_env(os.path.join(here, "..", "..", ".env"))
    s, body = call("POST", "/auth/login", None, {
        "identifier": env["IPKS_BOOTSTRAP_ADMIN_EMAIL"],
        "password": env["IPKS_BOOTSTRAP_ADMIN_PASSWORD"],
    })
    token = body["tokens"]["access_token"]
    s, body = call("GET", "/projects", token)
    pid = next(p["id"] for p in body["projects"] if p["code"] == "DEMO-05")
    base = f"/projects/{pid}"

    # Haftalık: 2025-06-02 (Pazartesi) .. 2026-10-26 (son tamamlanmış hafta)
    results = {"weekly": [], "monthly": [], "quarterly": [], "custom": []}
    d = date(2025, 6, 2)
    while d <= date(2026, 10, 26):
        s, b = call("POST", f"{base}/weekly-reports", token, {"period_start": d.isoformat()})
        results["weekly"].append((d.isoformat(), s))
        d += timedelta(days=7)

    # Aylık: 2025-06 .. 2026-10
    y, m = 2025, 6
    while (y, m) <= (2026, 10):
        s, b = call("POST", f"{base}/monthly-reports", token, {"kind": "monthly", "month": f"{y}-{m:02d}"})
        results["monthly"].append((f"{y}-{m:02d}", s))
        m += 1
        if m > 12:
            y, m = y + 1, 1

    # Çeyrek: 2025 Q3 .. 2026 Q3 (Q3 2026 tamamlandı)
    for year, q in [(2025, 3), (2025, 4), (2026, 1), (2026, 2), (2026, 3)]:
        s, b = call("POST", f"{base}/monthly-reports", token, {"kind": "quarterly", "year": year, "quarter": q})
        results["quarterly"].append((f"{year}Q{q}", s))

    # Özel dönem: proje başından 2026-10-31'e kadar
    s, b = call("POST", f"{base}/monthly-reports", token,
                {"kind": "custom", "start": "2025-06-01", "end": "2026-10-31"})
    results["custom"].append(("2025-06-01..2026-10-31", s))

    for k, v in results.items():
        ok = sum(1 for _, s in v if s in (200, 201))
        bad = [(p, s) for p, s in v if s not in (200, 201)]
        print(f"{k}: {ok}/{len(v)} oluşturuldu", ("hatalar: " + str(bad[:5])) if bad else "")


if __name__ == "__main__":
    main()

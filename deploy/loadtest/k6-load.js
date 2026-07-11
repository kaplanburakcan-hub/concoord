// İPKS — Faz 10 yük testi (k6). Kabul kriteri: 100 eşzamanlı kullanıcı.
//
// Çalıştırma:
//   BASE_URL=https://ipks.example.com \
//   IPKS_USER=admin@ipks.local IPKS_PASS='...' \
//   k6 run deploy/loadtest/k6-load.js
//
// Senaryo, gerçek bir oturumun sıcak yollarını taklit eder: login → proje
// listesi → portföy → seçili projenin dashboard'u → görev listesi. Yazma
// işlemleri bilinçli olarak dışarıda (yük testi prod veriyi kirletmemeli);
// okuma-ağırlıklı profil, dashboard/EVM toplama sorgularının 100 kullanıcı
// altındaki davranışını ölçer.
//
// Eşikler (thresholds) aşılırsa k6 sıfırdan farklı kod döner → CI'da kırmızı.

import http from "k6/http";
import { check, sleep, group } from "k6";
import { Rate } from "k6/metrics";

const errors = new Rate("errors");

const BASE = __ENV.BASE_URL || "http://localhost:8080";
const USER = __ENV.IPKS_USER || "admin@ipks.local";
const PASS = __ENV.IPKS_PASS || "";

export const options = {
  scenarios: {
    steady_100: {
      executor: "ramping-vus",
      startVUs: 0,
      stages: [
        { duration: "30s", target: 100 }, // rampa
        { duration: "3m", target: 100 },  // 100 eşzamanlı, sürekli
        { duration: "30s", target: 0 },   // iniş
      ],
      gracefulRampDown: "10s",
    },
  },
  thresholds: {
    http_req_failed: ["rate<0.01"],           // <%1 hata
    http_req_duration: ["p(95)<800", "p(99)<2000"], // p95<800ms, p99<2s
    errors: ["rate<0.01"],
  },
};

function authHeaders(token) {
  return { headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" } };
}

export function setup() {
  const res = http.post(
    `${BASE}/api/v1/auth/login`,
    JSON.stringify({ email: USER, password: PASS }),
    { headers: { "Content-Type": "application/json" } }
  );
  check(res, { "login 200": (r) => r.status === 200 });
  const body = res.json();
  return { token: body.access_token || (body.tokens && body.tokens.access_token) };
}

export default function (data) {
  const h = authHeaders(data.token);
  let projectId = null;

  group("projeler", () => {
    const res = http.get(`${BASE}/api/v1/projects`, h);
    ok(res, "projects list");
    const list = safeJson(res);
    const arr = (list && (list.projects || list.data || list)) || [];
    if (Array.isArray(arr) && arr.length > 0) projectId = arr[0].id;
  });

  group("portfoy", () => {
    const res = http.get(`${BASE}/api/v1/portfolio`, h);
    ok(res, "portfolio");
  });

  if (projectId) {
    group("dashboard", () => {
      const res = http.get(`${BASE}/api/v1/projects/${projectId}/dashboard`, h);
      ok(res, "project dashboard (EVM)");
    });
    group("gorevler", () => {
      const res = http.get(`${BASE}/api/v1/projects/${projectId}/tasks`, h);
      ok(res, "task list");
    });
  }

  sleep(Math.random() * 2 + 1); // düşünme süresi 1–3 sn
}

function ok(res, name) {
  const good = check(res, { [`${name} 2xx`]: (r) => r.status >= 200 && r.status < 300 });
  errors.add(!good);
}

function safeJson(res) {
  try {
    return res.json();
  } catch (_e) {
    return null;
  }
}

// İPKS — Faz 10 duman testi (smoke). Yük testinden önce hızlı sağlık kontrolü:
// 1 VU, kısa süre. Uçlar ayakta ve login çalışıyor mu?
//
//   BASE_URL=... IPKS_USER=... IPKS_PASS=... k6 run deploy/loadtest/k6-smoke.js

import http from "k6/http";
import { check } from "k6";

const BASE = __ENV.BASE_URL || "http://localhost:8080";
const USER = __ENV.IPKS_USER || "admin@ipks.local";
const PASS = __ENV.IPKS_PASS || "";

export const options = {
  vus: 1,
  iterations: 1,
  thresholds: { http_req_failed: ["rate<0.01"] },
};

export default function () {
  const hz = http.get(`${BASE}/healthz`);
  check(hz, { "healthz ok": (r) => r.status === 200 });

  const rz = http.get(`${BASE}/readyz`);
  check(rz, { "readyz ready": (r) => r.status === 200 });

  const login = http.post(
    `${BASE}/api/v1/auth/login`,
    JSON.stringify({ email: USER, password: PASS }),
    { headers: { "Content-Type": "application/json" } }
  );
  check(login, { "login 200": (r) => r.status === 200 });
}

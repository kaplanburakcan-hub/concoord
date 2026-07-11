# İPKS API — Faz 1: Kimlik ve Yetki Sözleşmesi

Tüm yanıtlar JSON. Hatalar Faz 0 error envelope'unu kullanır:
`{ "error": { "code, message, details?, request_id? } }`.
Korumalı uçlar `Authorization: Bearer <access_token>` bekler. Proje kapsamı
`X-Project-Id: <uuid>` başlığı veya `?project_id=` ile verilir (opsiyonel).

## Kimlik (herkese açık)

| Metot | Yol | Gövde | Yanıt |
|---|---|---|---|
| POST | `/api/v1/auth/login` | `{identifier, password}` | `{user, tokens{access_token, refresh_token, expires_at}}` |
| POST | `/api/v1/auth/refresh` | `{refresh_token}` | `{tokens{...}}` (rotasyon: eski iptal) |
| POST | `/api/v1/auth/logout` | `{refresh_token}` | `{status}` |
| POST | `/api/v1/auth/password/forgot` | `{email}` | `{status}` (+ dev'de `reset_token`) |
| POST | `/api/v1/auth/password/reset` | `{token, new_password}` | `{status}` |

`identifier` e-posta **veya** kullanıcı adı olabilir. Hatalı giriş her durumda
`401 unauthorized` döner (kullanıcı varlığı ifşa edilmez).

## Kimlik (oturum gerektirir)

| Metot | Yol | Açıklama |
|---|---|---|
| GET | `/api/v1/auth/me` | `{user, permissions[], project_id}` — etkin izin listesi (`<Can>` bunu kullanır) |
| POST | `/api/v1/auth/password/change` | `{current_password, new_password}` — tüm refresh oturumları iptal edilir |

## Admin Paneli (oturum + izin korumalı)

### Kullanıcılar — izin: `admin.manage_users`
| Metot | Yol | Notlar |
|---|---|---|
| GET | `/api/v1/admin/users?query=` | ad/e-posta/kullanıcı adı araması |
| POST | `/api/v1/admin/users` | `{email, username, full_name, phone?, password, is_active?}` |
| GET | `/api/v1/admin/users/{id}` | |
| PATCH | `/api/v1/admin/users/{id}` | `{full_name?, phone?, is_active?, row_version}` (optimistic lock) |
| DELETE | `/api/v1/admin/users/{id}` | soft delete + oturum iptali |

### İzin matrisi & sözlük — izin: `admin.manage_permissions`
| Metot | Yol | Notlar |
|---|---|---|
| GET | `/api/v1/admin/roles` | 7 sistem rolü |
| GET | `/api/v1/admin/permissions` | izin sözlüğü (§4) |
| GET | `/api/v1/admin/users/{id}/permissions?project_id=` | matris satırları: `{code, module, action, role_default, override, effective}` |
| PUT | `/api/v1/admin/users/{id}/permissions/{code}` | `{project_id?, effect}` · `effect ∈ {GRANT, DENY, null}` (null = temizle) |

Override etkisi **anında** geçerlidir: bir sonraki API kararı yeni değeri kullanır.

### Rol atama (proje üyeliği) — izin: `projects.manage_members` (proje kapsamlı)
| Metot | Yol |
|---|---|
| GET | `/api/v1/admin/projects/{projectID}/members` |
| POST | `/api/v1/admin/projects/{projectID}/members` — `{user_id, role_code}` |
| DELETE | `/api/v1/admin/projects/{projectID}/members/{userID}` |

### Denetim izi — izin: `admin.view_audit_log`
| Metot | Yol |
|---|---|
| GET | `/api/v1/admin/audit-logs?entity=&action=&actor_id=&limit=` |

## Yetki değerlendirme kuralı

Bir `(kullanıcı, proje, izin)` için karar (Plan §4):

```
1) uygulanabilir override'larda DENY varsa  → RED
2) yoksa GRANT varsa                         → İZİN
3) yoksa proje rolü varsayılanı              → rolün kararı
```

"Uygulanabilir override" = `project_id IS NULL` (global) **veya** istenen projeye
ait override. Proje bağımsız (global) kontrolde rol varsayılanı, kullanıcının
herhangi bir üyeliğindeki rolden gelebilir.

## Örnek: override etkisini doğrulama (kabul kriteri)

```bash
# 1) admin girişi
curl -sX POST localhost:8080/api/v1/auth/login \
  -H 'content-type: application/json' \
  -d '{"identifier":"admin@ipks.local","password":"<parola>"}'   # -> access_token

# 2) bir kullanıcıya global DENY yaz (örn. audit görüntülemeyi kapat)
curl -sX PUT localhost:8080/api/v1/admin/users/<uid>/permissions/admin.view_audit_log \
  -H "authorization: Bearer <admin_access>" -H 'content-type: application/json' \
  -d '{"effect":"DENY"}'

# 3) o kullanıcının /me izinleri arasında admin.view_audit_log ARTIK YOK,
#    ve /admin/audit-logs çağrısı 403 forbidden döner (anında etki).
```

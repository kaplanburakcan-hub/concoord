## Ne değişti?

<!-- Kısa özet: hangi modül, hangi davranış. -->

## Neden?

<!-- İlgili plan maddesi / issue / saha geri bildirimi. -->

## Nasıl test edildi?

- [ ] `make ci` yerelde yeşil (gofmt/vet, go test -race, typecheck, lint, vitest, build)
- [ ] Yeni migration varsa: up **ve** down yazıldı, `make migrate-smoke` geçti
- [ ] Yeni/değişen endpoint varsa: yetki (RBAC) kontrolü ve gerekirse `deploy/ci/api-smoke.sh` güncellendi
- [ ] Arayüz değişikliği varsa: ekran görüntüsü eklendi

## Notlar

<!-- Deploy sırasında dikkat edilecekler, geri alma planı, açık konular. -->

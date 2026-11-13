# Arayüz testleri (jsdom)

`go/web/` altındaki gerçek arayüzü (`index.html` + betikler) **jsdom** içinde çalıştırır. API yanıtları gerçek Go sunucusundan
kaydedilmiş `fixtures/` dosyalarından gelir; saat sabittir, zamanlayıcılar kontrollüdür. Amaç: arayüz kodunu (özellikle refaktör
sırasında) **davranışı değişmeden** düzenleyebilmek.

- `views.test.js`: sayfa ve pencerelerin DOM **anlık görüntüleri** (`snapshots/`). Refaktörde değişmemeli.
- `jobs.test.js`: iş akışı davranışı (tek/paralel iş, arka plana alma, dock, toplu işlem, eşzamanlılık ayarı) için açık beklentiler.
- `harness.js`: jsdom kurulumu, sahte fetch/xterm, anlık görüntü yardımcıları.

```bash
cd go/webtest
npm ci --ignore-scripts      # bir kez (yalnız geliştirme/CI aracı; ikiliye girmez, web/ altında değildir)
npm test                     # tüm testler
UPDATE_SNAPSHOTS=1 npm test  # anlık görüntüleri BİLEREK yenile; git diff ile gözle inceleyin
bash record-fixtures.sh      # fixtures/ dosyalarını gerçek Go sunucusundan yeniden kaydet (API şekli değişince)
```

`go test` bu testleri, `node` ve `node_modules` varsa çalıştırır; yoksa atlar.

**Sınırlar:** jsdom yerleşim/CSS hesaplamaz ve gerçek tarayıcı davranışını (odak, SSE, gerçek xterm) taklit etmez; yalnız DOM ve JS
mantığını kilitler. Görsel değişiklikler ve gerçek terminal için tarayıcıda elle bakılır.

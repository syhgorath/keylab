# xterm.js (üçüncü taraf, değiştirilmeden)

- `xterm.js`, `xterm.css`: npm `@xterm/xterm@5.5.0` (`lib/xterm.js`, `css/xterm.css`); jsDelivr/npm özetleriyle birebir eşleştiği doğrulandı (2026-11-12).
- `addon-fit.js`: npm `@xterm/addon-fit@0.9.0`/`0.10.0` (`lib/addon-fit.js`; iki sürümde dosya aynı); özet eşleşmesi doğrulandı.
- Tek fark (tüm eklentilerde): dosya sonundaki `//# sourceMappingURL=...` satırı çıkarıldı (harita dosyası dahil edilmiyor).
- Lisans: MIT (`LICENSE`).
- `addon-search.js`: npm `@xterm/addon-search@0.15.0` (`lib/addon-search.js`), `peerDependencies: @xterm/xterm ^5.0.0`. npm tarball'ı sha1 `5c772d5f14c26546c4bfbeb0c3d4b3333057411f` ve sha512 `ZBZKLQ+EuKE83CqCmSSz5y1tx+aNOCUaA7dm6emgOX+8J9H1FWXZyrKfzjwzV+V14TV3xToz1goIeRhXBS5qjg==` ile doğrulandı; jsDelivr'deki dosyayla birebir aynı (2026-11-14).
- `addon-web-links.js`: npm `@xterm/addon-web-links@0.11.0`, `peerDependencies: @xterm/xterm ^5.0.0`. sha1 `f283513b8c713757bad8e3bf04b6becc3b4e585f`, sha512 `nIHQ38pQI+a5kXnRaTgwqSHnX7KE6+4SVoceompgHL26unAxdfP6IPqUTSYPQgSwM56hsElfoNrrW5V7BUED/Q==` ile doğrulandı; jsDelivr ile birebir aynı (2026-11-14).
- Sürüm seçimi: npm'in `latest` etiketi (addon-search 0.16.0, addon-web-links 0.12.0) xterm 6 içindir; xterm 5.5.0 ile uyumlu son kararlı sürümler 0.15.0 ve 0.11.0'dır.
- pocketui'deki `addon-search.js`/`addon-web-links.js` resmî eklenti değil, elle yazılmış uyarlamalardı; bilerek ALINMADI, yerine resmî paketler kullanıldı.
- Güncellerken: yeni sürümü npm'den alın, özetini jsDelivr'in `?structure=flat` çıktısıyla karşılaştırın ve bu dosyayı güncelleyin.

"use strict";
/* "Bu Mac'e güveniyorum" (js/ui/trustedmac.js): taşınabilir modda Ayarlar kartı; işaret açılır/kapanır; değişiklik yeniden başlatınca geçerli. */
const test = require("node:test");
const assert = require("node:assert/strict");
const { boot, fixture } = require("./harness");

const T = (o = {}) => ({ portable: true, trusted: false, active: "guest", ...o });
const base = (tm, o = {}) => ({ "/api/home": () => fixture("home"), "/api/trusted-mac": tm, ...o });
const posts = h => h.calls.filter(c => c.method === "POST" && c.path === "/api/trusted-mac").map(c => c.body);

test("güvenilir Mac: taşınabilir değilse kart görünmez", async () => {
  const h = await boot({ hash: "", overrides: base({ portable: false }) });
  assert.equal(h.$("#tm-card").hidden, true); assert.deepEqual(h.errors, []); h.close();
});

test("güvenilir Mac: taşınabilir + misafir → kart görünür, mod ve düğme metni doğru; tıklayınca {enabled:true} gider ve 'yeniden başlat' notu çıkar", async () => {
  let st = T();
  const h = await boot({ hash: "", overrides: base(() => st, { "/api/trusted-mac": (u, o) => (o.method === "POST" ? (st = T({ trusted: o.body.enabled }), st) : st) }) });
  assert.equal(h.$("#tm-card").hidden, false); assert.equal(h.$("#tm-state").textContent, "misafir");
  assert.match(h.$("#tm-active").textContent, /misafir mod/); assert.equal(h.$("#tm-toggle").textContent, "Bu Mac'e güveniyorum: Keychain'i kullan"); assert.equal(h.$("#tm-pending").hidden, true);
  await h.click("#tm-toggle"); await h.settle();
  assert.deepEqual(posts(h), [{ enabled: true }]);
  assert.equal(h.$("#tm-state").textContent, "güvenilir"); assert.equal(h.$("#tm-toggle").textContent, "Bu Mac'e artık güvenme"); assert.equal(h.$("#tm-toggle").getAttribute("aria-pressed"), "true");
  assert.equal(h.$("#tm-pending").hidden, false); assert.match(h.$("#tm-pending").textContent, /yeniden başlatınca/);
  await h.click("#tm-toggle"); await h.settle();
  assert.deepEqual(posts(h), [{ enabled: true }, { enabled: false }]); assert.equal(h.$("#tm-pending").hidden, true, "işaret kapandı + mod hâlâ misafir: bekleyen değişiklik yok");
  assert.deepEqual(h.errors, []); h.close();
});

test("güvenilir Mac: Keychain modunda çalışırken mod satırı bunu söyler; işaret kapatılırsa yeniden başlatma notu çıkar", async () => {
  let st = T({ trusted: true, active: "keychain" });
  const h = await boot({ hash: "", overrides: base(() => st, { "/api/trusted-mac": (u, o) => (o.method === "POST" ? (st = T({ trusted: o.body.enabled, active: "keychain" }), st) : st) }) });
  assert.match(h.$("#tm-active").textContent, /Keychain'i ve ssh-agent'ı kullanılıyor/); assert.equal(h.$("#tm-pending").hidden, true);
  await h.click("#tm-toggle"); await h.settle();
  assert.deepEqual(posts(h), [{ enabled: false }]); assert.equal(h.$("#tm-pending").hidden, false);
  assert.deepEqual(h.errors, []); h.close();
});

test("güvenilir Mac: sunucu hata verirse bildirim çıkar, durum değişmez, düğme yeniden etkin olur", async () => {
  const h = await boot({ hash: "", overrides: base(() => T(), { "/api/trusted-mac": (u, o) => (o.method === "POST" ? { status: 500, body: "ayar kaydedilemedi: izin yok" } : T()) }) });
  await h.click("#tm-toggle"); await h.settle();
  assert.match(h.$("#toast").textContent, /ayar kaydedilemedi/); assert.equal(h.$("#tm-state").textContent, "misafir"); assert.equal(h.$("#tm-toggle").disabled, false);
  assert.deepEqual(h.errors, []); h.close();
});

test("güvenilir Mac: İngilizce arayüzde kart çevrilir", async () => {
  const h = await boot({ hash: "", lang: "en", overrides: base(() => T()) });
  assert.equal(h.$("#tm-card h2").textContent, "Keychain on this Mac"); assert.equal(h.$("#tm-toggle").textContent, "I trust this Mac: use the Keychain"); assert.equal(h.$("#tm-state").textContent, "guest"); h.close();
});

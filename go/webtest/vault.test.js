"use strict";
/* Şifreli passphrase kasası arayüzü (js/ui/vault.js): açılış girişi, yanlış parola/bekleme, oturum ortasında 423, kilitle düğmesi, Ayarlar kartı. */
const test = require("node:test");
const assert = require("node:assert/strict");
const { boot, fixture } = require("./harness");

const S = (o = {}) => ({ available: true, exists: true, unlocked: false, retryAfter: 0, idleMin: 15, ...o });
const base = (vault, o = {}) => ({ "/api/home": () => fixture("home"), "/api/vault": vault, ...o });
const posts = (h, p) => h.calls.filter(c => c.method === "POST" && c.path === p).map(c => c.body);
const fill = async (h, sel, v) => { h.$(sel).value = v; };
const submit = async h => { h.$("#vault-lock-form").dispatchEvent(new h.win.Event("submit", { bubbles: true, cancelable: true })); await h.settle(); };

test("kasa: misafir mod yoksa (available:false) kart, düğme ve perde görünmez", async () => {
  const h = await boot({ hash: "", overrides: base({ available: false }) });
  assert.equal(h.$("#vault-card").hidden, true); assert.equal(h.$("#vault-lock").hidden, true); assert.equal(h.$("#vault-lock-btn").hidden, true);
  assert.deepEqual(h.errors, []); h.close();
});

test("kasa: dosya yok → Ayarlar'da oluşturma formu; parola uyuşmaz/kısa ise istek gitmez; geçerli parola POST eder ve alanlar temizlenir", async () => {
  let st = S({ exists: false });
  const h = await boot({ hash: "", overrides: base(() => st, { "/api/vault/create": () => { st = S({ unlocked: true, stored: [] }); return { ok: true }; } }) });
  assert.equal(h.$("#vault-card").hidden, false); assert.equal(h.$("#vault-new").hidden, false); assert.equal(h.$("#vault-open").hidden, true); assert.equal(h.$("#vault-lock").hidden, true);
  await fill(h, "#vault-new-pw", "ana-parola-1"); await fill(h, "#vault-new-pw2", "başka-parola"); await h.click("#vault-create");
  assert.match(h.$("#vault-msg").textContent, /aynı değil/); assert.deepEqual(posts(h, "/api/vault/create"), []);
  await fill(h, "#vault-new-pw", "kısa"); await fill(h, "#vault-new-pw2", "kısa"); await h.click("#vault-create");
  assert.match(h.$("#vault-msg").textContent, /en az 8/); assert.deepEqual(posts(h, "/api/vault/create"), []);
  await fill(h, "#vault-new-pw", "ana-parola-1"); await fill(h, "#vault-new-pw2", "ana-parola-1"); await h.click("#vault-create"); await h.settle();
  assert.deepEqual(posts(h, "/api/vault/create"), [{ password: "ana-parola-1" }]);
  assert.equal(h.$("#vault-new-pw").value, ""); assert.equal(h.$("#vault-new-pw2").value, "");
  assert.equal(h.$("#vault-new").hidden, true); assert.equal(h.$("#vault-open").hidden, false); assert.equal(h.$("#vault-lock-btn").hidden, false);
  assert.deepEqual(h.errors, []); h.close();
});

test("kasa: açılışta kilitliyse giriş perdesi çıkar, arka plan inert olur; yanlış parola hata gösterir ve alanı temizler", async () => {
  const h = await boot({ hash: "", overrides: base(() => S(), { "/api/vault/unlock": () => ({ status: 401, body: { error: "ana parola yanlış", retryAfter: 0 } }) }) });
  assert.equal(h.$("#vault-lock").hidden, false);
  assert.ok(h.$("#app, .sidebar, header") && [...h.doc.body.children].some(el => el.id !== "vault-lock" && el.tagName !== "SCRIPT" && el.hasAttribute("inert")), "arka plan inert");
  await fill(h, "#vault-lock-pw", "yanlış"); await submit(h);
  assert.deepEqual(posts(h, "/api/vault/unlock"), [{ password: "yanlış" }]);
  assert.equal(h.$("#vault-lock-err").textContent, "Ana parola yanlış."); assert.equal(h.$("#vault-lock-pw").value, ""); assert.equal(h.$("#vault-lock").hidden, false);
  assert.equal(h.$("#vault-lock-go").disabled, false); assert.deepEqual(h.errors, []); h.close();
});

test("kasa: yanlış parola + sunucu bekleme süresi verirse iki mesaj birlikte görünür, süre bitince yalnız 'yanlış' kalır", async () => {
  const h = await boot({ hash: "", overrides: base(() => S(), { "/api/vault/unlock": () => ({ status: 401, body: { error: "ana parola yanlış", retryAfter: 2 } }) }) });
  await fill(h, "#vault-lock-pw", "x"); await submit(h);
  assert.equal(h.$("#vault-lock-err").textContent, "Ana parola yanlış. Çok fazla yanlış deneme; 2 sn bekleyin."); assert.equal(h.$("#vault-lock-go").disabled, true);
  h.timers.run(3000); await h.settle();
  assert.equal(h.$("#vault-lock-err").textContent, "Ana parola yanlış."); assert.equal(h.$("#vault-lock-go").disabled, false);
  assert.deepEqual(h.errors, []); h.close();
});

test("kasa: 429 bekleme → geri sayım düğmeyi kilitler, süre dolunca açar", async () => {
  const h = await boot({ hash: "", overrides: base(() => S(), { "/api/vault/unlock": () => ({ status: 429, body: { error: "çok fazla yanlış deneme", retryAfter: 3 } }) }) });
  await fill(h, "#vault-lock-pw", "x"); await submit(h);
  assert.equal(h.$("#vault-lock-go").disabled, true); assert.match(h.$("#vault-lock-err").textContent, /3 sn bekleyin/);
  h.timers.run(1000); await h.settle(); assert.match(h.$("#vault-lock-err").textContent, /2 sn bekleyin/);
  h.timers.run(3000); await h.settle(); assert.equal(h.$("#vault-lock-go").disabled, false); assert.equal(h.$("#vault-lock-err").textContent, "", "bekleme bitince eski mesaj kalmaz");
  assert.deepEqual(h.errors, []); h.close();
});

test("kasa: doğru parola → perde kalkar, arka plan inert'ten çıkar; sayfa kilitliyken yüklendiği için tam yenileme istenir", async () => {
  let st = S();
  const h = await boot({ hash: "", overrides: base(() => st, { "/api/vault/unlock": () => { st = S({ unlocked: true, stored: ["web1"] }); return { ok: true, loaded: ["web1"], failed: [] }; } }) });
  let reloads = 0; h.win.KeyLabTestReload = () => { reloads++; };
  await fill(h, "#vault-lock-pw", "ana-parola-1"); await submit(h);
  assert.deepEqual(posts(h, "/api/vault/unlock"), [{ password: "ana-parola-1" }]);
  assert.equal(h.$("#vault-lock").hidden, true); assert.equal(h.$("#vault-lock-pw").value, "");
  assert.ok(![...h.doc.body.children].some(el => el.hasAttribute("inert")), "inert kalktı");
  assert.equal(reloads, 1); assert.equal(h.$("#vault-lock-btn").hidden, false);
  assert.deepEqual(h.errors, []); h.close();
});

test("kasa: oturum ortasında 423 → perde yeniden iner (açık sayfa yenilenmez); açınca yalnız veri yenilenir", async () => {
  let st = S({ unlocked: true, stored: [] }), locked = false;
  const h = await boot({ hash: "", overrides: base(() => st, {
    "/api/vault/unlock": () => { locked = false; st = S({ unlocked: true, stored: [] }); return { ok: true, loaded: [], failed: [] }; },
    "/api/about": () => (locked ? { status: 423, body: "kasa kilitli" } : fixture("about")),
  }) });
  let reloads = 0; h.win.KeyLabTestReload = () => { reloads++; };
  assert.equal(h.$("#vault-lock").hidden, true);
  locked = true; st = S(); // sunucu boşta kilitledi
  await h.win.eval('fetch("/api/about", {headers:{"X-Requested-With":"sshpocket"}}).catch(()=>{})'); h.win.dispatchEvent(new h.win.CustomEvent("keylab:vaultlocked")); await h.settle();
  assert.equal(h.$("#vault-lock").hidden, false); assert.match(h.$("#vault-lock-err").textContent, /kilitlendi/);
  await fill(h, "#vault-lock-pw", "ana-parola-1"); await submit(h);
  assert.equal(h.$("#vault-lock").hidden, true); assert.equal(reloads, 0, "açık sayfa yenilenmez");
  assert.deepEqual(h.errors, []); h.close();
});

test("kasa: util.api 423'ü olaya çevirir: kasa açıkken bir istek 423 alırsa (sunucu boşta kilitlemiş) perde iner", async () => {
  const h = await boot({ hash: "", overrides: base(() => S({ unlocked: true, stored: [] }), { "/api/about": { status: 423, body: "kasa kilitli" } }) });
  assert.equal(h.$("#vault-lock").hidden, false); assert.match(h.$("#vault-lock-err").textContent, /kilitlendi/);
  assert.deepEqual(h.errors, []); h.close();
});

test("kasa: Ayarlar kartı açıkken passphrase ekler/siler, kilitler; yanıt hataları gösterilir ve alan temizlenir", async () => {
  let st = S({ unlocked: true, stored: [] });
  const h = await boot({ hash: "#v=settings", overrides: base(() => st, {
    "/api/vault/passphrase": (u, o) => (o.body.passphrase === "yanlış" ? { status: 400, body: { error: "passphrase anahtarı açmıyor (yanlış olabilir); kaydedilmedi" } } : (st = S({ unlocked: true, stored: [o.body.server] }), { ok: true })),
    "/api/vault/passphrase/delete": () => { st = S({ unlocked: true, stored: [] }); return { ok: true, removed: true }; },
    "/api/vault/lock": () => { st = S(); return { ok: true }; },
  }) });
  assert.equal(h.$("#vault-open").hidden, false); assert.match(h.$("#vault-idle").textContent, /15 dk/);
  assert.ok(h.$("#vault-srv").options.length >= 1, "sunucu listesi dolu");
  const srv = h.$("#vault-srv").value;
  await fill(h, "#vault-pp", "yanlış"); await h.click("#vault-add"); await h.settle();
  assert.match(h.$("#vault-msg").textContent, /kaydedilmedi/); assert.equal(h.$("#vault-pp").value, ""); assert.equal(h.$("#vault-msg").classList.contains("bad"), true);
  await fill(h, "#vault-pp", "doğru"); await h.click("#vault-add"); await h.settle();
  assert.deepEqual(posts(h, "/api/vault/passphrase").map(b => [b.server, b.passphrase]), [[srv, "yanlış"], [srv, "doğru"]]);
  assert.match(h.$("#vault-msg").textContent, /passphrase eklendi ve anahtar ajana yüklendi/); assert.equal(h.$("#vault-stored").textContent.includes(srv), true);
  await h.click('#vault-stored button[data-srv]'); await h.settle();
  assert.deepEqual(posts(h, "/api/vault/passphrase/delete"), [{ server: srv }]); assert.match(h.$("#vault-msg").textContent, /silindi/);
  await h.click("#vault-lock-btn"); await h.settle();
  assert.equal(posts(h, "/api/vault/lock").length, 1); assert.equal(h.$("#vault-lock").hidden, false); assert.equal(h.$("#vault-lock-btn").hidden, true);
  assert.deepEqual(h.errors, []); h.close();
});

test("kasa: ana parolayı değiştir → uyuşmazlık/kısa istek göndermez; geçerli istek eski+yeni parolayı yollar ve alanları temizler", async () => {
  const h = await boot({ hash: "#v=settings", overrides: base(() => S({ unlocked: true, stored: [] }), { "/api/vault/password": () => ({ ok: true }) }) });
  await fill(h, "#vault-old-pw", "eski-parola-1"); await fill(h, "#vault-chg-pw", "yeni-parola-2"); await fill(h, "#vault-chg-pw2", "yeni-parola-3"); await h.click("#vault-chg-go");
  assert.match(h.$("#vault-msg").textContent, /aynı değil/); assert.deepEqual(posts(h, "/api/vault/password"), []);
  await fill(h, "#vault-chg-pw2", "yeni-parola-2"); await h.click("#vault-chg-go"); await h.settle();
  assert.deepEqual(posts(h, "/api/vault/password"), [{ password: "eski-parola-1", newPassword: "yeni-parola-2" }]);
  for (const id of ["#vault-old-pw", "#vault-chg-pw", "#vault-chg-pw2"]) assert.equal(h.$(id).value, "");
  assert.match(h.$("#vault-msg").textContent, /değiştirildi/); assert.deepEqual(h.errors, []); h.close();
});

test("kasa: İngilizce arayüzde perde ve kart çevrilir", async () => {
  const h = await boot({ hash: "", lang: "en", overrides: base(() => S()) });
  assert.equal(h.$("#vault-lock-h").textContent, "KeyLab is locked"); assert.equal(h.$("#vault-lock-go").textContent, "Unlock");
  assert.equal(h.$("label[for=vault-lock-pw]").textContent, "Master password"); h.close();
});

test("kasa: gerçek kullanıcı girdisi (fare/klavye) boşta sayacı için /api/vault/touch bildirir; 30 sn içinde tekrar etmez; kilitliyken/kasa yokken yok", async () => {
  const h = await boot({ hash: "", overrides: base(() => S({ unlocked: true, stored: [] }), { "/api/vault/touch": () => ({ ok: true }) }) });
  const n = () => posts(h, "/api/vault/touch").length;
  h.$("body").dispatchEvent(new h.win.Event("pointerdown", { bubbles: true })); await h.settle();
  h.$("body").dispatchEvent(new h.win.Event("keydown", { bubbles: true })); await h.settle();
  assert.equal(n(), 1, "ilk girdi bildirilir, 30 sn içindeki ikincisi bildirilmez");
  assert.deepEqual(h.errors, []); h.close();
  const k = await boot({ hash: "", overrides: base(() => S()) });
  k.$("body").dispatchEvent(new k.win.Event("keydown", { bubbles: true })); await k.settle();
  assert.equal(k.calls.filter(c => c.path === "/api/vault/touch").length, 0, "kilitliyken bildirilmez"); k.close();
  const o = await boot({ hash: "", overrides: base({ available: false }) });
  o.$("body").dispatchEvent(new o.win.Event("keydown", { bubbles: true })); await o.settle();
  assert.equal(o.calls.filter(c => c.path === "/api/vault/touch").length, 0, "kasa yokken bildirilmez"); o.close();
});

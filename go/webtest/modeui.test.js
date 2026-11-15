"use strict";
/* Moda duyarlı arayüz: misafir (USB) modda Keychain kullanılmaz → sunucu sayfasında "Keychain parolası" adımı yerine şifreli kasa yönlendirmesi,
   knock dizisi düğmesi yok (saklanamaz); Keychain modunda (misafir değil) eski adımlar aynen durur. Bayrak: GET /api/about → guest. */
const test = require("node:test");
const assert = require("node:assert/strict");
const { boot, fixture } = require("./harness");

const about = guest => () => ({ ...fixture("about"), guest });
const base = (guest, o = {}) => ({ "/api/home": () => fixture("home"), "/api/about": about(guest), ...o });
const rows = h => h.$$("#srv-body .prep-row").map(r => r.querySelector(".prep-title").textContent.replace(/\s+/g, " ").trim());
const count = h => h.$("#srv-body .prep-count").textContent;

test("moda duyarlı: Keychain modunda (misafir değil) 'Keychain parolası' adımı ve knock düğmesi var, kasa yönlendirmesi yok", async () => {
  const h = await boot({ hash: "#s=web1", overrides: base(false) });
  assert.ok(rows(h).includes("Keychain parolası")); assert.ok(!rows(h).some(r => /şifreli kasa/.test(r)));
  assert.ok(h.$('#srv-body [data-task="knock"]'), "knock düğmesi var"); assert.equal(h.$("#srv-body [data-go-settings]"), null); // (Keychain düğmesi yalnız anahtarı olan sunucuda çıkar)
  assert.match(h.$("#srv-body .prep-count").title, /Keychain parolası/); assert.deepEqual(h.errors, []); h.close();
});

test("moda duyarlı: misafir modda 'Keychain parolası' yerine 'Passphrase (şifreli kasa)'; knock saklanamaz notu; ssh-agent adımı özel ajanı anlatır", async () => {
  const h = await boot({ hash: "#s=web1", overrides: base(true) });
  const r = rows(h); assert.ok(r.includes("Passphrase (şifreli kasa)")); assert.ok(!r.includes("Keychain parolası"), "misafirde Keychain adımı yok");
  assert.equal(h.$('#srv-body [data-task="keychain"]'), null, "Keychain düğmesi yok"); assert.ok(h.$('#srv-body [data-task="knock"]'), "knock dizisi şifreli kasada tutulduğu için düğme VAR");
  const txt = h.$("#srv-body .prep-list").textContent.replace(/\s+/g, " ");
  assert.match(txt, /Misafir \(USB\) modda Keychain kullanılmaz/); assert.match(txt, /dizi Keychain'e değil şifreli kasaya kaydedilir/); assert.match(txt, /bu oturumun özel ajanına yükler/); assert.doesNotMatch(txt, /\(macOS Keychain\)/);
  assert.match(h.$("#srv-body .prep-count").title, /şifreli kasadaki passphrase/); assert.deepEqual(h.errors, []); h.close();
});

test("moda duyarlı: 'Kasaya git' Ayarlar'ı açar; kasada kayıtlı sunucuda rozet çıkar ve adım hazır sayılır", async () => {
  const h = await boot({ hash: "#s=web1", overrides: base(true, { "/api/vault": () => ({ available: true, exists: true, unlocked: true, retryAfter: 0, idleMin: 15, stored: ["web1"] }) }) });
  const row = h.$$("#srv-body .prep-row").find(x => /şifreli kasa\)/.test(x.querySelector(".prep-title").textContent));
  assert.match(row.querySelector(".prep-badge").textContent, /kasada kayıtlı/);
  await h.click("#srv-body [data-go-settings]"); assert.equal(h.$(".view:not([hidden])").id, "view-settings"); assert.deepEqual(h.errors, []); h.close();
});

test("moda duyarlı: kasada kayıt yokken rozet yok; sayfa açıldıktan SONRA gelen kasa durumu/guest bayrağı sayfayı yeniden çizer", async () => {
  let st = { available: true, exists: true, unlocked: true, retryAfter: 0, idleMin: 15, stored: [] };
  const h = await boot({ hash: "#s=web1", overrides: base(true, { "/api/vault": () => st, "/api/vault/passphrase": (u, o) => { st = { ...st, stored: [o.body.server] }; return { ok: true }; } }) });
  const row = () => h.$$("#srv-body .prep-row").find(x => /şifreli kasa\)/.test(x.querySelector(".prep-title").textContent));
  assert.equal(row().querySelector(".prep-badge"), null, "kasada kayıt yok: rozet yok");
  // kasa durumu sayfa açıkken değişirse ('keylab:vaultstate' olayı) sunucu sayfası yeniden çizilir: passphrase eklenince rozet kendiliğinden çıkar
  h.$("#vault-srv").value = "web1"; h.$("#vault-pp").value = "gizli"; await h.click("#vault-add"); await h.settle();
  assert.match(row().querySelector(".prep-badge").textContent, /kasada kayıtlı/, "passphrase kasaya eklenince sunucu sayfası kendiliğinden güncellenir");
  assert.deepEqual(h.errors, []); h.close();
});

test("moda duyarlı: İngilizce arayüzde misafir metinleri çevrilir", async () => {
  const h = await boot({ hash: "#s=web1", lang: "en", overrides: base(true) });
  const r = rows(h); assert.ok(r.includes("Passphrase (encrypted vault)"), r.join("|")); assert.match(h.$("#srv-body .prep-list").textContent, /Keychain is not used/);
  assert.equal(h.$("#srv-body [data-go-settings]").textContent, "Go to the vault"); h.close();
});

test("eski oturum: önceki kapanış düzgün değilse temizlenenler açılışta bir kez bildirilir; temizlenen yoksa bildirim çıkmaz", async () => {
  let h = await boot({ hash: "", overrides: base(false, { "/api/about": () => ({ ...fixture("about"), guest: false, staleCleaned: { dirs: 2, agents: 1 } }) }) });
  assert.match(h.$("#toast").textContent, /Önceki oturum düzgün kapanmamıştı: 2 geçici klasör silindi, 1 özel ssh-agent durduruldu/); assert.deepEqual(h.errors, []); h.close();
  h = await boot({ hash: "", overrides: base(false, { "/api/about": () => ({ ...fixture("about"), guest: false, staleCleaned: { dirs: 0, agents: 0 } }) }) });
  assert.doesNotMatch(h.$("#toast").textContent, /düzgün kapanmamıştı/); h.close();
  h = await boot({ hash: "", lang: "en", overrides: base(false, { "/api/about": () => ({ ...fixture("about"), guest: false, staleCleaned: { dirs: 1, agents: 0 } }) }) });
  assert.match(h.$("#toast").textContent, /The previous session was not closed properly: 1 temporary folder\(s\) deleted, 0 private ssh-agent\(s\) stopped/); h.close();
});

/* ---- knock dizisi misafir modda şifreli kasada ---- */
const vaultSt = (o = {}) => ({ available: true, exists: true, unlocked: true, retryAfter: 0, idleMin: 15, stored: [], knock: [], ...o });
const knockPosts = h => h.calls.filter(c => c.method === "POST" && c.path === "/api/knock").map(c => c.body);
const ppPosts = h => h.calls.filter(c => c.method === "POST" && c.path === "/api/vault/passphrase").map(c => c.body);

test("kasa+knock: yalnız knock dizisi yazılırsa passphrase istenmez; /api/knock'a mevcut protokol/bekleme ile kaydedilir; liste 'knock' etiketi ve 'Knock'u sil' gösterir", async () => {
  let st = vaultSt();
  const h = await boot({ hash: "", overrides: base(true, { "/api/vault": () => st, "/api/knock": (u, o) => (o.method === "POST" ? (st = vaultSt({ knock: [o.body.name] }), { ok: true }) : { enabled: false, hasSeq: false, proto: "udp", delay: "500" }) }) });
  h.$("#vault-srv").value = "web1"; h.$("#vault-knock").value = "7001,7002,7003"; await h.click("#vault-add"); await h.settle();
  assert.deepEqual(ppPosts(h), [], "passphrase istenmedi"); assert.deepEqual(knockPosts(h), [{ name: "web1", action: "save", proto: "udp", delay: "500", seq: "7001,7002,7003" }]);
  assert.equal(h.$("#vault-knock").value, "", "dizi alanı temizlendi"); assert.match(h.$("#vault-msg").textContent, /web1: knock dizisi eklendi\./);
  const li = h.$$("#vault-stored li")[0]; assert.match(li.textContent, /web1/); assert.match(li.textContent, /knock/); assert.ok(li.querySelector("button[data-knock-rm]")); assert.equal(li.querySelector("button[data-srv]"), null);
  assert.deepEqual(h.errors, []); h.close();
});

test("kasa+knock: geçersiz dizi istek göndermez; boş (ikisi de yok) uyarır; passphrase+knock birlikte sırayla gider, knock hata verirse mesajda iki sonuç", async () => {
  const h = await boot({ hash: "", overrides: base(true, { "/api/vault": () => vaultSt(), "/api/vault/passphrase": () => ({ ok: true }), "/api/knock": (u, o) => (o.method === "POST" ? { status: 409, body: "misafir modda knock dizisi şifreli kasada tutulur" } : { enabled: false, hasSeq: false, proto: "tcp", delay: "300" }) }) });
  h.$("#vault-srv").value = "web1"; await h.click("#vault-add"); assert.match(h.$("#vault-msg").textContent, /passphrase ve\/ya da knock dizisi gerekli/); assert.equal(h.calls.filter(c => c.method === "POST").length, 0);
  h.$("#vault-knock").value = "70,abc"; await h.click("#vault-add"); assert.match(h.$("#vault-msg").textContent, /1-10 port olmalı/); assert.equal(h.calls.filter(c => c.method === "POST").length, 0);
  h.$("#vault-pp").value = "gizli"; h.$("#vault-knock").value = "7001,7002"; await h.click("#vault-add"); await h.settle();
  assert.deepEqual(ppPosts(h).map(b => b.server), ["web1"]); assert.equal(knockPosts(h).length, 1);
  assert.match(h.$("#vault-msg").textContent, /passphrase eklendi ve anahtar ajana yüklendi — knock dizisi kaydedilemedi: misafir modda knock dizisi şifreli kasada tutulur/); assert.ok(h.$("#vault-msg").classList.contains("bad"));
  h.close();
});

test("kasa+knock: 'Knock'u sil' /api/knock disable gönderir ve listeden kalkar", async () => {
  let st = vaultSt({ knock: ["web1"] });
  const h = await boot({ hash: "", overrides: base(true, { "/api/vault": () => st, "/api/knock": (u, o) => (o.method === "POST" ? (st = vaultSt(), { ok: true }) : { enabled: true, hasSeq: true, proto: "tcp", delay: "300" }) }) });
  await h.click("#vault-stored button[data-knock-rm]"); await h.settle();
  assert.deepEqual(knockPosts(h), [{ name: "web1", action: "disable" }]); assert.match(h.$("#vault-msg").textContent, /knock dizisi kasadan silindi/); assert.equal(h.$("#vault-stored button[data-knock-rm]"), null);
  assert.deepEqual(h.errors, []); h.close();
});

test("kasa+knock: misafir modda knock penceresi diziyi 'şifreli kasada' kayıtlı der (Keychain demez); Keychain modunda eski metin; sunucu sayfası satırı 'kasada tutulur' der", async () => {
  const knockOn = { enabled: true, hasSeq: true, proto: "tcp", delay: "300", hint: "7…1" };
  const srvOv = { "/api/server": u => ({ ...fixture("server-" + u.searchParams.get("name")), knock: knockOn }), "/api/knock": knockOn };
  let h = await boot({ hash: "#s=web1", overrides: base(true, srvOv) });
  assert.match(h.$("#srv-body .prep-list").textContent.replace(/\s+/g, " "), /Dizi şifreli kasada tutulur; kasa açıkken kullanılır/);
  await h.click('#srv-body [data-task="knock"]'); const dlg = () => h.$("#dlg").textContent.replace(/\s+/g, " ");
  assert.match(dlg(), /Dizi şifreli kasada kayıtlı/); assert.doesNotMatch(dlg(), /Keychain/); h.close();
  h = await boot({ hash: "#s=web1", overrides: base(false, srvOv) });
  await h.click('#srv-body [data-task="knock"]'); assert.match(h.$("#dlg").textContent.replace(/\s+/g, " "), /Dizi bu bilgisayarın Keychain'inde kayıtlı/); h.close();
});

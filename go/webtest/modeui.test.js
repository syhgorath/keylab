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
  assert.equal(h.$('#srv-body [data-task="keychain"]'), null, "Keychain düğmesi yok"); assert.equal(h.$('#srv-body [data-task="knock"]'), null, "saklanamayan knock için düğme yok");
  const txt = h.$("#srv-body .prep-list").textContent.replace(/\s+/g, " ");
  assert.match(txt, /Misafir \(USB\) modda Keychain kullanılmaz/); assert.match(txt, /knock dizisi saklanamaz/); assert.match(txt, /bu oturumun özel ajanına yükler/); assert.doesNotMatch(txt, /\(macOS Keychain\)/);
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

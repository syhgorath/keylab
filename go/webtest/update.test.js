"use strict";
/* Uygulama içi güncelleme (js/ui/update.js): sürüm gösterimi, elle denetim, GitHub sürüm notları (güvenli çizim), rozet, onaylı güncelleme akışı. */
const test = require("node:test");
const assert = require("node:assert/strict");
const { boot, fixture } = require("./harness");

const about = v => () => ({ ...fixture("about"), version: v });
const INFO = (o = {}) => ({ current: "4.0.10", latest: "4.0.11", newer: true, canApply: true, notes: "### Düzeltildi\n- **Önemli** hata düzeldi: `keylab update` artık çalışır\n- ikinci madde\n\nDüz paragraf.", ...o });
const base = (ov = {}) => ({ "/api/home": () => fixture("home"), "/api/about": about("4.0.10"), ...ov });
const calls = (h, p, m) => h.calls.filter(c => c.path === p && (!m || c.method === m));
const text = (h, s) => h.$(s).textContent.replace(/\s+/g, " ").trim();

test("güncelleme: kurulu sürüm kenar çubuğunda ve Güncelleme kartında görünür; sürüm yoksa eski metin", async () => {
  let h = await boot({ hash: "", overrides: base() });
  assert.equal(h.$("#brand-sub").textContent, "v4.0.10"); assert.equal(h.$("#appupd-cur").textContent, "v4.0.10"); assert.deepEqual(h.errors, []); h.close();
  h = await boot({ hash: "", overrides: base({ "/api/about": about("") }) });
  assert.equal(h.$("#brand-sub").textContent, "yerel · taslak"); h.close();
});

test("güncelleme: açılışta ağ isteği YOK; 'Güncellemeyi denetle' yeni sürümü, notları (güvenli DOM) ve rozeti gösterir", async () => {
  const h = await boot({ hash: "#v=settings", overrides: base({ "/api/update/check": () => INFO({ notes: INFO().notes + "\n<img src=x onerror=alert(1)> **kalın** <script>x</script>" }) }) });
  assert.equal(calls(h, "/api/update/check").length, 0, "KeyLab kendiliğinden internete çıkmaz"); assert.equal(h.$("#appupd-badge").hidden, true); assert.equal(h.$("#appupd-result").hidden, true);
  await h.click("#appupd-check"); await h.settle();
  assert.equal(calls(h, "/api/update/check").length, 1); assert.equal(text(h, "#appupd-line"), "Yeni sürüm var: v4.0.11 (kurulu: v4.0.10)"); assert.equal(h.$("#appupd-notes-h").textContent, "v4.0.11 sürüm notları");
  assert.equal(h.$("#appupd-notes h5").textContent, "Düzeltildi"); assert.equal(h.$$("#appupd-notes li").length, 2); assert.equal(h.$("#appupd-notes li b").textContent, "Önemli"); assert.equal(h.$("#appupd-notes li code").textContent, "keylab update");
  assert.equal(h.$("#appupd-notes img"), null, "sürüm notundaki HTML yorumlanmaz"); assert.equal(h.$("#appupd-notes script"), null); assert.match(h.$("#appupd-notes").textContent, /<img src=x onerror=alert\(1\)>/, "etiket düz metin olarak görünür");
  assert.equal(h.$("#appupd-badge").hidden, false); assert.equal(h.$("#appupd-badge").textContent, "⬆ v4.0.11"); assert.equal(h.$("#appupd-apply").hidden, false); assert.equal(h.$("#appupd-why").hidden, true);
  await h.click("#appupd-badge"); assert.equal(h.$(".view:not([hidden])").id, "view-settings");
  assert.deepEqual(h.errors, []); h.close();
});

test("güncelleme: güncelse rozet ve Güncelle düğmesi yok ama son sürümün notları görünür; çevrimdışı/hata mesajı gösterilir", async () => {
  let mode = "ok";
  const h = await boot({ hash: "#v=settings", overrides: base({ "/api/update/check": () => (mode === "ok" ? INFO({ latest: "4.0.10", newer: false, canApply: false, notes: "mevcut sürüm notları" }) : mode === "offline" ? { status: 502, body: { error: "offline" } } : { status: 500, body: "x" }) }) });
  await h.click("#appupd-check"); await h.settle();
  assert.equal(text(h, "#appupd-line"), "Güncelsiniz: v4.0.10 (son sürüm v4.0.10)"); assert.equal(h.$("#appupd-badge").hidden, true); assert.equal(h.$("#appupd-apply").hidden, true); assert.equal(text(h, "#appupd-notes"), "mevcut sürüm notları");
  mode = "offline"; await h.click("#appupd-check"); await h.settle(); assert.match(h.$("#appupd-state").textContent, /internet yok mu/); assert.equal(h.$("#appupd-result").hidden, true); assert.ok(h.$("#appupd-state").classList.contains("bad"));
  mode = "err"; await h.click("#appupd-check"); await h.settle(); assert.equal(h.$("#appupd-state").textContent, "Denetlenemedi."); assert.equal(h.$("#appupd-check").disabled, false);
  assert.deepEqual(h.errors.filter(e => !/500|fixture/.test(e)), []); h.close();
});

test("güncelleme: uygulanamıyorsa (komut satırı/translocation/salt okunur) Güncelle düğmesi yerine nedeni yazar", async () => {
  for (const [reason, re] of [["not_bundle", /`keylab update` kullanın/], ["translocated", /Uygulamalar'a \(ya da USB'ye\) taşıyıp/], ["not_writable", /yazılamıyor/]]) {
    const h = await boot({ hash: "#v=settings", overrides: base({ "/api/update/check": () => INFO({ canApply: false, reason }) }) });
    await h.click("#appupd-check"); await h.settle();
    assert.equal(h.$("#appupd-apply").hidden, true, reason); assert.equal(h.$("#appupd-why").hidden, false); assert.match(h.$("#appupd-why").textContent, re); assert.equal(h.$("#appupd-badge").hidden, false, "yeni sürüm var: rozet yine görünür");
    assert.deepEqual(h.errors, []); h.close();
  }
});

test("güncelleme: Güncelle → onay penceresi → POST {force:false}; adımlar durum ucundan gösterilir; başarıda 'yeniden başlatılıyor'", async () => {
  let release; const gate = new Promise(r => { release = r; });
  const h = await boot({ hash: "#v=settings", overrides: base({ "/api/update/check": () => INFO(), "/api/update/status": () => ({ state: "running", step: "imza doğrulanıyor" }),
    "/api/update/apply": async () => { await gate; return { ok: true, restarting: true, version: "4.0.11" }; } }) });
  await h.click("#appupd-check"); await h.settle();
  await h.click("#appupd-apply"); assert.ok(h.$("#dlg").classList.contains("open")); assert.equal(h.$("#dlg-title").textContent, "Güncelle ve yeniden başlat"); assert.match(h.$("#dlg").textContent.replace(/\s+/g, " "), /imzası doğrulanacak/);
  assert.equal(calls(h, "/api/update/apply").length, 0, "onaylanmadan istek gitmez");
  h.$("#dlg-ok").dispatchEvent(new h.win.MouseEvent("click", { bubbles: true, cancelable: true })); // POST bekler: settle çağrılmaz
  h.timers.run(600); for (let i = 0; i < 6; i++) await new Promise(r => setImmediate(r));
  assert.deepEqual(calls(h, "/api/update/apply", "POST").map(c => c.body), [{ force: false }]); assert.equal(h.$("#appupd-apply").disabled, true); assert.equal(h.$("#appupd-state").textContent, "İmza doğrulanıyor…");
  release(); await h.settle();
  assert.equal(h.$("#appupd-state").textContent, "Güncellendi: v4.0.11. KeyLab yeniden başlatılıyor…"); assert.equal(h.$("#appupd-apply").hidden, true);
  const n = calls(h, "/api/update/status").length; h.timers.run(5000); await h.settle(); assert.equal(calls(h, "/api/update/status").length, n, "bitince yoklama durur");
  assert.deepEqual(h.errors, []); h.close();
});

test("güncelleme: çalışan iş varsa (409 running) ikinci uyarılı onay, ardından {force:true}; hata mesajları gösterilir", async () => {
  let n = 0;
  const h = await boot({ hash: "#v=settings", overrides: base({ "/api/update/check": () => INFO(), "/api/update/status": () => ({ state: "" }),
    "/api/update/apply": () => (++n === 1 ? { status: 409, body: { error: "running", jobs: 2 } } : n === 2 ? { status: 500, body: { error: "failed", message: "İMZA DOĞRULANAMADI: paket kurulmadı" } } : { status: 409, body: { error: "translocated" } }) }) });
  await h.click("#appupd-check"); await h.settle(); await h.click("#appupd-apply"); await h.click("#dlg-ok"); await h.settle();
  assert.deepEqual(calls(h, "/api/update/apply", "POST").map(c => c.body), [{ force: false }]); assert.ok(h.$("#dlg").classList.contains("open"));
  assert.match(h.$("#dlg").textContent.replace(/\s+/g, " "), /2 iş çalışıyor: çıkarsanız yarıda kesilir/); assert.equal(h.$("#dlg-ok").textContent, "Yine de güncelle");
  await h.click("#dlg-ok"); await h.settle();
  assert.deepEqual(calls(h, "/api/update/apply", "POST").map(c => c.body), [{ force: false }, { force: true }]); assert.match(h.$("#appupd-state").textContent, /İMZA DOĞRULANAMADI/); assert.equal(h.$("#appupd-apply").disabled, false, "hata sonrası yeniden denenebilir");
  await h.click("#appupd-apply"); await h.click("#dlg-ok"); await h.settle(); assert.match(h.$("#appupd-state").textContent, /karantina nedeniyle/);
  assert.deepEqual(h.errors.filter(e => !/500|409|fixture/.test(e)), []); h.close();
});

// Regresyon: Paketler sayfası kendi #upd-card / #upd-check öğelerini dinamik çizer; uygulama güncelleme kartı bunlarla ÇAKIŞMAMALI (aynı kimlik iki kez olursa
// packages.js onun yerine Ayarlar'daki kartı bulup içini ezerdi).
test("güncelleme: sayfada aynı kimlik iki kez yok (Paketler sayfası çizilmişken de); Paketler'in kendi güncelleme kartı bozulmaz", async () => {
  const h = await boot({ hash: "#v=packages", overrides: base() });
  const ids = h.$$("[id]").map(e => e.id), dup = ids.filter((x, i) => ids.indexOf(x) !== i);
  assert.deepEqual([...new Set(dup)], [], "yinelenen kimlikler");
  assert.ok(h.$("#view-packages #upd-card .card"), "Paketler sayfasının paket güncelleme kartı kendi yerinde çizildi");
  assert.ok(h.$("#appupd-card #appupd-check") && !h.$("#view-settings #upd-check"), "uygulama güncelleme kartı ayrı kimliklerde");
  assert.deepEqual(h.errors, []); h.close();
});

test("güncelleme: İngilizce arayüzde kart, mesajlar ve sürüm notu başlığı çevrilir", async () => {
  const h = await boot({ hash: "#v=settings", lang: "en", overrides: base({ "/api/update/check": () => INFO() }) });
  assert.equal(h.$("#appupd-card h2").textContent, "Update"); assert.equal(h.$("#appupd-check").textContent, "Check for updates");
  await h.click("#appupd-check"); await h.settle();
  assert.equal(text(h, "#appupd-line"), "New version available: v4.0.11 (installed: v4.0.10)"); assert.equal(h.$("#appupd-notes-h").textContent, "Release notes for v4.0.11"); assert.equal(h.$("#appupd-apply").textContent, "Update and restart"); h.close();
});

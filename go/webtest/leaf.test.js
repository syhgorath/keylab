"use strict";
/* Refaktör 2b: js/ui/shortcuts.js, context-menu.js, ssh-import.js için davranış testleri (tüm uygulama içinde, kullanıcı etkileşimiyle).
   Anlık görüntüler bu pencerelerin AÇIK hâlini ve etkileşimlerini kapsamaz; burada açık beklentiler var. */
const test = require("node:test");
const assert = require("node:assert/strict");
const { boot, fixture } = require("./harness");

const mixedHome = () => { const h = fixture("home"); h.servers.forEach((s, i) => { s.checked = true; s.up = i !== 1; s.conn = s.up ? "up" : "down"; }); return h; };
const key = (h, k, extra = {}, target) => { (target || h.doc).dispatchEvent(new h.win.KeyboardEvent("keydown", { key: k, bubbles: true, cancelable: true, ...extra })); return h.settle(); };
const open = (h, sel) => h.$(sel).classList.contains("open");

// ---------------- klavye kısayolları penceresi ----------------
test("kısayollar: ? tuşu açar/kapatır, odak açılışta Kapat düğmesine gider ve kapanınca geri döner", async () => {
  const h = await boot({ overrides: { "/api/home": mixedHome } });
  const before = h.$("#search-input"); before.focus();
  assert.ok(!open(h, "#shortcuts-modal"));
  // arama kutusundayken ? yazı olarak girilir: pencere açılmaz
  await key(h, "?", {}, before); assert.ok(!open(h, "#shortcuts-modal"), "metin alanında açılmamalı");
  before.blur(); h.doc.body.focus();
  await key(h, "?"); assert.ok(open(h, "#shortcuts-modal")); assert.equal(h.doc.activeElement.id, "sc-close");
  await key(h, "?"); assert.ok(!open(h, "#shortcuts-modal"), "ikinci ? kapatır");
  assert.deepEqual(h.errors, []); h.close();
});

test("kısayollar: düğme açar; Kapat, ✕, arka plan ve Esc kapatır; ctrl/meta/alt ile açılmaz; başka pencere açıkken açılmaz", async () => {
  const h = await boot({ overrides: { "/api/home": mixedHome } });
  const opener = h.$("#shortcuts-btn"); opener.focus();
  const reopen = async () => { await h.click("#shortcuts-btn"); assert.ok(open(h, "#shortcuts-modal")); };
  await reopen(); await h.click("#sc-close"); assert.ok(!open(h, "#shortcuts-modal"));
  assert.equal(h.doc.activeElement, opener, "odak açan öğeye döner");
  await reopen(); await h.click("#sc-x"); assert.ok(!open(h, "#shortcuts-modal"));
  await reopen(); await h.click("#shortcuts-modal"); assert.ok(!open(h, "#shortcuts-modal"), "arka plana tıklama kapatır");
  await reopen(); await h.click(".modal", ); // iç tıklama (ilk .modal başka pencerenin olabilir): kapanmamalı
  await reopen().catch(() => {});
  await key(h, "Escape"); assert.ok(!open(h, "#shortcuts-modal"), "Esc kapatır");
  for (const mod of [{ ctrlKey: true }, { metaKey: true }, { altKey: true }]) { await key(h, "?", mod); assert.ok(!open(h, "#shortcuts-modal"), JSON.stringify(mod)); }
  h.$("#runmodal").classList.add("open");                      // başka pencere açık
  await key(h, "?"); assert.ok(!open(h, "#shortcuts-modal"), "başka pencere açıkken ? açmaz");
  assert.deepEqual(h.errors, []); h.close();
});

// ---------------- bağlam menüsü ----------------
const tile = h => h.$$("#servers [data-name]").find(el => el.dataset.name === "web1");
const ctxOpen = async (h, x = 300, y = 200) => { tile(h).dispatchEvent(new h.win.MouseEvent("contextmenu", { bubbles: true, cancelable: true, clientX: x, clientY: y })); await h.settle(); };

test("bağlam menüsü: sağ tık açar (başlık = sunucu adı, ilk düğmeye odak) ve konumu pencere içine sıkıştırır", async () => {
  const h = await boot({ overrides: { "/api/home": mixedHome } });
  await ctxOpen(h, 300, 200);
  assert.ok(open(h, "#ctx")); assert.equal(h.$("#ctx-head").textContent, "web1"); assert.equal(h.doc.activeElement, h.$("#ctx button"));
  assert.equal(h.$("#ctx").style.left, "300px"); assert.equal(h.$("#ctx").style.top, "200px");
  await ctxOpen(h, 5000, -40);   // jsdom'da menü genişliği 0 → pencere (1024x768) kenarından 8 px içeride
  assert.equal(h.$("#ctx").style.left, `${h.win.innerWidth - 8}px`); assert.equal(h.$("#ctx").style.top, "8px");
  assert.deepEqual(h.errors, []); h.close();
});

test("bağlam menüsü: F10 / ContextMenu tuşu ve ⋯ düğmesi açar", async () => {
  const h = await boot({ overrides: { "/api/home": mixedHome } });
  await key(h, "F10", {}, tile(h)); assert.ok(open(h, "#ctx")); assert.equal(h.$("#ctx-head").textContent, "web1");
  h.$("#ctx").classList.remove("open");
  await key(h, "ContextMenu", {}, tile(h)); assert.ok(open(h, "#ctx"));
  h.$("#ctx").classList.remove("open");
  const menuBtn = tile(h).querySelector('[data-act="menu"]'); assert.ok(menuBtn, "⋯ düğmesi var");
  await h.click(menuBtn); assert.ok(open(h, "#ctx"), "⋯ menüyü açar (kartın kendi tıklaması sunucuya gitmez)"); assert.equal(h.$("#view-server").hidden, true);
  assert.deepEqual(h.errors, []); h.close();
});

test("bağlam menüsü: ok tuşları odağı döngüyle gezdirir; dışarı tıklama ve Esc kapatır, iç tıklama kapatmaz", async () => {
  const h = await boot({ overrides: { "/api/home": mixedHome } });
  await ctxOpen(h);
  const btns = h.$$("#ctx button"), n = btns.length; assert.ok(n >= 5);
  assert.equal(h.doc.activeElement, btns[0]);
  await key(h, "ArrowDown", {}, h.doc.activeElement); assert.equal(h.doc.activeElement, btns[1]);
  await key(h, "ArrowUp", {}, h.doc.activeElement); assert.equal(h.doc.activeElement, btns[0]);
  await key(h, "ArrowUp", {}, h.doc.activeElement); assert.equal(h.doc.activeElement, btns[n - 1], "yukarıdan başa sarar");
  await key(h, "ArrowDown", {}, h.doc.activeElement); assert.equal(h.doc.activeElement, btns[0], "aşağıdan başa sarar");
  await h.click("#ctx-head"); assert.ok(open(h, "#ctx"), "menü içine tıklama kapatmaz");
  await h.click("#view-dash h2"); assert.ok(!open(h, "#ctx"), "dışarı tıklama kapatır");
  await ctxOpen(h); await key(h, "Escape"); assert.ok(!open(h, "#ctx"), "Esc kapatır");
  assert.deepEqual(h.errors, []); h.close();
});

test("bağlam menüsü: eylemler (yenile, günlükler, komutu kopyala) menüyü kapatıp doğru işi yapar", async () => {
  const h = await boot({ overrides: { "/api/home": mixedHome, "/api/action": { cmd: "ssh root@10.0.0.1" } } });
  const homeCalls = () => h.calls.filter(c => c.path === "/api/home").length;
  const pick = a => h.click(`#ctx button[data-ctx="${a}"]`);
  await ctxOpen(h); const n0 = homeCalls(); await pick("refresh");
  assert.ok(!open(h, "#ctx")); assert.equal(homeCalls(), n0 + 1); assert.equal(h.$("#toast").textContent, "Yenilendi");
  await ctxOpen(h); await pick("logs"); assert.equal(h.$(".view:not([hidden])").id, "view-logs");
  await h.click('[data-nav="dash"]');
  await ctxOpen(h); await pick("copy");
  assert.deepEqual(h.calls.filter(c => c.path === "/api/action").map(c => c.body), [{ name: "web1", action: "cmd" }]);
  assert.deepEqual(h.win.__clip, ["ssh root@10.0.0.1"]); assert.equal(h.$("#toast").textContent, "Komut kopyalandı");
  assert.deepEqual(h.errors, []); h.close();
});

test("bağlam menüsü: eylem hatası bildirim olarak gösterilir (menü yine kapanır)", async () => {
  const h = await boot({ overrides: { "/api/home": mixedHome, "/api/action": { status: 500, body: "boom" } } });
  await ctxOpen(h); await h.click('#ctx button[data-ctx="copy"]');
  assert.ok(!open(h, "#ctx")); assert.equal(h.$("#toast").textContent, "Hata: boom");
  assert.deepEqual(h.errors, []); h.close();
});

// ---------------- ssh config içe aktarma ----------------
const hosts = [
  { alias: "web1", name: "web1", host: "10.0.0.1", user: "root", port: "22", added: false },
  { alias: "db-1", name: "db_1", host: "10.0.0.2", user: "", port: "", added: false },
  { alias: "old", name: "old", host: "10.0.0.3", user: "admin", port: "2222", added: true },
  { alias: "<b>x</b>", name: "x", host: "h", user: "", port: "", added: false },
];
const importOv = extra => ({ "/api/home": mixedHome, "/api/ssh-config-hosts": { exists: true, hosts }, ...extra });
const goAdd = async h => { await h.click('[data-nav="servers"]'); await h.click('#view-servers [data-nav-target="add"]'); };
const openImport = async h => { await goAdd(h); await h.click("#imp-open"); };

test("ssh içe aktarma: liste çizilir (eklenmiş olan seçilemez, takma ad ≠ ad gösterilir, HTML kaçışlanır) ve seçime göre durum güncellenir", async () => {
  const h = await boot({ overrides: importOv() });
  await openImport(h);
  assert.ok(open(h, "#import-modal"));
  const boxes = h.$$("#imp-list input[data-alias]"); assert.equal(boxes.length, 4);
  assert.equal(boxes.find(b => b.dataset.alias === "old").disabled, true, "ekli sunucu seçilemez");
  assert.match(h.$("#imp-list").textContent, /db-1.*→ db_1/); assert.match(h.$("#imp-list").textContent, /10\.0\.0\.3 · admin · :2222/);
  assert.equal(h.$$("#imp-list b").filter(b => b.textContent === "<b>x</b>").length, 1, "takma ad metin olarak kalır");
  assert.equal(h.$$("#imp-list b > b").length, 0, "enjekte edilmiş <b> elemanı olmamalı");
  assert.equal(h.$("#imp-go").disabled, true); assert.equal(h.$("#imp-status").textContent, "");
  await h.click(boxes[0]); assert.equal(h.$("#imp-go").disabled, false); assert.equal(h.$("#imp-status").textContent, "1 seçili");
  await h.click(boxes[1]); assert.equal(h.$("#imp-status").textContent, "2 seçili");
  await h.click(boxes[0]); await h.click(boxes[1]); assert.equal(h.$("#imp-go").disabled, true);
  assert.deepEqual(h.errors, []); h.close();
});

test("ssh içe aktarma: yalnız seçilen takma adlar gönderilir; sonuç özeti gösterilir ve liste yenilenir", async () => {
  const h = await boot({ overrides: importOv({ "/api/ssh-config-import": { results: [{ alias: "web1", status: "imported" }, { alias: "db-1", status: "skipped" }, { alias: "<b>x</b>", status: "error", error: "geçersiz <i>ad</i>" }] } }) });
  await openImport(h);
  const homeBefore = h.calls.filter(c => c.path === "/api/home").length;
  for (const a of ["web1", "db-1", "<b>x</b>"]) await h.click(h.$$("#imp-list input[data-alias]").find(b => b.dataset.alias === a));
  await h.click("#imp-go");
  const post = h.calls.find(c => c.path === "/api/ssh-config-import"); assert.deepEqual(post.body, { aliases: ["web1", "db-1", "<b>x</b>"] });
  const txt = h.$("#imp-list").textContent; assert.match(txt, /1.*sunucu eklendi.*1.*zaten vardı.*1.*hata/);
  assert.match(txt, /geçersiz <i>ad<\/i>/, "hata metni kaçışlı"); assert.equal(h.$$("#imp-list i").length, 0);
  assert.equal(h.$("#imp-go").hidden, true); assert.equal(h.calls.filter(c => c.path === "/api/home").length, homeBefore + 1, "sunucu listesi yenilenir");
  assert.deepEqual(h.errors, []); h.close();
});

test("ssh içe aktarma: dosya yok / boş / sunucu hatası iletileri; hata sonrası yeniden denenebilir", async () => {
  let mode = "none";
  const ov = importOv({ "/api/ssh-config-hosts": () => (mode === "none" ? { exists: false, hosts: [] } : mode === "empty" ? { exists: true, hosts: [] } : { status: 500, body: "dosya okunamadı" }) });
  const h = await boot({ overrides: ov });
  await openImport(h); assert.match(h.$("#imp-list").textContent, /dosyası bulunamadı/); await h.click("#imp-cancel"); assert.ok(!open(h, "#import-modal"));
  mode = "empty"; await h.click("#imp-open"); assert.match(h.$("#imp-list").textContent, /içe aktarılabilir Host kaydı yok/); await h.click("#imp-cancel");
  mode = "err"; await h.click("#imp-open"); assert.match(h.$("#imp-list").textContent, /^Hata: dosya okunamadı$/); assert.equal(h.$("#imp-go").disabled, true);
  assert.deepEqual(h.errors, []); h.close();
});

test("ssh içe aktarma: içe aktarma isteği başarısız olursa düğme yeniden etkinleşir ve hata gösterilir", async () => {
  const h = await boot({ overrides: importOv({ "/api/ssh-config-import": { status: 500, body: "yazılamadı" } }) });
  await openImport(h); await h.click(h.$$("#imp-list input[data-alias]")[0]); await h.click("#imp-go");
  assert.match(h.$("#imp-status").textContent, /Hata: yazılamadı/); assert.equal(h.$("#imp-go").disabled, false);
  assert.deepEqual(h.errors, []); h.close();
});

test("ssh içe aktarma: Esc, ✕ ve arka plan kapatır; odak açan düğmeye döner; Karşılama kartından da açılır", async () => {
  const h = await boot({ overrides: importOv() });
  await goAdd(h); const opener = h.$("#imp-open"); opener.focus();
  await h.click(opener); assert.ok(open(h, "#import-modal"));
  await key(h, "Escape"); assert.ok(!open(h, "#import-modal")); assert.equal(h.doc.activeElement, opener);
  await h.click(opener); await h.click("#imp-x"); assert.ok(!open(h, "#import-modal"));
  await h.click(opener); await h.click("#import-modal"); assert.ok(!open(h, "#import-modal"));
  // karşılama kartı (sunucu yok): "~/.ssh/config'ten içe aktar" → Sunucu ekle sayfası + pencere
  const e = await boot({ overrides: { ...importOv(), "/api/home": () => ({ ...fixture("home"), servers: [] }) } });
  await e.click("#ob-import"); assert.equal(e.$(".view:not([hidden])").id, "view-add"); assert.ok(open(e, "#import-modal"));
  assert.deepEqual(h.errors.concat(e.errors), []); h.close(); e.close();
});

test("ssh içe aktarma: çift tırnaklı takma ad öznitelikte kırılmaz, gönderilen ad birebir aynıdır", async () => {
  const quoted = { alias: 'q"uote', name: "q_uote", host: "10.0.0.9", user: "", port: "", added: false };
  const h = await boot({ overrides: importOv({ "/api/ssh-config-hosts": { exists: true, hosts: [quoted] }, "/api/ssh-config-import": { results: [{ alias: 'q"uote', status: "imported" }] } }) });
  await openImport(h);
  const box = h.$("#imp-list input[data-alias]"); assert.equal(box.dataset.alias, 'q"uote', "öznitelik değeri tırnakla kesilmemeli");
  await h.click(box); await h.click("#imp-go");
  assert.deepEqual(h.calls.find(c => c.path === "/api/ssh-config-import").body, { aliases: ['q"uote'] });
  assert.deepEqual(h.errors, []); h.close();
});

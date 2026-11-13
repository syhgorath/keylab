"use strict";
/* Refaktör 3b: paylaşılan durum (js/state.js → appState). Dokuz alanın (home, srv, srvName, srvTab, view, mode, selected, routing,
   query) sürdüğü DAVRANIŞLAR: arama, görünüm düğmesi, sekmeler, gezinme geçmişi, eski yanıtın yenisini ezmemesi, Lynis seçimi.
   (Anlık görüntüler bu davranışları yalnız son hâl olarak görür; burada etkileşim ve sıra denetlenir.) */
process.env.TZ = "UTC";
const test = require("node:test");
const assert = require("node:assert/strict");
const path = require("path");
const { pathToFileURL } = require("url");
const { boot, fixture } = require("./harness");

const mixedHome = () => { const h = fixture("home"); h.servers.forEach((s, i) => { s.checked = true; s.up = i !== 1; s.conn = s.up ? "up" : "down"; }); return h; };
const status = () => ({ db1: { up: true, conn: "up" }, vm1: { up: false, conn: "down" }, web1: { up: true, conn: "up" } });
const ov = extra => ({ "/api/home": mixedHome, "/api/status": status, ...extra });
const names = h => h.$$("#servers [data-name]").map(e => e.dataset.name);
const view = h => h.$(".view:not([hidden])").id;
const tick = ms => new Promise(r => setTimeout(r, ms));   // gerçek (Node) zamanlayıcı: jsdom geçmiş gezintisi için

test("appState: belgelenmiş dokuz alan ve varsayılanları", async () => {
  const { appState } = await import(pathToFileURL(path.join(__dirname, "..", "web", "js", "state.js")).href);
  assert.deepEqual(Object.keys(appState).sort(), ["askAvail", "askMode", "home", "mode", "query", "routing", "selected", "srv", "srvName", "srvTab", "view"]);
  assert.deepEqual(appState.home, { servers: [], packages: [], indexSerial: "" });
  assert.deepEqual([appState.view, appState.mode, appState.query, appState.selected, appState.srvName, appState.srvTab, appState.srv, appState.routing, appState.askMode, appState.askAvail], ["dash", "tiles", "", "", "", "", null, false, "system", false]);
});

test("arama: sorgu sunucu listesini süzer, boş sonuç iletisi gösterir, temizlenince geri gelir; / tuşu kutuya odaklanır", async () => {
  const h = await boot({ overrides: ov() });
  assert.deepEqual(names(h).sort(), ["db1", "vm1", "web1"]);
  await h.type("#search-input", "WEB"); assert.deepEqual(names(h), ["web1"], "büyük/küçük harf duyarsız");
  await h.type("#search-input", "  vm1 "); assert.deepEqual(names(h), ["vm1"], "kırpılır");
  await h.type("#search-input", "linux-hardening"); assert.deepEqual(names(h), ["web1"], "son çalışan paket adıyla da bulunur");
  await h.type("#search-input", "yokboyle"); assert.deepEqual(names(h), []); assert.match(h.$("#servers").textContent, /Sunucu bulunamadı/);
  await h.type("#search-input", ""); assert.deepEqual(names(h).sort(), ["db1", "vm1", "web1"]);
  h.$("#search-input").blur(); h.doc.body.focus();
  h.doc.dispatchEvent(new h.win.KeyboardEvent("keydown", { key: "/", bubbles: true, cancelable: true })); await h.settle();
  assert.equal(h.doc.activeElement.id, "search-input");
  assert.deepEqual(h.errors, []); h.close();
});

test("arama: Sunucular sayfasındaki listeyi de süzer (aynı sorgu)", async () => {
  const h = await boot({ hash: "#v=servers", overrides: ov() });
  await h.type("#search-input", "db1");
  assert.deepEqual(h.$$("#servers-all [data-name]").map(e => e.dataset.name), ["db1"]);
  assert.deepEqual(h.errors, []); h.close();
});

test("görünüm düğmesi: Simgeler/Liste geçişi listeyi yeniden çizer ve tercihi saklar; Ayarlar'daki düğme de aynı durumu değiştirir", async () => {
  const h = await boot({ overrides: ov() });
  assert.ok(h.$("#servers").classList.contains("tiles")); assert.ok(h.$("#servers .stile"));
  await h.click('.mode-toggle button[data-mode="list"]');
  assert.ok(!h.$("#servers").classList.contains("tiles")); assert.equal(h.$$("#servers li.server").length, 3); assert.equal(h.$$("#servers .stile").length, 0);
  assert.equal(h.win.localStorage.getItem("sshpocket.mode"), "list");
  assert.equal(h.$('.mode-toggle button[data-mode="list"]').getAttribute("aria-pressed"), "true");
  await h.click('.mode-toggle button[data-mode="tiles"]'); assert.ok(h.$("#servers").classList.contains("tiles")); assert.equal(h.win.localStorage.getItem("sshpocket.mode"), "tiles");
  await h.click('[data-nav="settings"]'); await h.click('#set-mode button[data-v="list"]');
  await h.click('[data-nav="dash"]'); assert.ok(!h.$("#servers").classList.contains("tiles"), "Ayarlar'dan seçilen görünüm Pano'ya yansır");
  assert.deepEqual(h.errors, []); h.close();
});

test("sunucu sayfası: sekme düğmeleri sekmeyi değiştirir, adres çubuğunu GEÇMİŞ ÇOĞALTMADAN günceller", async () => {
  const h = await boot({ hash: "#s=web1", overrides: ov() });
  assert.equal(view(h), "view-server"); assert.equal(h.$('#srv-tabs [aria-selected="true"]').dataset.p, "ozet");
  const len = h.win.history.length;
  await h.click('#srv-tabs button[data-p="utility"]');
  assert.equal(h.$('#srv-tabs [aria-selected="true"]').dataset.p, "utility"); assert.equal(h.win.location.hash, "#s=web1&p=utility");
  assert.match(h.$("#srv-body").textContent, /Durumu oku|durum/i, "utility paketinin eylemleri çizilir");
  await h.click('#srv-tabs button[data-p="kurulum"]'); assert.equal(h.win.location.hash, "#s=web1&p=kurulum");
  await h.click('#srv-tabs button[data-p="ozet"]'); assert.equal(h.win.location.hash, "#s=web1&p=ozet");
  assert.equal(h.win.history.length, len, "sekme değişimi geçmişe kayıt eklemez");
  assert.deepEqual(h.errors, []); h.close();
});

test("sunucu sayfası: bilinmeyen sekme özet'e düşer; Özet'teki 'git' bağlantıları ilgili sekmeye götürür", async () => {
  const h = await boot({ hash: "#s=web1&p=olmayan", overrides: ov() });
  assert.equal(h.$('#srv-tabs [aria-selected="true"]').dataset.p, "ozet");
  const go = h.$$("#srv-body [data-goto]"); assert.ok(go.length > 0, "özet sayfasında sekmeye geçiş bağlantıları var");
  await h.click(go[0]); assert.equal(h.$('#srv-tabs [aria-selected="true"]').dataset.p, go[0].dataset.goto);
  assert.equal(h.win.location.hash, `#s=web1&p=${go[0].dataset.goto}`);
  assert.deepEqual(h.errors, []); h.close();
});

test("gezinme geçmişi: yeni sayfa geçmişe EKLENİR, geri/ileri (popstate) YENİ KAYIT AÇMAZ ve sayfayı geri yükler", async () => {
  const h = await boot({ overrides: ov() });
  const L0 = h.win.history.length;
  await h.click('[data-nav="servers"]'); assert.equal(h.win.history.length, L0 + 1); assert.equal(h.win.location.hash, "#v=servers");
  await h.click('[data-nav="servers"]'); assert.equal(h.win.history.length, L0 + 1, "aynı sayfayı yeniden seçmek kayıt eklemez");
  await h.click('[data-tsrv="db1"]').catch(() => {});                  // sunucu sayfasında değiliz: ağaç yok
  await h.click('#servers-all [data-name="db1"]'); assert.equal(view(h), "view-server"); assert.equal(h.win.history.length, L0 + 2);
  assert.equal(h.win.location.hash, "#s=db1&p=ozet");
  h.win.history.back(); await tick(30); await h.settle();
  assert.equal(view(h), "view-servers", "geri: Sunucular sayfası"); assert.equal(h.win.history.length, L0 + 2, "geri yeni kayıt açmaz");
  h.win.history.back(); await tick(30); await h.settle();
  assert.equal(view(h), "view-dash"); assert.equal(h.win.location.hash, ""); assert.equal(h.win.history.length, L0 + 2);
  h.win.history.forward(); await tick(30); await h.settle(); assert.equal(view(h), "view-servers"); assert.equal(h.win.history.length, L0 + 2);
  // geri/ileri sonrası YENİ bir gezinme yine geçmişe eklenir (routing bayrağı işi bitince kapanmalı): geri gidince Sunucular'a dönülür
  await h.click('[data-nav="lynis"]'); assert.equal(h.win.location.hash, "#v=lynis");
  h.win.history.back(); await tick(30); await h.settle(); assert.equal(view(h), "view-servers", "yeni gezinme geçmişe eklendi (üzerine yazılmadı)");
  assert.deepEqual(h.errors, []); h.close();
});

test("gezinme: adres çubuğundaki #s=sunucu&p=sekme açılışta uygulanır (geçmişe fazladan kayıt eklemeden)", async () => {
  const h = await boot({ hash: "#s=web1&p=utility", overrides: ov() });
  assert.equal(view(h), "view-server"); assert.equal(h.$('#srv-tabs [aria-selected="true"]').dataset.p, "utility"); assert.equal(h.win.history.length, 1);
  const h2 = await boot({ hash: "#v=lynis", overrides: ov() }); assert.equal(view(h2), "view-lynis"); assert.equal(h2.win.history.length, 1);
  const h3 = await boot({ hash: "#v=geçersiz", overrides: ov() }); assert.equal(view(h3), "view-dash", "bilinmeyen sayfa Pano'ya düşer");
  assert.deepEqual(h.errors.concat(h2.errors, h3.errors), []); h.close(); h2.close(); h3.close();
});

test("eski yanıt yenisini ezmez: yavaş gelen sunucu yanıtı, kullanıcı başka sunucuya geçtiyse çizilmez", async () => {
  let release;
  const slow = new Promise(r => { release = r; });
  const h = await boot({ hash: "#s=db1", overrides: ov({ "/api/server?name=web1": async () => { await slow; return fixture("server-web1"); } }) });
  assert.equal(h.$("#srv-head h1").textContent, "db1");
  await h.click('[data-tsrv="web1"]');                                  // web1 yanıtı bekletilir
  assert.equal(h.$("#srv-body").textContent.trim(), "yükleniyor…", "yanıt gelene kadar eski sunucunun içeriği silinir");
  assert.equal(h.$("#srv-tabs").children.length, 0, "eski sunucunun sekmeleri temizlenir");
  await h.click('[data-tsrv="vm1"]');                                   // kullanıcı başka sunucuya geçti
  assert.equal(h.$("#srv-head h1").textContent, "vm1"); await h.settle();
  const vm1Html = h.$("#srv-body").innerHTML;
  release(); await h.settle();                                          // web1'in YAVAŞ yanıtı şimdi gelir
  assert.equal(h.$("#srv-head h1").textContent, "vm1"); assert.equal(h.$("#srv-body").innerHTML, vm1Html, "gecikmiş web1 yanıtı vm1 sayfasını ezmemeli");
  assert.equal(h.win.location.hash, "#s=vm1&p=ozet");
  assert.deepEqual(h.errors, []); h.close();
});

test("sunucu geçişi: yükleme sürerken pencere yeniden boyutlanırsa ESKİ sunucunun verisi yeni başlığın altında çizilmez", async () => {
  let release; const slow = new Promise(r => { release = r; });
  const h = await boot({ hash: "#s=db1", overrides: ov({ "/api/server?name=web1": async () => { await slow; return fixture("server-web1"); } }) });
  await h.click('[data-tsrv="web1"]');
  h.win.dispatchEvent(new h.win.Event("resize")); h.timers.run(300); await h.settle();   // yeniden boyutlandırma (250 ms) sunucu sayfasını yeniden çizer
  assert.equal(h.$("#srv-head h1").textContent, "web1"); assert.equal(h.$("#srv-body").textContent.trim(), "yükleniyor…", "yeni sunucunun verisi gelene kadar eski veri çizilmemeli");
  assert.equal(h.$("#srv-tabs").children.length, 0);
  release(); await h.settle(); assert.ok(h.$("#srv-tabs").children.length > 0, "yanıt gelince normal çizilir");
  assert.deepEqual(h.errors, []); h.close();
});

test("geçersiz adres Pano'ya düşer ve geçmişe KAYIT EKLEMEZ (açılışta ve geri/ileri sırasında)", async () => {
  const h = await boot({ hash: "#v=zzz", overrides: ov() });
  assert.equal(view(h), "view-dash"); assert.equal(h.win.history.length, 1, "açılışta düzeltme mevcut kaydı değiştirir"); assert.equal(h.win.location.hash, "");
  await h.click('[data-nav="servers"]'); const L = h.win.history.length;
  h.win.history.replaceState(null, "", "#v=baska-bir-sey"); h.win.dispatchEvent(new h.win.PopStateEvent("popstate"));   // geçmişten bozuk bir adres
  await h.settle(); assert.equal(view(h), "view-dash"); assert.equal(h.win.history.length, L, "geçmişten uygulanan gezinme yeni kayıt açmaz"); assert.equal(h.win.location.hash, "");
  assert.deepEqual(h.errors, []); h.close();
});

test("eski yanıt: kullanıcı sayfadan ayrıldıysa gelen sunucu yanıtı çizilmez", async () => {
  let release; const slow = new Promise(r => { release = r; });
  const h = await boot({ hash: "#v=servers", overrides: ov({ "/api/server?name=web1": async () => { await slow; return fixture("server-web1"); } }) });
  await h.click('#servers-all [data-name="web1"]'); assert.equal(view(h), "view-server");
  await h.click("#srv-back"); assert.equal(view(h), "view-servers");
  const before = h.$("#srv-body").innerHTML; release(); await h.settle();
  assert.equal(view(h), "view-servers"); assert.equal(h.$("#srv-body").innerHTML, before, "sunucu sayfası arka planda çizilmemeli");
  assert.deepEqual(h.errors, []); h.close();
});

test("Lynis: Pano kartında en yeni ölçümlü sunucu seçilir; Lynis sayfasında seçici sunucuyu değiştirir", async () => {
  const h = await boot({ overrides: ov() });
  assert.match(h.$("#a-lynis").textContent, /web1/); assert.match(h.$("#a-lynis").textContent, /74/);
  await h.click('[data-nav="lynis"]'); assert.equal(view(h), "view-lynis");
  assert.equal(h.$('#lynis-pick [aria-selected="true"]').dataset.n, "web1", "en yeni ölçümlü sunucu seçili gelir");
  const picks = h.$$("#lynis-pick [data-n]"); assert.equal(picks.length, 3);
  await h.click(picks.find(b => b.dataset.n === "vm1"));
  assert.equal(h.$('#lynis-pick [aria-selected="true"]').dataset.n, "vm1"); assert.match(h.$("#lynis-detail").textContent, /ölçüm yok/);
  await h.click(picks.find(b => b.dataset.n === "web1")); assert.match(h.$("#lynis-detail").textContent, /74\/100/); assert.equal(h.$("#lyn-count").textContent, "0 / 0 bulgu");
  assert.deepEqual(h.errors, []); h.close();
});

// ---- Düzeltilen eski hatalar (refaktör 3b'de test ile bulunmuş, main'de doğrulanmıştı) ----
test("Lynis: eski sunucu/ikili bulgu listesini null gönderirse (findings: null) sayfa çökmez, '0 / 0 bulgu' gösterir", async () => {
  const nullFindings = u => ({ ...fixture("lynis-" + u.searchParams.get("server")), findings: null });
  const h = await boot({ overrides: ov({ "/api/lynis": nullFindings }) });
  await h.click('[data-nav="lynis"]');
  assert.deepEqual(h.errors, []); assert.equal(h.$("#lyn-count").textContent, "0 / 0 bulgu"); assert.match(h.$("#lynis-detail").textContent, /74\/100/);
  h.close();
});

test("Lynis: #v=lynis adresiyle açılışta (sayfa yenileme) sunucu seçici dolu gelir, 'Sunucu yok' yazmaz", async () => {
  const h = await boot({ hash: "#v=lynis", overrides: ov() });
  assert.equal(h.$$("#lynis-pick [data-n]").length, 3); assert.doesNotMatch(h.$("#lynis-detail").textContent, /Sunucu yok/);
  assert.equal(h.$('#lynis-pick [aria-selected="true"]').dataset.n, "web1"); assert.match(h.$("#lynis-detail").textContent, /74\/100/);
  assert.deepEqual(h.errors, []); h.close();
});

test("Lynis: gerçekten sunucu yoksa 'Sunucu yok' gösterilir (açılışta yeniden çizim bunu bozmaz)", async () => {
  const h = await boot({ hash: "#v=lynis", overrides: ov({ "/api/home": () => ({ ...fixture("home"), servers: [] }) }) });
  assert.match(h.$("#lynis-detail").textContent, /Sunucu yok/); assert.equal(h.$$("#lynis-pick [data-n]").length, 0);
  assert.deepEqual(h.errors, []); h.close();
});

test("açılışta #s=sunucu ve #v=lynis birlikte değil: sunucu adresi Lynis yeniden çizimini tetiklemez", async () => {
  const h = await boot({ hash: "#s=web1", overrides: ov() });
  assert.equal(view(h), "view-server"); assert.equal(h.calls.filter(c => c.path.startsWith("/api/lynis")).length, 0, "sunucu sayfası açılırken Lynis sayfası yüklenmez");
  assert.deepEqual(h.errors, []); h.close();
});

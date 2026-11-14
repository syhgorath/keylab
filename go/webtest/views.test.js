"use strict";
/* Sayfa ve pencere anlık görüntüleri: refaktör sırasında DOM çıktısı DEĞİŞMEMELİ. Bilerek değişirse:
   UPDATE_SNAPSHOTS=1 npm test   ve değişen golden dosyaları gözle inceleyin. */
const test = require("node:test");
const assert = require("node:assert/strict");
const { boot, snapshot, expectSnapshot, fixture } = require("./harness");

// bazı sunucular çevrimiçi/ölçülmüş olsun: Pano, süzgeç ve KPI tonları dolu görünsün
const mixedHome = () => {
  const h = fixture("home");
  h.servers.forEach((s, i) => { s.checked = true; s.up = i !== 1; s.conn = s.up ? "up" : "down"; });
  return h;
};
const mixedStatus = () => ({ db1: { up: true, conn: "up" }, vm1: { up: false, conn: "down" }, web1: { up: true, conn: "up" } });
const mixed = { "/api/home": mixedHome, "/api/status": mixedStatus };

async function page(t, name, { hash = "", overrides = {}, storage, lang, root = "main", after } = {}) {
  const h = await boot({ hash, overrides: { ...mixed, ...overrides }, storage, lang });
  if (after) await after(h);
  assert.deepEqual(h.errors, [], "konsol/betik hatası olmamalı");
  expectSnapshot(assert, name, snapshot(h.$(root)));
  h.close();
}

test("Pano (karışık durum)", t => page(t, "dash"));
test("Pano (boş: karşılama kartı)", t => page(t, "dash-empty", { overrides: { "/api/home": () => ({ ...fixture("home"), servers: [] }) } }));
test("Sunucular: süzgeç 'kapalı'", t => page(t, "servers-filter-down", { hash: "#v=servers", after: h => h.click('#view-servers [data-ff="down"]') }));
test("Sunucular (simgeler)", t => page(t, "servers-tiles", { hash: "#v=servers" }));
test("Sunucular (liste)", t => page(t, "servers-list", { hash: "#v=servers", storage: { "sshpocket.mode": "list" } }));
test("Paketler", t => page(t, "packages", { hash: "#v=packages" }));
// db1'de bir değişken farklı olsun: sapma (drift) satırı görünür
const driftVars = u => {
  const d = fixture(`vars-${u.searchParams.get("server")}-${u.searchParams.get("package")}`);
  if (u.searchParams.get("server") === "db1" && u.searchParams.get("package") === "utility") { d.vars[0].value = "Europe/Istanbul"; d.vars[0].set = true; }
  return d;
};
test("Karşılaştır", t => page(t, "compare", { hash: "#v=compare", overrides: { "/api/vars": driftVars } }));
test("Raporlar", t => page(t, "reports", { hash: "#v=reports" }));
test("Lynis", t => page(t, "lynis", { hash: "#v=lynis" }));
test("Günlükler", t => page(t, "logs", { hash: "#v=logs" }));
test("Ayarlar", t => page(t, "settings", { hash: "#v=settings" }));
test("Sunucu ekle", t => page(t, "add", { hash: "#v=add" }));
test("Sunucu sayfası: özet", t => page(t, "server-web1", { hash: "#s=web1" }));
test("Sunucu sayfası: linux-hardening", t => page(t, "server-web1-hardening", { hash: "#s=web1&p=linux-hardening" }));
test("Sunucu sayfası: utility", t => page(t, "server-web1-utility", { hash: "#s=web1&p=utility" }));
test("Sunucu sayfası: kurulum", t => page(t, "server-web1-kurulum", { hash: "#s=web1&p=kurulum" }));
test("Sunucu sayfası: çevrimdışı sunucu", t => page(t, "server-vm1", { hash: "#s=vm1" }));

test("Alt çubuk açık", t => page(t, "dock-open", { root: "#wb-dock", after: h => h.click("#dock-tab") }));
test("Toplu işlem penceresi (seçim)", t => page(t, "batch-choose", { root: "#batch-modal", overrides: { "/api/batch/options": { options: [{ package: "utility", action: "durum", ok: 2, total: 3 }], total: 3 } }, after: h => h.click("#batch-open") }));
test("Kısayollar penceresi", t => page(t, "shortcuts", { root: "#shortcuts-modal", after: async h => { h.doc.dispatchEvent(new h.win.KeyboardEvent("keydown", { key: "?", bubbles: true })); await h.settle(); } }));
test("Sihirbaz/ekleme ve içe aktarma penceresi", t => page(t, "import-modal", { hash: "#v=add", root: "#import-modal", overrides: { "/api/ssh-config-hosts": { hosts: [{ alias: "web1", host: "10.0.0.1", user: "root", port: "22", exists: false }] } }, after: h => h.click("#imp-open") }));

test("İngilizce: Pano ve Ayarlar", async () => {
  for (const [name, hash] of [["en-dash", ""], ["en-settings", "#v=settings"]]) {
    const h = await boot({ hash, overrides: mixed, lang: "en" });
    assert.deepEqual(h.errors, []);
    h.timers.run(1000); await h.settle();
    expectSnapshot(assert, name, snapshot(h.$("main")));
    h.close();
  }
});

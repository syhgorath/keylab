"use strict";
/* Paketler sayfası ek testleri (refaktör 5b): mutasyon turunda kaçanlar — eksik sürüm parçası, keşfet satırında ad=kimlik,
   iş sürerken yükseltme açılmaz, kapalı pencerenin yeniden çizilmemesi. */
const test = require("node:test");
const assert = require("node:assert/strict");
const { boot, fixture } = require("./harness");
const { fakeJobs } = require("./lib/fakejobs");

const mixedHome = () => { const h = fixture("home"); h.servers.forEach((s, i) => { s.checked = true; s.up = i !== 1; s.conn = s.up ? "up" : "down"; }); return h; };
const withPk = f => ({ "/api/home": mixedHome, "/api/packages": () => { const d = fixture("packages"); f(d); return d; } });
const pkgOf = (d, id) => d.packages.find(p => p.id === id);
const REPORT = { ok: true, serial: "7", valid: "2027-01-01", exit: 0, raw: "HAM", rows: [
  { id: "utility", installed: "0.1.0", verdict: "upgrade", version: "0.1.1", text: "" },
] };
const dlgOpen = h => h.$("#dlg").classList.contains("open");

test("paketler: eksik sürüm parçası 0 sayılır (1.0 < 1.0.1 → eski; 1.0 = 1.0.0 → eski değil)", async () => {
  const h = await boot({ hash: "#v=packages", overrides: withPk(d => {
    const a = pkgOf(d, "linux-hardening"); a.current = "1.0.1"; a.uses[0].version = "1.0";
    const b = pkgOf(d, "utility"); b.current = "1.0.0"; b.uses[0].version = "1.0";
  }) });
  assert.match(h.$('[data-pk="linux-hardening"]').textContent, /1 sunucuda eski sürüm/);
  assert.doesNotMatch(h.$('[data-pk="utility"]').textContent, /eski sürüm/); h.close();
});

test("keşfet: paket adı kimlikle aynıysa kimlik ikinci kez yazılmaz", async () => {
  const h = await boot({ hash: "#v=packages", overrides: { ...withPk(d => { pkgOf(d, "utility").name = "utility"; }), "/api/updates": REPORT } });
  await h.click("#disc-open");
  const nm = h.$("#disc-list .pkg-name"); assert.equal(nm.textContent.trim(), "utility"); assert.equal(nm.querySelector(".mono"), null); h.close();
  const g = await boot({ hash: "#v=packages", overrides: { "/api/home": mixedHome, "/api/updates": REPORT } });
  await g.click("#disc-open"); assert.ok(g.$("#disc-list .pkg-name .mono"), "ad farklıysa kimlik küçük yazıyla görünür"); g.close();
});

test("yükseltme: bir iş sürerken Yükselt yeni pencere açmaz", async () => {
  const f = fakeJobs();
  const h = await boot({ hash: "#s=web1&p=utility", overrides: { ...f.overrides, "/api/home": mixedHome, "/api/updates": REPORT } });
  const run = h.$$("#srv-body button[data-run]").find(b => b.textContent.trim() === "Çalıştır");
  await h.click(run); await h.click("#run-go");
  const title = h.$("#run-title").textContent; assert.match(title, /durum/);
  await h.click('[data-nav="packages"]'); await h.click("#upd-check"); await h.click('#dlg-body [data-upg="utility"]');
  assert.equal(h.$("#run-title").textContent, title, "çalışan işin penceresi bozulmadı"); assert.equal(h.$("#u-ack"), null);
  assert.deepEqual(h.errors, []); h.close();
});

test("kapalı iletişim penceresi indeks yenilenince yeniden çizilmez", async () => {
  const h = await boot({ hash: "#v=packages", overrides: { "/api/home": mixedHome, "/api/updates": REPORT } });
  await h.click("#upd-check"); await h.click("#dlg-cancel"); assert.ok(!dlgOpen(h));
  h.$("#dlg-body").innerHTML = "<i id='isaret'>x</i>";
  await h.click('[data-nav="dash"]'); await h.click('[data-nav="packages"]'); // sayfa yüklenince indeks kartı yeniden çizilir
  assert.ok(h.$("#isaret"), "kapalı pencerenin gövdesi değişmez");
  assert.deepEqual(h.errors, []); h.close();
});

test("paket ayrıntısı: 4–8 sunucuda arama kutusu yok (eşik 8'den çok)", async () => {
  const h = await boot({ hash: "#v=packages", overrides: withPk(d => { for (let i = 0; i < 3; i++) d.servers.push({ name: "y" + i, os: "debian" }); }) });
  await h.click('[data-pk="utility"]'); assert.equal(h.$("#pd-q"), null); assert.equal(h.$("#pd-stale"), null, "eski sürümlü yok, az sunucu → süzgeç satırı yok"); h.close();
});

test("güncellemeler: denetim sürerken pencere düğmesi etkisiz (ikinci istek gitmez)", async () => {
  let release; const gate = new Promise(r => { release = r; });
  const h = await boot({ hash: "#v=packages", overrides: { "/api/home": mixedHome, "/api/updates": async () => { await gate; return REPORT; } } });
  h.$("#upd-check").dispatchEvent(new h.win.MouseEvent("click", { bubbles: true })); await new Promise(r => setImmediate(r)); await new Promise(r => setImmediate(r));
  h.$("#upd-recheck").dispatchEvent(new h.win.MouseEvent("click", { bubbles: true })); await new Promise(r => setImmediate(r));
  assert.equal(h.calls.filter(c => c.path === "/api/updates").length, 1);
  release(); await h.settle(); h.close();
});

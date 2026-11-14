"use strict";
/* Pano = genel özet: kompakt sunucu durum çipleri (sorunlu sunucular önce, en çok 12, kalanı Sunucular'da); süzgeç, görünüm düğmesi ve
   "Sunucu ekle" karosu Pano'da yok (Sunucular sayfasında). */
const test = require("node:test");
const assert = require("node:assert/strict");
const { boot, fixture } = require("./harness");

const base = (f, st) => {
  const home = () => { const h = fixture("home"); h.servers.forEach((s, i) => { s.checked = true; s.up = i !== 1; s.conn = s.up ? "up" : "down"; }); f && f(h); return h; };
  return { "/api/home": home, "/api/status": () => Object.fromEntries(home().servers.filter(s => s.checked).map(s => [s.name, { up: s.up, conn: s.conn }])) };
};
const names = h => h.$$("#servers .schip[data-name]").map(e => e.dataset.name);
const many = n => h => { for (let i = 0; i < n; i++) h.servers.push({ ...h.servers[0], name: "x" + String(i).padStart(2, "0"), up: true, conn: "up", checked: true, last: null }); };

test("pano: sunucular kompakt çip olarak (ad, son işlem, durum noktası); sorunlu sunucular önce", async () => {
  const h = await boot({ overrides: base() });
  assert.deepEqual(names(h), ["vm1", "db1", "web1"], "çevrimdışı vm1 önce, çevrimiçiler sonra (kendi sırasıyla)");
  const web = h.$('#servers [data-name="web1"]'); assert.equal(web.querySelector(".schip-name").textContent, "web1"); assert.match(web.querySelector(".schip-sub").textContent, /^✔ audit · /); assert.ok(web.querySelector("svg.osico[width='28']")); assert.ok(web.querySelector(".status-dot.on"));
  assert.equal(h.$('#servers [data-name="vm1"] .schip-sub').textContent, "kayıt yok"); assert.ok(h.$('#servers [data-name="vm1"].is-off')); assert.equal(h.$('#servers [data-name="web1"]').getAttribute("aria-label"), "web1, çevrimiçi");
  assert.deepEqual(h.errors, []); h.close();
});

test("pano: süzgeç çipleri, görünüm düğmesi ve 'Sunucu ekle' karosu yok; Tümünü gör Sunucular'a götürür", async () => {
  const h = await boot({ overrides: base() });
  assert.equal(h.$("#view-dash .fleet-filter"), null); assert.equal(h.$("#view-dash .mode-toggle"), null); assert.equal(h.$("#servers .stile.add"), null); assert.equal(h.$("#view-dash [data-nav-target='add']"), null);
  assert.equal(h.$("#view-dash .card-head h2").textContent, "Sunucu durumu");
  await h.click('#view-dash [data-nav-target="servers"]'); assert.equal(h.$(".view:not([hidden])").id, "view-servers"); h.close();
});

test("pano: çipe tıklamak ve Enter sunucuyu açar; sağ tık ve F10 bağlam menüsünü açar", async () => {
  const h = await boot({ overrides: base() });
  await h.click('#servers [data-name="db1"]'); assert.equal(h.$(".view:not([hidden])").id, "view-server"); assert.equal(h.$("#srv-head h1").textContent, "db1");
  await h.click("#srv-back"); await h.click('[data-nav="dash"]');
  h.$('#servers [data-name="web1"]').dispatchEvent(new h.win.KeyboardEvent("keydown", { key: "Enter", bubbles: true })); await h.settle(); assert.equal(h.$("#srv-head h1").textContent, "web1");
  await h.click('[data-nav="dash"]'); h.$('#servers [data-name="vm1"]').dispatchEvent(new h.win.MouseEvent("contextmenu", { bubbles: true, cancelable: true })); assert.equal(h.$("#ctx-head").textContent, "vm1"); h.close();
});

test("pano: en çok 12 çip; fazlası '+N sunucu daha' çipiyle Sunucular'a bağlanır (sorunlular önce, ilk 12'ye girer)", async () => {
  const h = await boot({ overrides: base(many(12)) }); // 3 + 12 = 15 sunucu
  assert.equal(names(h).length, 12); assert.equal(names(h)[0], "vm1", "çevrimdışı sunucu kesilmez"); const more = h.$("#servers .schip.more"); assert.ok(more); assert.match(more.textContent, /\+ 3 sunucu daha/);
  await h.click(more); assert.equal(h.$(".view:not([hidden])").id, "view-servers"); assert.equal(h.$$("#servers-all .stile-rich").length, 15, "Sunucular sayfası hepsini gösterir"); h.close();
  const g = await boot({ overrides: base(many(9)) }); assert.equal(names(g).length, 12, "tam 12 ise 'daha' çipi yok"); assert.equal(g.$("#servers .schip.more"), null); g.close();
});

test("pano: geçersiz sunucu çipi uyarı metni taşır; başarısız son işlem kırmızı", async () => {
  const h = await boot({ overrides: base(x => { x.servers.find(s => s.name === "db1").valid = false; x.servers.find(s => s.name === "web1").last = { ...x.servers.find(s => s.name === "web1").last, result: "fail" }; }) });
  const db = h.$('#servers [data-name="db1"] .schip-sub'); assert.equal(db.textContent, "⚠ server.env geçersiz"); assert.ok(db.classList.contains("bad"));
  const web = h.$('#servers [data-name="web1"] .schip-sub'); assert.match(web.textContent, /^✖ audit · /); assert.ok(web.classList.contains("bad")); h.close();
});

test("pano: arama çipleri süzer; Sunucular sayfasının süzgeci/kipi Pano'yu etkilemez", async () => {
  const h = await boot({ overrides: base() });
  await h.type("#search-input", "web"); assert.deepEqual(names(h), ["web1"]); await h.type("#search-input", "yokboyle"); assert.deepEqual(names(h), []); assert.match(h.$("#servers").textContent, /Sunucu bulunamadı/);
  await h.type("#search-input", ""); await h.click('[data-nav="servers"]'); await h.click('#view-servers [data-ff="down"]'); assert.deepEqual(h.$$("#servers-all .stile-rich").map(e => e.dataset.name), ["vm1"]);
  assert.deepEqual(names(h), ["vm1", "db1", "web1"], "Sunucular süzgeci Pano çiplerini daraltmaz"); h.close();
});

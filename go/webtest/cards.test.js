"use strict";
/* Sunucular sayfasındaki zengin sunucu kartı testleri: içerik (durum rozeti, adres, anahtar, son işlem), hızlı düğmeler
   (Terminal, Kopya, Yönet), liste kipi, Pano'nun küçük karosunun değişmemesi. */
const test = require("node:test");
const assert = require("node:assert/strict");
const { boot, fixture } = require("./harness");

const mixedHome = () => { const h = fixture("home"); h.servers.forEach((s, i) => { s.checked = true; s.up = i !== 1; s.conn = s.up ? "up" : "down"; }); return h; };
const home = f => () => { const h = mixedHome(); f && f(h); return h; };
// /api/home + eşleşen /api/status (açılıştan sonra durum ölçümü home'daki up/conn bayraklarını ezmesin)
const ov = f => ({ "/api/home": home(f), "/api/status": () => Object.fromEntries(home(f)().servers.filter(s => s.checked).map(s => [s.name, { up: s.up, conn: s.conn }])) });
const card = (h, n) => h.$(`#servers-all .stile-rich[data-name="${n}"]`);
const view = h => h.$(".view:not([hidden])").id;
const t = el => el.textContent.replace(/\s+/g, " ").trim();

test("sunucu kartı: ad, OS · platform, durum rozeti, adres + anahtar, son işlem, hızlı düğmeler", async () => {
  const h = await boot({ hash: "#v=servers", overrides: ov(x => { const s = x.servers.find(s => s.name === "web1"); s.hasKey = true; s.os = "ubuntu"; s.platform = "26.04"; }) });
  const c = card(h, "web1");
  assert.equal(t(c.querySelector(".stile-name")), "web1"); assert.equal(t(c.querySelector(".stile-os")), "Ubuntu · 26.04"); assert.ok(c.querySelector("svg.osico[width='40']"));
  assert.equal(t(c.querySelector(".stile-pill")), "çevrimiçi"); assert.ok(c.querySelector(".stile-pill").classList.contains("ok"));
  assert.equal(t(c.querySelector(".stile-addr .mono")), "root@10.255.0.10:22"); assert.equal(t(c.querySelector(".stile-addr .badge")), "Anahtar var"); assert.ok(c.querySelector(".stile-addr .badge.ok"));
  assert.match(t(c.querySelector(".stile-last")), /^✔ linux-hardening · audit · /);
  assert.deepEqual([...c.querySelectorAll(".stile-foot [data-qa]")].map(b => b.dataset.qa), ["term", "copy", "open"]); assert.equal(t(c.querySelector('[data-qa="open"]')), "Yönet →");
  assert.deepEqual(h.errors, []); h.close();
});

test("sunucu kartı: durum rozeti tonları ve anahtar yok uyarısı; kayıt yoksa 'kayıt yok'", async () => {
  const h = await boot({ hash: "#v=servers", overrides: ov(x => { x.servers.find(s => s.name === "db1").knock = true; x.servers.find(s => s.name === "db1").up = false; x.servers.find(s => s.name === "db1").conn = "down"; x.servers.find(s => s.name === "web1").checked = false; }) });
  assert.ok(card(h, "db1").querySelector(".stile-pill.warn")); assert.match(t(card(h, "db1").querySelector(".stile-pill")), /knock gerekir/);
  assert.ok(card(h, "vm1").querySelector(".stile-pill.danger")); assert.ok(card(h, "web1").querySelector(".stile-pill.plain")); assert.match(t(card(h, "web1").querySelector(".stile-pill")), /denetleniyor/);
  assert.equal(t(card(h, "vm1").querySelector(".stile-addr .badge")), "Anahtar yok"); assert.ok(card(h, "vm1").querySelector(".stile-addr .badge.warn")); assert.equal(t(card(h, "vm1").querySelector(".stile-last")), "kayıt yok"); h.close();
});

test("sunucu kartı: geçersiz sunucuda adres 'server.env geçersiz', Terminal/Kopya kapalı, Yönet açık", async () => {
  const h = await boot({ hash: "#v=servers", overrides: ov(x => { x.servers.find(s => s.name === "db1").valid = false; }) });
  const c = card(h, "db1"); assert.equal(t(c.querySelector(".stile-addr .mono")), "server.env geçersiz"); assert.match(t(c.querySelector(".stile-last")), /server\.env geçersiz/); assert.ok(c.querySelector(".stile-last.bad"));
  assert.equal(c.querySelector('[data-qa="term"]').disabled, true); assert.equal(c.querySelector('[data-qa="copy"]').disabled, true); assert.equal(c.querySelector('[data-qa="open"]').disabled, false); h.close();
});

test("sunucu kartı: Terminal → /api/action terminal; Kopya → komut panoya; ikisi de sunucu sayfasını AÇMAZ", async () => {
  const h = await boot({ hash: "#v=servers", overrides: { "/api/home": mixedHome, "/api/action": (u, { body }) => (body.action === "cmd" ? { cmd: "ssh root@10.255.0.10" } : { ok: true }) } });
  const L = h.win.history.length;
  await h.click(card(h, "web1").querySelector('[data-qa="term"]')); assert.deepEqual(h.calls.find(c => c.path === "/api/action").body, { name: "web1", action: "terminal" }); assert.equal(t(h.$("#toast")), "Terminal'de açıldı");
  await h.click(card(h, "web1").querySelector('[data-qa="copy"]')); assert.deepEqual(h.win.__clip, ["ssh root@10.255.0.10"]); assert.equal(t(h.$("#toast")), "Komut kopyalandı");
  assert.equal(view(h), "view-servers"); assert.equal(h.win.history.length, L); assert.deepEqual(h.errors, []); h.close();
});

test("sunucu kartı: Yönet → sunucu sayfası (tek geçmiş kaydı); kartın kendisine tıklamak ve Enter da açar, iç düğmede Enter açmaz", async () => {
  const h = await boot({ hash: "#v=servers", overrides: { "/api/home": mixedHome, "/api/action": { ok: true } } });
  const L = h.win.history.length;
  await h.click(card(h, "web1").querySelector('[data-qa="open"]')); assert.equal(view(h), "view-server"); assert.equal(h.$("#srv-head h1").textContent, "web1"); assert.equal(h.win.history.length, L + 1);
  await h.click("#srv-back"); const c = card(h, "db1");
  c.querySelector('[data-qa="term"]').dispatchEvent(new h.win.KeyboardEvent("keydown", { key: "Enter", bubbles: true })); await h.settle(); assert.equal(view(h), "view-servers", "iç düğmede Enter sunucuyu açmaz");
  c.dispatchEvent(new h.win.KeyboardEvent("keydown", { key: "Enter", bubbles: true })); await h.settle(); assert.equal(view(h), "view-server"); assert.equal(h.$("#srv-head h1").textContent, "db1");
  await h.click("#srv-back"); await h.click(card(h, "vm1")); assert.equal(h.$("#srv-head h1").textContent, "vm1"); h.close();
});

test("sunucu kartı: menü düğmesi ve sağ tık bağlam menüsünü açar", async () => {
  const h = await boot({ hash: "#v=servers", overrides: { "/api/home": mixedHome } });
  await h.click(card(h, "web1").querySelector('[data-act="menu"]')); assert.ok(h.$("#ctx").classList.contains("open")); assert.equal(h.$("#ctx-head").textContent, "web1"); assert.equal(view(h), "view-servers");
  card(h, "db1").dispatchEvent(new h.win.MouseEvent("contextmenu", { bubbles: true, cancelable: true })); assert.equal(h.$("#ctx-head").textContent, "db1"); h.close();
});

test("sunucular sayfası: Liste kipinde satırlar (kart yok); Pano'nun küçük karosu zengin değil", async () => {
  const h = await boot({ hash: "#v=servers", overrides: { "/api/home": mixedHome }, storage: { "sshpocket.mode": "list" } });
  assert.equal(h.$$("#servers-all .stile-rich").length, 0); assert.equal(h.$$("#servers-all li.server").length, 3); h.close();
  const g = await boot({ hash: "", overrides: { "/api/home": mixedHome } });
  assert.equal(g.$$("#servers .stile-rich").length, 0, "Pano küçük karoları korur"); assert.equal(g.$$("#servers .schip[data-name]").length, 3); assert.equal(g.$("#servers .stile-addr"), null); assert.equal(g.$$("#servers-all .stile-rich").length, 3); g.close();
});

test("sunucular sayfası: arama ve süzgeç kartları süzer; ekle kartı yalnız süzgeçsizken", async () => {
  const h = await boot({ hash: "#v=servers", overrides: ov() });
  assert.ok(h.$("#servers-all .stile.add")); await h.type("#search-input", "web");
  assert.deepEqual(h.$$("#servers-all .stile-rich").map(e => e.dataset.name), ["web1"]); assert.equal(h.$("#servers-all .stile.add"), null);
  await h.type("#search-input", ""); await h.click('#view-servers [data-ff="down"]'); assert.deepEqual(h.$$("#servers-all .stile-rich").map(e => e.dataset.name), ["vm1"]); h.close();
});

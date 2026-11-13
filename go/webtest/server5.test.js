"use strict";
/* Sunucu sayfası ek testleri 2 (refaktör 5d): süzgeç düşmesi, yalnız port değişikliği, kayıt sonrası başlık, bağlam menüsü konumu, sunucu ağacı. */
const test = require("node:test");
const assert = require("node:assert/strict");
const { boot, fixture } = require("./harness");

const mixedHome = () => { const h = fixture("home"); h.servers.forEach((s, i) => { s.checked = true; s.up = i !== 1; s.conn = s.up ? "up" : "down"; }); return h; };
const srv = f => ({ "/api/home": mixedHome, "/api/vars": () => ({ vars: fixture("vars-web1-kurulum").vars }), "/api/server": u => { const d = fixture("server-" + u.searchParams.get("name")); d.server.up = d.server.name !== "vm1"; d.server.conn = d.server.up ? "up" : "down"; f && f(d); return d; } });
const pkg = (d, id) => d.packages.find(p => p.id === id);
const txt = (h, s) => h.$(s).textContent.replace(/\s+/g, " ").trim();

test("katalog: durumu bilinmeyen sunucuda eski süzgeç 'Tümü'ne düşer (kartlar gizli kalmaz)", async () => {
  const k = d => { pkg(d, "kurulum").actions.find(a => a.id === "durum").keys = d.server.name === "web1" ? { kurulu: "docker" } : {}; };
  const h = await boot({ hash: "#s=web1&p=kurulum", overrides: srv(k) });
  await h.click('[data-sf="on"]'); assert.equal(h.$$("#store-grid .app-card").filter(c => !c.hidden).length, 1);
  await h.click('[data-tsrv="db1"]'); await h.click('#srv-tabs [data-p="kurulum"]');
  assert.equal(h.$$("#store-grid .app-card").filter(c => !c.hidden).length, 4, "durum bilinmiyor: tüm kartlar görünür"); h.close();
});

test("adres düzenleme: yalnız port değişse de kaydedilir; kayıttan sonra başlık yeni adresle yenilenir", async () => {
  let host = "10.255.0.10", port = "22"; const edits = [];
  const h = await boot({ hash: "#s=web1", overrides: { ...srv(), "/api/server": () => { const d = fixture("server-web1"); d.server.up = true; d.server.host = host; d.server.port = port; return d; }, "/api/server-edit": (u, { body }) => { edits.push(body); host = body.host; port = body.port; return { ok: true }; } } });
  await h.click("#srv-edit"); h.$("#ea-port").value = "2222"; await h.click("#dlg-ok");
  assert.deepEqual(edits, [{ name: "web1", host: "10.255.0.10", user: "root", port: "2222" }]);
  assert.equal(txt(h, "#srv-addr"), "root@10.255.0.10:2222", "başlık yeni adresi gösterir"); h.close();
});

test("sunucu sayfası: '⋯' menüsü düğmenin altında açılır; sunucu ağacında açık sunucu işaretlenir", async () => {
  const h = await boot({ hash: "#s=web1", overrides: srv() });
  assert.equal(h.$('#srvtree-list [data-tsrv="web1"]').getAttribute("aria-current"), "page"); assert.ok(h.$('#srvtree-list [data-tsrv="web1"]').classList.contains("on")); assert.equal(h.$('#srvtree-list [data-tsrv="db1"]').getAttribute("aria-current"), "false");
  h.$("#srv-more").getBoundingClientRect = () => ({ right: 500, bottom: 100, left: 0, top: 0, width: 0, height: 0 });
  await h.click("#srv-more"); const m = h.$("#ctx"); assert.ok(m && m.classList.contains("open")); assert.equal(m.style.left, "270px"); assert.equal(m.style.top, "106px"); assert.equal(h.$("#ctx-head").textContent, "web1");
  await h.click('[data-tsrv="db1"]'); assert.equal(h.$('#srvtree-list [data-tsrv="db1"]').getAttribute("aria-current"), "page"); assert.equal(h.$('#srvtree-list [data-tsrv="web1"]').getAttribute("aria-current"), "false"); h.close();
});

"use strict";
/* Birleşik utility paketi (0.2.0+): kurulum kataloğu ("uygulama mağazası") geri geldi. Katalog, `kurulum_durum` eylemi olan utility paketi için çizilir;
   kurulu bilgisi o eylemin `kurulu`/`portlar` anahtarlarından okunur; paketin kendi kartları (bilgi, değişkenler, Son durum) katalogun altında görünür. */
process.env.TZ = "UTC";
const test = require("node:test");
const assert = require("node:assert/strict");
const { boot, fixture } = require("./harness");
const { mixedHome } = require("./lib/fakejobs");

const pkg = (d, id) => d.packages.find(p => p.id === id);
const KV = fixture("vars-web1-kurulum").vars;
const merged = d => {
  const kur = pkg(d, "kurulum"), u = pkg(d, "utility"), dur = kur.actions.find(a => a.id === "durum");
  u.actions.push({ ...JSON.parse(JSON.stringify(dur)), id: "kurulum_durum", keys: { kurulu: "docker:docker+nginx:native", portlar: "nginx:80" } });
  d.packages = d.packages.filter(p => p !== kur);
};
const open = async f => { const h = await boot({ hash: "#s=web1&p=linux-hardening", overrides: {
  "/api/home": mixedHome,
  "/api/server": u => { const d = fixture("server-" + u.searchParams.get("name")); d.server.up = true; f && f(d); return d; },
  "/api/vars": () => ({ vars: KV }),
} }); await h.click('#srv-tabs [data-p="utility"]'); return h; };
const txt = (h, s) => h.$(s).textContent.replace(/\s+/g, " ").trim();

test("birleşik utility: kurulum_durum varsa uygulama kartları çizilir, kurulu bilgisi kurulum_durum'dan, paketin kendi kartları altta", async () => {
  const h = await open(merged);
  assert.equal(txt(h, ".store-head h2"), "Uygulama kataloğu");
  assert.deepEqual(h.$$("#store-grid .app-card").map(c => c.dataset.app), ["docker", "nginx", "mariadb", "redis"]);
  assert.deepEqual(h.$$("#store-grid .app-card").map(c => c.dataset.st), ["on", "on", "off", "off"]);
  assert.ok(h.$("#srv-body .pk-top"), "utility'nin kendi bilgi/değişken kartları katalogun altında");
  assert.deepEqual(h.errors, []); h.close();
});

test("birleşik olmayan utility (kurulum_durum yok): katalog çizilmez, normal paket sayfası", async () => {
  const h = await open(null);
  assert.equal(h.$("#store-grid"), null); assert.ok(h.$(".pk-info")); h.close();
});

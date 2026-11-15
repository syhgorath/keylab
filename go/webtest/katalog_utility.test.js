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

const withNtp = d => {
  merged(d); const u = pkg(d, "utility"), dur = u.actions.find(a => a.id === "kurulum_durum");
  for (const id of ["ntp_durum", "ntp_ayarla"]) u.actions.push({ ...JSON.parse(JSON.stringify(dur)), id, keys: {}, applicable: true, runnable: id === "ntp_durum", write: id === "ntp_ayarla" });
};

test("birleşik utility: sayfanın en üstünde 'Saat ve NTP' kartı, NTP durum/ayarla düğmeleri ve değişkenleri", async () => {
  const h = await open(withNtp);
  const first = h.$("#srv-body").firstElementChild; assert.ok(first.classList.contains("ntp-card"), "katalogdan önce, en üstte");
  assert.deepEqual([...first.querySelectorAll(".ntp-btns .btn")].map(b => b.textContent), ["NTP durumunu oku", "NTP ayarla"]);
  assert.ok(first.querySelector('[data-run="utility|ntp_durum"]')); assert.ok(first.querySelector('[data-write="utility|ntp_ayarla"]'));
  assert.deepEqual(h.errors, []); h.close();
});

test("'Genel eylemler' açıksa sayfa yeniden çizilince (başka sekmeye gidip dönünce) açık kalır", async () => {
  const h = await open(withNtp);
  const det = () => h.$("#srv-body .store-actions");
  assert.equal(det().open, false); det().open = true; det().dispatchEvent(new h.win.Event("toggle"));
  await h.click('#srv-tabs [data-p="linux-hardening"]'); await h.click('#srv-tabs [data-p="utility"]');
  assert.equal(det().open, true, "açık durum korunur"); h.close();
});

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

// genel okuma (kurulum_durum) ile uygulama başına okumalar (<uyg>_durum): rozet en yenisinden
const read = (id, lastTime, keys, extra) => d => { const u = pkg(d, "utility"), base = u.actions.find(a => a.id === "kurulum_durum");
  u.actions.push({ ...JSON.parse(JSON.stringify(base)), id, lastTime, lastResult: "ok", effects: [], keys, ...extra }); };
const overview = (lastTime, keys) => d => { const a = pkg(d, "utility").actions.find(x => x.id === "kurulum_durum"); Object.assign(a, { lastTime, lastResult: "ok", ...(keys ? { keys } : {}) }); };
const compose = (...fs) => d => { merged(d); fs.forEach(f => f(d)); };
const states = h => h.$$("#store-grid .app-card").map(c => `${c.dataset.app}:${c.dataset.st}`);
const card = (h, id) => h.$(`#store-grid .app-card[data-app="${id}"]`).textContent.replace(/\s+/g, " ");
const chips = h => h.$$("#srv-body [data-sf]").map(b => b.textContent.replace(/\s+/g, " ").trim());
const T1 = "2026-11-10T10:00:00Z", T2 = "2026-11-10T11:00:00Z";

test("katalog rozeti: uygulamanın daha yeni okuması genel okumayı geçer (kurulum, durmuş, kaldırma, biçim ve port)", async () => {
  const h = await open(compose(overview(T1),
    read("mariadb_durum", T2, { mariadb_kurulu: "evet", mariadb_mod: "docker", mariadb_port: "3307" }),
    read("redis_durum", T2, { redis_kurulu: "durmus", redis_mod: "native", redis_port: "ic" }),
    read("nginx_durum", T2, { nginx_kurulu: "hayir", nginx_mod: "yok", nginx_port: "yok" })));
  assert.deepEqual(states(h), ["docker:on", "nginx:off", "mariadb:on", "redis:on"]);
  assert.match(card(h, "mariadb"), /✔ Kurulu \(docker\).*Port: 3307/); assert.match(card(h, "redis"), /✔ Kurulu \(native\)/); assert.doesNotMatch(card(h, "redis"), /Port:/);
  assert.match(card(h, "nginx"), /Kurulmadı/); assert.deepEqual(chips(h).slice(1), ["Kurulu 3", "Kurulabilir 1"]);
  assert.deepEqual(h.errors, []); h.close();
});

test("katalog rozeti: genel okuma daha yeniyse o geçerli; başarısız, bilinmeyen ve değiştiren eylem sonuçları sayılmaz", async () => {
  const eski = await open(compose(overview(T2), read("mariadb_durum", T1, { mariadb_kurulu: "evet" }), read("nginx_durum", T1, { nginx_kurulu: "hayir" })));
  assert.deepEqual(states(eski), ["docker:on", "nginx:on", "mariadb:off", "redis:off"]); eski.close();
  const h = await open(compose(overview(T1),
    read("mariadb_durum", T2, { mariadb_kurulu: "evet" }, { lastResult: "fail" }),
    read("nginx_durum", T2, { nginx_kurulu: "unknown" }),
    read("redis_kur", T2, { redis_kurulu: "evet" }, { effects: ["changes-config"] }),
    read("redis_durum", T2, { redis_kurulu: "unknown" })));
  assert.deepEqual(states(h), ["docker:on", "nginx:on", "mariadb:off", "redis:off"]); h.close();
  // genel okuma uygulama için "unknown" derse eski davranış: yönetilenler listesi geçerli
  const g = await open(compose(overview(T1, { kurulu: "nginx:native", portlar: "yok", nginx_kurulu: "unknown" })));
  assert.deepEqual(states(g), ["docker:off", "nginx:on", "mariadb:off", "redis:off"]); assert.match(card(g, "nginx"), /✔ Kurulu \(native\)/); g.close();
});

test("katalog rozeti: genel okuma hiç yapılmadıysa yalnız okunan uygulamalar bilinir", async () => {
  const h = await open(compose(overview("", {}), read("redis_durum", T2, { redis_kurulu: "evet", redis_mod: "docker" })));
  assert.deepEqual(states(h), ["docker:?", "nginx:?", "mariadb:?", "redis:on"]);
  assert.match(card(h, "nginx"), /durum okunmadı/); assert.deepEqual(chips(h).slice(1), ["Kurulu 1", "Kurulabilir 0"]);
  assert.equal(h.$(".store-head + .notice"), null, "en az bir okuma varsa 'henüz okunmadı' uyarısı yok");
  assert.deepEqual(h.errors, []); h.close();
});

// yerleşik listede olmayan uygulama (ör. proxy): ayar kartı + `<kart>_kurulu` bildiren salt okunur eylem
const proxy = (keys, extra = {}) => d => {
  const u = pkg(d, "utility");
  u.settingCards = [{ id: "proxy", label: "Nginx Proxy Manager", description: "ters proxy" }, { id: "sistem", label: "Sistem" }];
  u.settings = ["proxy", "sistem"].map(card => ({ id: card + "_a", label: "Ad", card, var: "v_" + card, applyVia: [card + "_kur"], desired: [], actual: [], status: "unknown" }));
  read("proxy_durum", T2, keys, { outKeys: ["proxy_kurulu", "proxy_mod"], ...extra })(d);
  read("durum", T2, {}, { outKeys: ["sistem_kurulu"], effects: ["changes-config"] })(d); // değiştiren eylem kart tanımlamaz
};
const appEl = (h, id) => h.$(`#store-grid .app-card[data-setcard="${id}"]`);

test("listede olmayan uygulama kartı: rozet, süzgeç ve sayılar; uygulama olmayan ayar kartı sayılmaz", async () => {
  const h = await open(compose(overview(T1), proxy({ proxy_kurulu: "evet", proxy_mod: "docker" })));
  assert.equal(appEl(h, "proxy").dataset.st, "on"); assert.match(appEl(h, "proxy").textContent.replace(/\s+/g, " "), /✔ Kurulu \(docker\)/);
  assert.equal(appEl(h, "sistem").dataset.st, "settings"); assert.doesNotMatch(appEl(h, "sistem").textContent, /Kurul|durum okunmadı/);
  assert.deepEqual(chips(h), ["Tümü 6", "Kurulu 3", "Kurulabilir 2"]);
  await h.click('#srv-body [data-sf="on"]'); assert.equal(appEl(h, "proxy").hidden, false); assert.equal(appEl(h, "sistem").hidden, true);
  assert.deepEqual(h.errors, []); h.close();
});

test("listede olmayan uygulama: okunmadan 'durum okunmadı'; genel okumanın listesi onu 'Kurulmadı' yapmaz", async () => {
  const bos = await open(compose(overview(T2), proxy({}, { lastTime: "" })));
  assert.equal(appEl(bos, "proxy").dataset.st, "?"); assert.match(appEl(bos, "proxy").textContent, /durum okunmadı/); bos.close();
  const eski = await open(compose(overview("2026-11-10T12:00:00Z"), proxy({ proxy_kurulu: "evet" })));
  assert.equal(appEl(eski, "proxy").dataset.st, "on", "daha yeni genel okuma uygulamayı bilmiyor"); eski.close();
});


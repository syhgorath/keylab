"use strict";
/* Sunucu sayfası DAVRANIŞ testleri 3/3 (refaktör 5d): yazılım kataloğu (eski kurulum + genel `items` paketleri). Taşımadan ÖNCE eski kodda da geçer. */
const test = require("node:test");
const assert = require("node:assert/strict");
const { boot, fixture } = require("./harness");

const mixedHome = () => { const h = fixture("home"); h.servers.forEach((s, i) => { s.checked = true; s.up = i !== 1; s.conn = s.up ? "up" : "down"; }); return h; };
const pkg = (d, id) => d.packages.find(p => p.id === id);
const act = (d, p, a) => pkg(d, p).actions.find(x => x.id === a);
const KV = fixture("vars-web1-kurulum").vars;
const api = (f, vars) => ({
  "/api/home": mixedHome,
  "/api/server": u => { const d = fixture("server-" + u.searchParams.get("name")); d.server.up = true; f && f(d); return d; },
  "/api/vars": u => ({ vars: vars === undefined ? KV : vars }),
});
const txt = (h, s) => (typeof s === "string" ? h.$(s) : h.querySelector(s.sel)).textContent.replace(/\s+/g, " ").trim();
const ctxt = (el, sel) => el.querySelector(sel).textContent.replace(/\s+/g, " ").trim();
const open = (f, vars, hash = "#s=web1&p=kurulum") => boot({ hash, overrides: api(f, vars) });
const known = (kurulu, portlar) => d => { act(d, "kurulum", "durum").keys = { kurulu, ...(portlar ? { portlar } : {}) }; };
const card = (h, id) => h.$(`#store-grid [data-app="${id}"]`);
const varsGets = h => h.calls.filter(c => c.path.startsWith("/api/vars") && c.method === "GET").length;

/* ---------------- eski kurulum (items yok) ---------------- */
test("katalog (eski kurulum): dört uygulama kartı, kapalı düğmeler, genel ayarlar, başlık; durum okunmadı uyarısı", async () => {
  const h = await open();
  assert.equal(txt(h, ".store-head h2"), "Uygulama kataloğu"); assert.match(txt(h, ".store-head"), /Kurulum motoru v0\.1\.0 · bu USB'de\. Docker ya da yerel paket \(native\) olarak kurulur\./);
  assert.deepEqual(h.$$("#store-grid .app-card").map(c => c.dataset.app), ["docker", "nginx", "mariadb", "redis"]); assert.deepEqual(h.$$("#store-grid .app-card").map(c => c.dataset.st), ["?", "?", "?", "?"]);
  assert.deepEqual(h.$$(".app-ico").map(i => i.textContent), ["🐳", "🌐", "🗄️", "⚡"]); assert.match(ctxt(card(h, "docker"), ".app-title"), /Docker Engine\s*durum okunmadı/);
  const btns = [...card(h, "docker").querySelectorAll(".app-btns button")]; assert.deepEqual(btns.map(b => b.textContent), ["Önizle", "Kur"]); assert.ok(btns.every(b => b.disabled)); assert.match(btns[0].title, /Öğeye özel çalıştırma yakında/); assert.match(ctxt(card(h, "docker"), ".app-foot"), /Eylemler yakında/);
  assert.match(txt(h, ".store-head + .notice"), /Hangi uygulamaların kurulu olduğu henüz okunmadı\./); assert.ok(h.$('.notice [data-term="kurulum|durum"]'), "durum eylemi terminalde → Durumu oku düğmesi"); assert.equal(h.$$("[data-sf]").length, 1, "durum bilinmeyince yalnız 'Tümü' çipi");
  assert.deepEqual(h.$$(".gen-grid .kv").map(k => k.querySelector(".muted").textContent), ["Kurulum modu", "Ana klasör", "Dışarıya açık", "Eylem hedefi"]);
  assert.match(txt(h, ".store-actions summary"), /durum, preview, kur, yukselt, kaldir · kurulum_yazilim seçimine göre çalışır/); assert.match(txt(h, ".gen-card .oz-foot"), /Son durum: okunmadı/);
  assert.deepEqual(h.errors, []); h.close();
});

test("katalog: kurulu/kurulmadı rozetleri (mod), port, çipler ve süzgeç; olmayan süzgeç 'Tümü'ne düşer", async () => {
  const h = await open(known("docker:native+nginx", "nginx:80+docker:2375"));
  assert.deepEqual(h.$$("#store-grid .app-card").map(c => c.dataset.st), ["on", "on", "off", "off"]);
  assert.match(ctxt(card(h, "docker"), ".app-title"), /✔ Kurulu \(native\)/); assert.match(ctxt(card(h, "nginx"), ".app-title"), /✔ Kurulu$/); assert.match(ctxt(card(h, "mariadb"), ".app-title"), /Kurulmadı/);
  assert.match(ctxt(card(h, "docker"), ".app-meta"), /Port: 2375/); assert.equal(card(h, "mariadb").querySelector(".app-meta"), null, "kurulu değilse port gösterilmez");
  assert.deepEqual(h.$$("[data-sf]").map(b => b.dataset.sf + ":" + b.querySelector(".muted").textContent), ["all:4", "on:2", "off:2"]); assert.equal(h.$("[data-sf]").getAttribute("aria-pressed"), "true");
  assert.equal(h.$(".store-head + .notice"), null, "durum bilinince uyarı yok"); assert.match(txt(h, ".gen-card .oz-foot"), /Durumu yenile/);
  const vis = () => h.$$("#store-grid .app-card").filter(c => !c.hidden).map(c => c.dataset.app);
  await h.click('[data-sf="on"]'); assert.deepEqual(vis(), ["docker", "nginx"]); assert.equal(h.$('[data-sf="on"]').getAttribute("aria-pressed"), "true"); assert.equal(h.$('[data-sf="all"]').getAttribute("aria-pressed"), "false");
  await h.click('[data-sf="off"]'); assert.deepEqual(vis(), ["mariadb", "redis"]);
  await h.click('#srv-tabs [data-p="kurulum"]'); assert.deepEqual(vis(), ["mariadb", "redis"], "aynı sekmede yeniden çizimde süzgeç korunur");
  assert.deepEqual(h.errors, []); h.close();
});

test("katalog: eski kurulumda kurulu kartta Yükselt/Kaldır kapalı düğmeleri; 'kurulu' anahtarı 'yok' → hiçbiri kurulu değil", async () => {
  let h = await open(known("docker")); assert.deepEqual([...card(h, "docker").querySelectorAll(".app-btns button")].map(b => b.textContent), ["Önizle", "Yükselt", "Kaldır"]); assert.deepEqual([...card(h, "nginx").querySelectorAll(".app-btns button")].map(b => b.textContent), ["Önizle", "Kur"]); h.close();
  h = await open(known("yok")); assert.deepEqual(h.$$("#store-grid .app-card").map(c => c.dataset.st), ["off", "off", "off", "off"]); assert.match(txt(h, ".gen-card .oz-foot"), /Son durum: /); h.close();
});

test("katalog: süzgeç sekmeler arasında 'Tümü'ne düşer (durum bilinmeyen sunucuda çipler yok)", async () => {
  const h = await open(known("docker")); await h.click('[data-sf="on"]');
  await h.click('#srv-tabs [data-p="utility"]'); await h.click('#srv-tabs [data-p="kurulum"]'); assert.equal(h.$('[data-sf="on"]').getAttribute("aria-pressed"), "true", "süzgeç değişkeni sekme dönüşünde de durur");
  h.close();
  const g = await open(); assert.equal(g.$("[data-sf='on']"), null); g.close();
});

/* ---------------- değişkenler ---------------- */
test("katalog: değişkenler yüklenirken 'yükleniyor…', sonra gruba göre satırlar; ad kısaltma; ayarlı rozeti", async () => {
  let release; const gate = new Promise(r => { release = r; });
  const h = await boot({ hash: "#s=web1&p=kurulum", overrides: { ...api(), "/api/vars": async () => { await gate; return { vars: KV.map(v => (v.name === "kurulum_docker_bridge" || /docker/.test(v.name) ? { ...v, set: true, value: "ZZ" } : v)) }; } } });
  assert.match(ctxt(card(h, "docker"), ".app-vars"), /Değişkenler yükleniyor…/); release(); await h.settle();
  assert.doesNotMatch(ctxt(card(h, "docker"), ".app-vars"), /yükleniyor/);
  const rows = card(h, "docker").querySelectorAll(".av-row"); assert.ok(rows.length > 0); assert.ok([...rows].every(r => !r.querySelector(".av-name").textContent.startsWith("kurulum_")), "paket ön eki ve öğe ön eki kısaltılır");
  assert.ok([...rows].some(r => r.querySelector(".badge.ok")), "ayarlı rozeti"); assert.ok([...rows].every(r => r.querySelector(".av-name").title.startsWith("kurulum_")));
  assert.equal(varsGets(h), 1, "yükleme tek sefer (döngü yok)"); assert.deepEqual(h.errors, []); h.close();
});

test("katalog: değişken yoksa 'Değişken bulunamadı.'; yükleme hatasında döngü yok ve 'yükleniyor…' kalır", async () => {
  let h = await open(null, []); assert.match(ctxt(card(h, "docker"), ".app-vars"), /Değişken bulunamadı\./); assert.match(txt(h, ".gen-card"), /Eylem hedefi\s*—/); assert.equal(varsGets(h), 1); h.close();
  h = await boot({ hash: "#s=web1&p=kurulum", overrides: { ...api(), "/api/vars": { status: 500, body: "bozuk" } } });
  assert.match(ctxt(card(h, "docker"), ".app-vars"), /Değişkenler yükleniyor…/); assert.equal(varsGets(h), 1, "başarısızlıkta yeniden deneme döngüsü yok"); h.close();
});

test("katalog: genel ayarlar değerleri (varsayılan ya da ayarlı) ve değer boşsa '—'", async () => {
  const h = await open(null, KV.map(v => (v.name === "kurulum_mod" ? { ...v, set: true, value: "docker" } : v)));
  const kv = n => h.$$(".gen-grid .kv").find(k => k.querySelector(".muted").textContent === n);
  assert.equal(kv("Kurulum modu").querySelector(".chip").textContent, "docker"); assert.equal(kv("Kurulum modu").querySelector(".chip").title, "kurulum_mod");
  const def = KV.find(v => v.name === "kurulum_yazilim").default; assert.equal(kv("Eylem hedefi").querySelector(".chip").textContent, def || "—"); h.close();
});

test("katalog: sunucu değişince değişkenler yeniden okunur; aynı sunucu/pakette tekrar çizimde okunmaz", async () => {
  const h = await open(); const base = varsGets(h);
  await h.click('#srv-tabs [data-p="kurulum"]'); await h.click('#srv-tabs [data-p="kurulum"]'); assert.equal(varsGets(h), base, "yeniden çizimde yeniden okuma yok");
  await h.click("#srv-back"); await h.click('[data-tsrv="db1"]'); await h.click('#srv-tabs [data-p="kurulum"]'); assert.equal(varsGets(h), base + 1, "başka sunucu için okunur");
  assert.ok(h.calls.some(c => c.path === "/api/vars?server=db1&package=kurulum")); h.close();
});

/* ---------------- değişken pencereleri ve eylem bağları ---------------- */
test("katalog: 'Düzenle…' öğenin grubuyla, 'Tüm değişkenler…' grupsuz açar", async () => {
  const h = await open();
  const b = card(h, "docker").querySelector("[data-kvars]"); assert.ok(b.dataset.kvars === "Docker" || b.dataset.kvars === "", "tek gruplu öğede grup adı");
  await h.click(b); assert.ok(h.$("#varsmodal").classList.contains("open")); const withGroup = b.dataset.kvars; assert.equal(h.$("#vars-title").textContent, "Değişkenler · Kurulum" + (withGroup ? " · " + withGroup : ""));
  if (withGroup) assert.ok(Number(h.$("#vars-count").textContent.split(" / ")[1]) < KV.length);
  await h.click("#vars-close"); await h.click("[data-kvars-all]"); assert.equal(h.$("#vars-title").textContent, "Değişkenler · Kurulum"); assert.equal(h.$("#vars-count").textContent, `${KV.length} / ${KV.length}`); h.close();
});

test("katalog: Genel eylemler bölümü (süzgeç çipleri, düğmeler) çalışır", async () => {
  const h = await open(); h.$(".store-actions").open = true;
  assert.ok(h.$$(".store-actions [data-actcat]").length >= 2); await h.click('.store-actions [data-actcat="read"]'); assert.ok(h.$$("#act-list > .act[data-cat]").filter(a => !a.hidden).every(a => a.dataset.cat === "read"));
  await h.click('.store-actions [data-term="kurulum|preview"]'); assert.ok(h.$("#runmodal").classList.contains("open")); assert.match(h.$("#run-title").textContent, /preview/); h.close();
});

/* ---------------- genel `items` bildiren paket ---------------- */
const itemsPkg = (extra = {}) => d => {
  const k = pkg(d, "kurulum");
  Object.assign(k, { id: "kurulum", version: "0.2.0", description: "Katalog açıklaması", itemVar: "kurulum_yazilim", items: [
    { id: "docker", label: "Docker Engine", description: "Konteyner", params: ["kurulum_mod"], actions: ["preview", "kur", "yukselt", "kaldir"],
      states: { preview: { runnable: true }, kur: { write: true, reason: "yazar" }, yukselt: { terminal: true }, kaldir: { write: true, destructive: true, reason: "yıkıcı" } } },
    { id: "nginx", label: "nginx web", description: "Proxy", params: [], actions: ["preview", "kur"], states: { preview: { reason: "önizleme kapalı" } } },
    { id: "ozel", label: "özel öğe", description: "", params: [], actions: [] },
  ], ...extra });
  k.actions.find(a => a.id === "durum").keys = { kurulu: "docker:native", portlar: "docker:2375" };
};
test("katalog (items): öğe eylem düğmeleri — durum türüne göre; kuruluya Kur gösterilmez, kurulu değile Yükselt/Kaldır gösterilmez", async () => {
  const h = await open(itemsPkg());
  const b = id => [...card(h, id).querySelectorAll(".app-btns button")];
  assert.deepEqual(b("docker").map(x => x.textContent), ["Önizle", "Yükselt", "Kaldır"]); assert.deepEqual(b("nginx").map(x => x.textContent), ["Önizle", "Kur"]);
  assert.equal(b("docker")[0].dataset.irun, "kurulum|preview|docker"); assert.equal(b("docker")[1].dataset.iterm, "kurulum|yukselt|docker"); assert.equal(b("docker")[2].dataset.iwrite, "kurulum|kaldir|docker"); assert.ok(b("docker")[2].classList.contains("dangerbtn")); assert.equal(b("docker")[2].title, "yıkıcı");
  assert.ok(b("nginx")[0].disabled); assert.equal(b("nginx")[0].title, "önizleme kapalı"); assert.ok(b("nginx")[1].disabled); assert.equal(b("nginx")[1].title, "durum okunamadı");
  assert.match(ctxt(card(h, "docker"), ".app-foot"), /Önizlemeden sonra kurulur/); assert.match(ctxt(card(h, "ozel"), ".app-foot"), /Eylemler yakında/); assert.ok([...card(h, "ozel").querySelectorAll(".app-btns button")].every(x => x.disabled), "eylem bildirmeyen öğe eskisi gibi kapalı");
  assert.equal(card(h, "ozel").querySelector(".app-ico").textContent, "Ö", "simge: etiketin büyük harfli ilk harfi"); assert.equal(card(h, "docker").querySelector(".app-ico").textContent, "🐳"); assert.equal(card(h, "nginx").querySelector(".app-ico").textContent, "🌐");
  assert.match(txt(h, ".store-head"), /Kurulum merkezi: Docker, nginx, MariaDB, Redis v0\.2\.0 · bu USB'de\. Katalog açıklaması/); assert.doesNotMatch(txt(h, ".store-head"), /Kurulum motoru/);
  assert.match(txt(h, ".store-actions summary"), /kurulum_yazilim seçimine göre çalışır/); h.close();
});

test("katalog (items): öğe düğmeleri iş pencerelerini açar (Çalıştır/Onayla/Terminal), öğe kimliğiyle", async () => {
  let h = await open(itemsPkg()); await h.click('[data-irun="kurulum|preview|docker"]'); assert.ok(h.$("#runmodal").classList.contains("open")); assert.match(h.$("#run-title").textContent, /preview/); h.close();
  h = await open(itemsPkg()); await h.click('[data-iwrite="kurulum|kaldir|docker"]'); assert.ok(h.$("#runmodal").classList.contains("open")); assert.match(h.$("#run-title").textContent, /kaldir/); h.close();
  h = await open(itemsPkg()); await h.click('[data-iterm="kurulum|yukselt|docker"]'); assert.ok(h.$("#runmodal").classList.contains("open")); assert.match(h.$("#run-title").textContent, /yukselt/); h.close();
});

test("katalog (items): ortak ayarlar = hiçbir öğeye ait olmayan ilk 8 değişken (paket ön eki kısaltılır); itemVar yoksa özet boş", async () => {
  const vars = Array.from({ length: 10 }, (_, i) => ({ name: "kurulum_o" + i, type: "string", default: "d" + i, value: "", set: false, group: "", description: "" })).concat([{ name: "kurulum_mod", type: "string", default: "native", value: "", set: false, group: "", description: "" }, { name: "diger_ad", type: "string", default: "", value: "", set: false, group: "", description: "" }]);
  const h = await open(itemsPkg({ itemVar: "" }), vars);
  assert.equal(h.$$(".gen-grid .kv").length, 8); assert.equal(h.$$(".gen-grid .kv .muted")[0].textContent, "o0"); assert.equal(h.$$(".gen-grid .kv .chip")[0].textContent, "d0"); assert.doesNotMatch(txt(h, ".gen-grid"), /\bmod\b/, "öğeye ait (params) değişken ortak ayarlarda değil");
  assert.doesNotMatch(txt(h, ".store-actions summary"), /seçimine göre/); assert.match(ctxt(card(h, "docker"), ".app-vars"), /mod/); h.close();
  const g = await open(itemsPkg(), []); assert.match(txt(g, ".gen-grid"), /Ortak ayar yok\./); g.close();
});

test("katalog (items): öğeye grup eşleşmesi — params yoksa değişkenin grubu öğenin grubuna eşit olmalı (genel öğelerde group boş: değişken eşleşmez)", async () => {
  const vars = [{ name: "kurulum_x", type: "string", default: "1", value: "", set: false, group: "G", description: "" }];
  const h = await open(itemsPkg(), vars); assert.match(ctxt(card(h, "nginx"), ".app-vars"), /Değişken bulunamadı\./); assert.equal(card(h, "nginx").querySelector("[data-kvars]").dataset.kvars, ""); assert.match(txt(h, ".gen-grid"), /x\s*1/); h.close();
});

test("katalog: katalog olmayan paket (utility) normal sayfa çizer; kurulum sekmesinden çıkınca katalog kalmaz", async () => {
  const h = await open(); assert.ok(h.$("#store-grid")); await h.click('#srv-tabs [data-p="utility"]'); assert.equal(h.$("#store-grid"), null); assert.ok(h.$(".pk-info")); h.close();
});

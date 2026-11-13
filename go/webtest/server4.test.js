"use strict";
/* Sunucu sayfası ek testleri (refaktör 5d): mutasyon turunda kaçanlar. */
const test = require("node:test");
const assert = require("node:assert/strict");
const { boot, fixture } = require("./harness");

const mixedHome = () => { const h = fixture("home"); h.servers.forEach((s, i) => { s.checked = true; s.up = i !== 1; s.conn = s.up ? "up" : "down"; }); return h; };
const srv = (f, vars) => ({ "/api/home": mixedHome, "/api/vars": () => ({ vars: vars || fixture("vars-web1-kurulum").vars }), "/api/server": u => { const d = fixture("server-" + u.searchParams.get("name")); d.server.up = d.server.name !== "vm1"; d.server.conn = d.server.up ? "up" : "down"; f && f(d); return d; } });
const pkg = (d, id) => d.packages.find(p => p.id === id);
const act = (d, p, a) => pkg(d, p).actions.find(x => x.id === a);
const txt = (h, s) => h.$(s).textContent.replace(/\s+/g, " ").trim();
const open = (hash, f, vars) => boot({ hash, overrides: srv(f, vars) });

test("özet: TCP rozeti sınıfları (denetleniyor → plain, knock → warn, erişilemiyor → danger, açık → ok)", async () => {
  const cls = async f => { const h = await open("#s=web1", d => f(d.server)); const c = h.$(".oz-top .badge").className; h.close(); return c; };
  assert.match(await cls(s => { s.checked = false; }), /\bplain\b/); assert.match(await cls(s => { s.up = false; s.knock = true; }), /\bwarn\b/); assert.match(await cls(s => { s.up = false; }), /\bdanger\b/); assert.match(await cls(() => {}), /\bok\b/);
  assert.match(await cls(s => { s.valid = false; }), /\bwarn\b/);
});

test("eylem kartı: eski sonuç rozeti — eski değiştiren eylem 'güncel olmayabilir' olmaz; yalnız salt okunur olanlar olur", async () => {
  const h = await open("#s=web1&p=utility", d => { pkg(d, "utility").actions = [
    { id: "chg_old", applicable: true, runnable: true, effects: ["changes-config"], lastTime: "2026-11-10T10:00:00Z", lastResult: "ok" },
    { id: "chg_new", applicable: true, runnable: true, effects: ["changes-config"], lastTime: "2026-11-10T12:00:00Z", lastResult: "ok" },
    { id: "rd_old", applicable: true, runnable: true, effects: [], lastTime: "2026-11-10T09:00:00Z", lastResult: "ok" },
    { id: "rd_new", applicable: true, runnable: true, effects: [], lastTime: "2026-11-10T13:00:00Z", lastResult: "ok" },
  ]; });
  const c = id => h.$$("#act-list > .act").find(a => a.querySelector(".act-name").textContent === id);
  assert.equal(c("chg_old").querySelector(".stale"), null); assert.equal(c("chg_new").querySelector(".stale"), null); assert.ok(c("rd_old").querySelector(".stale")); assert.equal(c("rd_new").querySelector(".stale"), null); h.close();
});

test("değişken kartı: açıklama 150 karakteri aşınca kırpılır (170 karakter kırpılır, 150 kırpılmaz)", async () => {
  const w = n => ("a".repeat(9) + " ").repeat(Math.ceil(n / 10)).slice(0, n);
  const h = await open("#s=web1&p=utility", d => { pkg(d, "utility").vars = [{ name: "v170", set: true, value: "1", default: "", description: w(170) + "x", needed: false }, { name: "v150", set: true, value: "1", default: "", description: w(150), needed: false }]; });
  const d = n => [...h.$$(".pk-top .urow")].find(r => r.textContent.includes(n)).querySelector(".small.muted").textContent;
  assert.ok(d("v170").endsWith("…")); assert.ok(!d("v150").endsWith("…")); assert.equal(d("v150").length, 150); h.close();
});

test("hardening: audit hem runnable hem write ise 'Çalıştır' (data-run) kullanılır; durum kartı düğmesi de aynı sırayı izler", async () => {
  const h = await open("#s=web1&p=linux-hardening", d => { const a = act(d, "linux-hardening", "audit"); a.runnable = true; a.write = true; a.terminal = true; });
  assert.ok(h.$('.hero-card [data-run="linux-hardening|audit"]')); assert.equal(h.$('.hero-card [data-write="linux-hardening|audit"]'), null); h.close();
  const g = await open("#s=web1&p=utility", d => { const a = act(d, "utility", "durum"); a.runnable = true; a.write = true; });
  assert.ok(g.$('.st-card [data-run="utility|durum"]')); assert.equal(g.$('.st-card [data-write="utility|durum"]'), null); g.close();
});

test("son durum: sapma sayı değilse ton verilmez", async () => {
  const h = await open("#s=web1&p=utility", d => { Object.assign(act(d, "utility", "durum"), { keys: { offset_ms: "1", sapma_ms: "abc" }, lastTime: "2026-11-10T11:00:00Z", lastResult: "ok" }); });
  const v = h.$(".st-tile .st-val"); assert.ok(!v.classList.contains("ok") && !v.classList.contains("warn")); assert.match(h.$(".st-tile").textContent, /Sapma: abc ms/); h.close();
});

/* ---------------- katalog ---------------- */
const items = extra => d => {
  const k = pkg(d, "kurulum");
  Object.assign(k, { version: "0.2.0", itemVar: "kurulum_yazilim", items: [
    { id: "docker", label: "Docker", description: "", params: ["kurulum_mod", "kurulum_x"], actions: ["preview", "kur", "yukselt"], states: { preview: { runnable: true, write: true }, kur: { write: true }, yukselt: { terminal: true } } },
    { id: "nginx", label: "nginx", description: "", params: ["kurulum_x"], actions: ["preview"], states: { preview: { runnable: true } } },
  ] });
  k.actions.find(a => a.id === "durum").keys = { kurulu: "docker", portlar: "nginx:80+docker:2375" };
  extra && extra(d);
};
const V = (name, group, def = "d") => ({ name, type: "string", default: def, value: "", set: false, group, description: "" });
const card = (h, id) => h.$(`#store-grid [data-app="${id}"]`);

test("katalog: öğe tek grupluysa düzenle düğmesi grup adını taşır, çok gruplu/grupsuzsa boş", async () => {
  const vars = [V("kurulum_mod", "A"), V("kurulum_x", "B"), V("kurulum_y", "A")];
  const h = await open("#s=web1&p=kurulum", items(), vars);
  assert.equal(card(h, "docker").querySelector("[data-kvars]").dataset.kvars, "", "iki grup → boş"); assert.equal(card(h, "nginx").querySelector("[data-kvars]").dataset.kvars, "B", "tek grup → ad"); h.close();
  const g = await open("#s=web1&p=kurulum", items(), [V("kurulum_mod", ""), V("kurulum_x", "")]); assert.equal(card(g, "docker").querySelector("[data-kvars]").dataset.kvars, "", "grupsuz → boş"); g.close();
});

test("katalog: port yalnız kurulu öğede gösterilir", async () => {
  const h = await open("#s=web1&p=kurulum", items(), [V("kurulum_mod", "A")]);
  assert.match(card(h, "docker").querySelector(".app-meta").textContent, /Port: 2375/); assert.equal(card(h, "nginx").querySelector(".app-meta"), null, "nginx kurulu değil, portu 80 yazsa da"); h.close();
});

test("katalog: öğe düğmesi türü — runnable önce gelir (write de olsa)", async () => {
  const h = await open("#s=web1&p=kurulum", items(), [V("kurulum_mod", "A")]);
  const b = card(h, "docker").querySelector(".app-btns button"); assert.equal(b.textContent, "Önizle"); assert.ok(b.dataset.irun); assert.equal(b.dataset.iwrite, undefined); h.close();
});

test("katalog: öğelere ait (params) değişkenler ortak ayarlarda tekrarlanmaz", async () => {
  const vars = [V("kurulum_mod", "A", "native"), V("kurulum_x", "A", "xx"), V("kurulum_serbest", "", "free")];
  const h = await open("#s=web1&p=kurulum", items(), vars);
  const gen = [...h.$$(".gen-grid .kv")].map(k => k.querySelector(".muted").textContent); assert.deepEqual(gen, ["serbest"]); h.close();
});

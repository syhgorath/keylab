"use strict";
/* Ayar kartları (paketin `settings` bildirimi): istenen ↔ sunucuda okunan değer yan yana; durum arka uçtan gelir (pkgsettings.go), arayüz yalnız gösterir.
   Hiçbir okuma kendiliğinden başlamaz; düğmeler paketin mevcut eylemlerini (ve kapılarını) kullanır. */
process.env.TZ = "UTC";
const test = require("node:test");
const assert = require("node:assert/strict");
const { boot, fixture } = require("./harness");
const { mixedHome } = require("./lib/fakejobs");

const set = (id, status, extra) => ({ id, label: "Ad " + id, group: "NTP", var: "utility_" + id, applyVia: ["ntp_ayarla"], readAction: "ntp_durum", desired: [], actual: [], status, ...extra });
const SETTINGS = [
  set("kaynak", "drift", { desired: ["0.tr.pool.ntp.org"], actual: ["eski.example.org"], risk: ["time-jump", "uydurma"], riskNote: "not" }),
  set("bilinmez", "unknown", { desired: ["x"], note: "henüz okunmadı" }),
  set("bos", "unmanaged"),
  set("adim", "unreadable", { desired: ["true"], readAction: "" }),
  set("zone", "match", { group: "Zaman", desired: ["Europe/Istanbul"], actual: ["Europe_Istanbul"], readAction: "durum", applyVia: ["apply"] }),
];
const withSettings = (settings, f) => d => {
  const u = d.packages.find(p => p.id === "utility"), base = u.actions.find(a => a.id === "durum");
  const mk = (id, extra) => ({ ...JSON.parse(JSON.stringify(base)), id, keys: {}, applicable: true, runnable: false, write: false, terminal: false, destructive: false, effects: [], requires: [], prereqs: [], lastTime: "", lastResult: "", ...extra });
  u.actions = u.actions.filter(a => !["preview", "apply", "ntp_durum", "ntp_ayarla"].includes(a.id));
  u.actions.push(mk("ntp_durum", { runnable: true }), mk("preview", { terminal: true }),
    mk("ntp_ayarla", { write: true, effects: ["changes-config"], requires: ["preview"] }), mk("apply", { effects: ["changes-config"], requires: ["preview"], reason: "önce preview" }));
  u.settings = settings; if (f) f(u);
};
const open = async f => {
  const h = await boot({ hash: "#s=web1&p=linux-hardening", overrides: {
    "/api/home": mixedHome,
    "/api/server": u => { const d = fixture("server-" + u.searchParams.get("name")); d.server.up = true; if (f) f(d); return d; },
    "/api/vars": () => ({ vars: [] }),
  } });
  await h.click('#srv-tabs [data-p="utility"]');
  return h;
};
const txt = el => el.textContent.replace(/\s+/g, " ").trim();
const row = (h, id) => h.$(`#srv-body [data-set="${id}"]`);

test("değişken grubuna göre kartlar, sayfanın en üstünde; satırlar istenen ve sunucudaki değeri durumuyla gösterir", async () => {
  const h = await open(withSettings(SETTINGS));
  const cards = h.$$("#srv-body .set-card");
  assert.deepEqual(cards.map(c => c.dataset.setgroup), ["NTP", "Zaman"]);
  assert.equal(h.$("#srv-body").firstElementChild, cards[0], "en üstte");
  assert.deepEqual(h.$$("#srv-body .set-row[data-set]").map(r => [r.dataset.set, r.dataset.st]),
    [["kaynak", "drift"], ["bilinmez", "unknown"], ["bos", "unmanaged"], ["adim", "unreadable"], ["zone", "match"]]);
  const k = row(h, "kaynak");
  assert.match(txt(k.querySelector(".set-want")), /0\.tr\.pool\.ntp\.org/);
  assert.match(txt(k.querySelector(".set-have")), /eski\.example\.org/);
  assert.equal(txt(k.querySelector(".set-state")), "⚠ sapma");
  assert.deepEqual([...k.querySelectorAll(".set-risk .badge")].map(txt), ["saati sıçratır"], "sözlük dışı risk gösterilmez");
  assert.match(txt(row(h, "bilinmez").querySelector(".set-state")), /bilinmiyor.*henüz okunmadı/);
  assert.match(txt(row(h, "bos").querySelector(".set-state")), /yönetilmiyor/);
  assert.match(txt(row(h, "adim").querySelector(".set-state")), /okunamaz/);
  assert.equal(txt(row(h, "zone").querySelector(".set-have .chip")), "Europe/Istanbul", "uyumluysa istenen yazım");
  assert.deepEqual([...cards[0].querySelectorAll(".set-sum .badge")].map(txt), ["1 sapma", "1 bilinmiyor"]);
  assert.deepEqual([...cards[1].querySelectorAll(".set-sum .badge")].map(txt), ["uyumlu"]);
  assert.deepEqual(h.errors, []); h.close();
});

test("düğmeler paketin eylemleridir: Kontrol et, Önizle, Uygula; çalıştırılamayan eylem kapalı ve nedeni yazılı", async () => {
  const h = await open(withSettings(SETTINGS));
  const [ntp, zaman] = h.$$("#srv-body .set-card");
  assert.deepEqual([...ntp.querySelectorAll(".set-foot .btn")].map(txt), ["Kontrol et", "Önizle", "Uygula: ntp_ayarla"]);
  assert.ok(ntp.querySelector('.set-foot [data-run="utility|ntp_durum"]'));
  assert.ok(ntp.querySelector('.set-foot [data-term="utility|preview"]'));
  assert.ok(ntp.querySelector('.set-foot [data-write="utility|ntp_ayarla"]'));
  const apply = [...zaman.querySelectorAll(".set-foot .btn")].find(b => txt(b) === "Uygula: apply");
  assert.ok(apply.disabled); assert.equal(apply.title, "önce preview");
  assert.match(txt(ntp.querySelector(".set-foot")), /ntp_durum henüz okunmadı/);
  assert.ok(ntp.querySelector('[data-svars="NTP"]'));
  h.close();
});

test("eskimiş okuma kart başlığında uyarılır", async () => {
  const h = await open(withSettings([set("kaynak", "match", { desired: ["a"], actual: ["a"], stale: true })]));
  assert.deepEqual(h.$$("#srv-body .set-sum .badge").map(txt), ["uyumlu", "güncel olmayabilir"]); h.close();
});

test("paket metni HTML olarak yorumlanmaz", async () => {
  const bad = '<img src=x onerror="window.__x=1">';
  const h = await open(withSettings([set("kaynak", "drift", { label: bad, group: bad, description: bad, desired: [bad], actual: [bad], note: bad, riskNote: bad, risk: ["data-loss"] })]));
  assert.equal(h.$("#srv-body img"), null); assert.equal(h.win.__x, undefined); h.close();
});

test("settings yoksa kart yok", async () => {
  const h = await open(null);
  assert.equal(h.$("#srv-body .set-card"), null); h.close();
});

test("katalog sayfasında: settings varsa genel kartlar elle yazılmış 'Saat ve NTP' kartının yerini alır", async () => {
  const catalog = u => { const d = u.actions.find(a => a.id === "durum"); u.actions.push({ ...JSON.parse(JSON.stringify(d)), id: "kurulum_durum", keys: {} }); };
  const h = await open(withSettings(SETTINGS, catalog));
  assert.ok(h.$("#store-grid"), "katalog çizildi"); assert.ok(h.$("#srv-body .set-card")); assert.equal(h.$("#srv-body .ntp-card"), null); h.close();
  const eski = await open(withSettings([], catalog));
  assert.ok(eski.$("#srv-body .ntp-card"), "settings'siz sürümde eski kart"); eski.close();
});

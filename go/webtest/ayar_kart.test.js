"use strict";
/* Ayar kartı (paketin `settings` bildirimi): istenen ↔ sunucuda okunan değer yan yana; durum arka uçtan gelir (pkgsettings.go), arayüz yalnız gösterir.
   Tek kart, değişken grubu başına başlık satırı; okunmayan seçenekler "Uygulama seçenekleri" satırında; açıklamalar ada tıklayınca açılır.
   Hiçbir okuma kendiliğinden başlamaz; düğmeler paketin mevcut eylemlerini (ve kapılarını) kullanır. */
process.env.TZ = "UTC";
const test = require("node:test");
const assert = require("node:assert/strict");
const { boot, fixture } = require("./harness");
const { mixedHome } = require("./lib/fakejobs");

const set = (id, status, extra) => ({ id, label: "Ad " + id, group: "NTP", var: "utility_" + id, applyVia: ["ntp_ayarla"], readAction: "ntp_durum", desired: [], actual: [], status, ...extra });
const SETTINGS = [
  set("kaynak", "drift", { desired: ["0.tr.pool.ntp.org"], actual: ["eski.example.org"], risk: ["time-jump", "uydurma"], riskNote: "risk notu", description: "uzun açıklama" }),
  set("bilinmez", "unknown", { desired: ["x"], note: "henüz okunmadı" }),
  set("adim", "unreadable", { desired: ["true"], readAction: "", risk: ["time-jump"] }),
  set("kapali", "unreadable", { desired: ["false"], readAction: "" }),
  set("platform", "unreadable", { desired: ["y"], readAction: "", note: "bu sunucuda okunamıyor" }),
  set("zone", "match", { group: "Zaman", desired: ["Europe/Istanbul"], actual: ["Europe_Istanbul"], readAction: "durum", applyVia: ["apply"] }),
  set("dil", "unmanaged", { group: "Dil", actual: ["unset"], readAction: "durum", applyVia: ["apply"] }),
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
const group = (h, g) => h.$(`#srv-body .set-group[data-setgroup="${g}"]`);
const buttons = (h, g) => [...group(h, g).querySelectorAll(".set-ghead .btn")];

test("tek kart, en üstte; değişken grupları bölüm olur; satırlar istenen ve sunucudaki değeri durumuyla gösterir", async () => {
  const h = await open(withSettings(SETTINGS));
  assert.equal(h.$$("#srv-body .set-card").length, 1);
  assert.equal(h.$("#srv-body").firstElementChild, h.$("#srv-body .set-card"), "en üstte");
  assert.equal(h.$$("#srv-body .set-row.head").length, 1, "tablo başlığı bir kez");
  assert.deepEqual(h.$$("#srv-body .set-group").map(g => g.dataset.setgroup), ["NTP", "Zaman", "Dil"]);
  assert.deepEqual(h.$$("#srv-body .set-row[data-set]").map(r => [r.dataset.set, r.dataset.st]),
    [["kaynak", "drift"], ["bilinmez", "unknown"], ["platform", "unreadable"], ["zone", "match"], ["dil", "unmanaged"]]);
  const k = row(h, "kaynak");
  assert.match(txt(k.querySelector(".set-want")), /0\.tr\.pool\.ntp\.org/);
  assert.match(txt(k.querySelector(".set-have")), /eski\.example\.org/);
  assert.equal(txt(k.querySelector(".set-state")), "⚠ sapma");
  assert.deepEqual([...k.querySelectorAll(".set-risk .badge")].map(txt), ["saati sıçratır"], "sözlük dışı risk gösterilmez");
  assert.equal(txt(row(h, "bilinmez").querySelector(".set-note")), "henüz okunmadı");
  assert.match(txt(row(h, "platform")), /okunamaz.*bu sunucuda okunamıyor/);
  assert.equal(txt(row(h, "zone").querySelector(".set-have .chip")), "Europe/Istanbul", "uyumluysa istenen yazım");
  assert.deepEqual(h.$$("#srv-body .card-head .set-sum .badge").map(txt), ["1 sapma", "1 bilinmiyor"]);
  assert.ok(h.$('#srv-body .set-card [data-svars=""]'));
  assert.deepEqual(h.errors, []); h.close();
});

test("okunmayan seçenekler tabloya girmez: tek 'Uygulama seçenekleri' satırı, aç/kapa sözcükle, 'okunamaz' demeden", async () => {
  const h = await open(withSettings(SETTINGS));
  const opts = group(h, "NTP").querySelector(".set-opts");
  assert.deepEqual([...opts.querySelectorAll(".set-opt")].map(o => [o.dataset.set, txt(o.querySelector(".chip"))]), [["adim", "açık"], ["kapali", "kapalı"]]);
  assert.match(txt(opts), /Uygulama seçenekleri:/); assert.doesNotMatch(txt(opts), /okunamaz/);
  assert.deepEqual([...opts.querySelectorAll(".set-opt .badge")].map(txt), ["saati sıçratır"]);
  assert.equal(group(h, "Zaman").querySelector(".set-opts"), null);
  h.close();
});

test("açıklama kapalı gelir, ada tıklayınca açılır ve sayfa yeniden çizilince açık kalır", async () => {
  const h = await open(withSettings(SETTINGS));
  const det = () => row(h, "kaynak").querySelector("details.set-info");
  assert.equal(det().open, false); assert.match(txt(det()), /uzun açıklama.*risk notu/);
  assert.equal(row(h, "zone").querySelector("details"), null, "açıklaması olmayan satırda açılır yok");
  det().open = true; det().dispatchEvent(new h.win.Event("toggle"));
  await h.click('#srv-tabs [data-p="linux-hardening"]'); await h.click('#srv-tabs [data-p="utility"]');
  assert.equal(det().open, true); h.close();
});

test("grup düğmeleri paketin eylemleridir; çalıştırılamayan eylem kapalı ve nedeni yazılı; yönetilmeyen grupta yalnız Kontrol et", async () => {
  const h = await open(withSettings(SETTINGS));
  assert.deepEqual(buttons(h, "NTP").map(txt), ["Kontrol et", "Önizle", "Uygula"]);
  assert.ok(group(h, "NTP").querySelector('.set-ghead [data-run="utility|ntp_durum"]'));
  assert.ok(group(h, "NTP").querySelector('.set-ghead [data-term="utility|preview"]'));
  assert.ok(group(h, "NTP").querySelector('.set-ghead [data-write="utility|ntp_ayarla"][title="ntp_ayarla"]'));
  const apply = buttons(h, "Zaman").find(b => txt(b) === "Uygula");
  assert.ok(apply.disabled); assert.equal(apply.title, "önce preview");
  assert.deepEqual(buttons(h, "Dil").map(txt), ["Kontrol et"]);
  assert.equal(txt(group(h, "NTP").querySelector(".set-when")), "Son okuma: henüz okunmadı");
  h.close();
});

test("bir grupta birden çok uygulama eylemi varsa düğmeler eylem adını taşır", async () => {
  const h = await open(withSettings([set("a", "drift", { desired: ["1"] }), set("b", "drift", { desired: ["2"], applyVia: ["apply"] })]));
  assert.deepEqual(buttons(h, "NTP").map(txt), ["Kontrol et", "Önizle", "Uygula: ntp_ayarla", "Uygula: apply"]);
  h.close();
});

test("eskimiş okuma grup başlığında uyarılır; hepsi uyumluysa kart başlığında 'uyumlu'", async () => {
  const h = await open(withSettings([set("kaynak", "match", { desired: ["a"], actual: ["a"], stale: true })]));
  assert.deepEqual(h.$$("#srv-body .card-head .set-sum .badge").map(txt), ["uyumlu"]);
  assert.deepEqual([...group(h, "NTP").querySelectorAll(".set-ghead .badge")].map(txt), ["güncel olmayabilir"]); h.close();
});

test("paket metni HTML olarak yorumlanmaz", async () => {
  const bad = '<img src=x onerror="window.__x=1">';
  const h = await open(withSettings([
    set("kaynak", "drift", { label: bad, group: bad, description: bad, desired: [bad], actual: [bad], note: bad, riskNote: bad, risk: ["data-loss"] }),
    set("secenek", "unreadable", { label: bad, group: bad, description: bad, desired: [bad], readAction: "" }),
  ]));
  assert.equal(h.$("#srv-body img"), null); assert.equal(h.win.__x, undefined); h.close();
});

test("settings yoksa kart yok", async () => {
  const h = await open(null);
  assert.equal(h.$("#srv-body .set-card"), null); h.close();
});

test("katalog sayfasında: settings varsa genel kart elle yazılmış 'Saat ve NTP' kartının yerini alır", async () => {
  const catalog = u => { const d = u.actions.find(a => a.id === "durum"); u.actions.push({ ...JSON.parse(JSON.stringify(d)), id: "kurulum_durum", keys: {} }); };
  const h = await open(withSettings(SETTINGS, catalog));
  assert.ok(h.$("#store-grid"), "katalog çizildi"); assert.ok(h.$("#srv-body .set-card")); assert.equal(h.$("#srv-body .ntp-card"), null); h.close();
  const eski = await open(withSettings([], catalog));
  assert.ok(eski.$("#srv-body .ntp-card"), "settings'siz sürümde eski kart"); eski.close();
});

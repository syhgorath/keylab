"use strict";
/* Paket ayar kartları ve ayar penceresi (component.json "settings" / "setting_cards"). Kart yalnız durum özetini gösterir; pencerede istenen değer
   düzenlenir (Kaydet yalnız .vars dosyasını yazar) ve altında sunucudan okunan değer durur. Durum arka uçtan gelir (pkgsettings.go).
   Hiçbir okuma kendiliğinden başlamaz; Kontrol et / Önizle / Uygula paketin mevcut eylemleri ve pencereleridir. */
process.env.TZ = "UTC";
const test = require("node:test");
const assert = require("node:assert/strict");
const { boot, fixture } = require("./harness");
const { mixedHome } = require("./lib/fakejobs");

const set = (id, status, extra) => ({ id, label: "Ad " + id, group: "NTP", card: "saat", var: "v_" + id, applyVia: ["ntp_ayarla"], readAction: "ntp_durum", desired: [], actual: [], status, ...extra });
const SETTINGS = [
  set("kaynak", "drift", { desired: ["0.tr.pool.ntp.org"], actual: ["eski.example.org"], risk: ["time-jump", "uydurma"], riskNote: "risk notu", description: "uzun açıklama" }),
  set("adim", "unreadable", { desired: ["true"], readAction: "", risk: ["time-jump"] }),
  set("platform", "unreadable", { desired: ["y"], readAction: "", note: "bu sunucuda okunamıyor" }),
  set("zone", "match", { card: "sistem", group: "Zaman", desired: ["Europe/Istanbul"], actual: ["Europe_Istanbul"], readAction: "durum", applyVia: ["apply"] }),
  set("dil", "unmanaged", { card: "sistem", group: "Dil", actual: ["unset"], readAction: "durum", applyVia: ["apply"] }),
];
const CARDS = [{ id: "saat", label: "Saat (NTP)", description: "chrony ile saat eşitleme" }, { id: "sistem", label: "Sistem", description: "saat dilimi ve dil" }];
const V = (name, extra) => ({ name, type: "string", default: "", value: "", set: false, group: "G", description: "", ...extra });
const VARS = () => [
  V("v_kaynak", { type: "list[string]", value: "0.tr.pool.ntp.org", set: true }), V("v_adim", { type: "bool", default: "true" }),
  V("v_platform", { value: "y", set: true }), V("v_zone", { value: "Europe/Istanbul", set: true }), V("v_dil"),
];

// state: testin değiştirebildiği sahte sunucu; posts: /api/vars'a yazılanlar
function world(opts = {}) {
  const w = { settings: opts.settings || JSON.parse(JSON.stringify(SETTINGS)), cards: opts.cards === undefined ? CARDS : opts.cards, vars: opts.vars || VARS(), posts: [], reply: opts.reply, catalog: opts.catalog, f: opts.f };
  const server = u => {
    const d = fixture("server-" + u.searchParams.get("name")); d.server.up = true;
    const p = d.packages.find(x => x.id === "utility"), base = p.actions.find(a => a.id === "durum");
    const mk = (id, extra) => ({ ...JSON.parse(JSON.stringify(base)), id, keys: {}, applicable: true, runnable: false, write: false, terminal: false, destructive: false, effects: [], requires: [], prereqs: [], lastTime: "", lastResult: "", ...extra });
    p.actions = p.actions.filter(a => !["durum", "preview", "apply", "ntp_durum", "ntp_ayarla"].includes(a.id));
    p.actions.push(mk("durum", { runnable: true }), mk("ntp_durum", { runnable: true }), mk("preview", { terminal: true }),
      mk("ntp_ayarla", { write: true, effects: ["changes-config"], requires: ["preview"] }), mk("apply", { effects: ["changes-config"], requires: ["preview"], reason: "önce preview" }));
    if (w.catalog) p.actions.push(mk("kurulum_durum", { runnable: true }));
    p.settings = w.settings; p.settingCards = w.cards || undefined;
    if (w.f) w.f(p);
    return d;
  };
  w.overrides = {
    "/api/home": mixedHome, "/api/server": server,
    "/api/vars": (u, { method, body }) => {
      if (method !== "POST") return { vars: JSON.parse(JSON.stringify(w.vars)) };
      w.posts.push(body);
      const r = w.reply && w.reply(body);
      if (r) return r;
      const v = w.vars.find(x => x.name === body.name); v.set = body.action === "set"; v.value = v.set ? body.value : "";
      return { ok: true };
    },
  };
  return w;
}
const open = async opts => {
  const w = world(opts);
  const h = await boot({ hash: "#s=web1&p=linux-hardening", overrides: w.overrides });
  await h.click('#srv-tabs [data-p="utility"]');
  h.w = w; return h;
};
const txt = el => el.textContent.replace(/\s+/g, " ").trim();
const cardEl = (h, id) => h.$(`#srv-body [data-setcard="${id}"]`);
const openCard = (h, id) => h.click(`#srv-body [data-setopen="${id}"]`);
const modal = h => h.$("#setmodal");
const isOpen = h => modal(h).classList.contains("open");
const field = (h, id) => h.$(`#set-body .set-field[data-set="${id}"]`);
const ctl = (h, name) => h.$(`#set-body [data-var="${name}"]`);
const btns = h => [...h.$$("#set-reads .btn"), ...h.$$("#set-acts .btn")].map(txt);
const btn = (h, text) => [...h.$$("#set-reads .btn"), ...h.$$("#set-acts .btn")].find(b => txt(b) === text);
const edit = async (h, name, value) => { const el = ctl(h, name); if (el.type === "checkbox") el.checked = value; else el.value = value; el.dispatchEvent(new h.win.Event(el.type === "checkbox" ? "change" : "input", { bubbles: true })); await h.settle(); };
const esc = async h => { h.doc.dispatchEvent(new h.win.KeyboardEvent("keydown", { key: "Escape", bubbles: true })); await h.settle(); };

test("kartlar paketin bildirdiği sırayla; rozet durum özeti, ayrıntı kartta yok; sayfa açılınca hiçbir şey çalışmaz", async () => {
  const h = await open();
  assert.deepEqual(h.$$("#srv-body [data-setcard]").map(c => [c.dataset.setcard, txt(c.querySelector(".app-title span")), txt(c.querySelector(".app-title .badge"))]),
    [["saat", "Saat (NTP)", "1 sapma"], ["sistem", "Sistem", "uyumlu"]]);
  assert.match(txt(cardEl(h, "saat")), /chrony ile saat eşitleme.*Son okuma: henüz okunmadı.*Ayarlar/);
  assert.doesNotMatch(txt(cardEl(h, "saat")), /0\.tr\.pool/);
  assert.ok(!isOpen(h));
  assert.equal(h.w.posts.length, 0); assert.equal(h.calls.filter(c => c.method === "POST").length, 0);
  assert.deepEqual(h.errors, []); h.close();
});

test("katalog sayfasında kartlar mağazanın başında; 'Tümü' sayar, kurulu süzgeci gizler; eski 'Saat ve NTP' kartı yok", async () => {
  const h = await open({ catalog: true, f: p => { const k = p.actions.find(a => a.id === "kurulum_durum"); k.keys = { kurulu: "docker:docker", portlar: "yok" }; } });
  const grid = h.$$("#store-grid > .app-card");
  assert.deepEqual(grid.slice(0, 2).map(c => c.dataset.setcard), ["saat", "sistem"]);
  assert.equal(h.$("#srv-body .ntp-card"), null);
  assert.equal(txt(h.$('[data-sf="all"]')), `Tümü ${grid.length}`);
  await h.click('[data-sf="on"]'); assert.equal(cardEl(h, "saat").hidden, true);
  await h.click('[data-sf="all"]'); assert.equal(cardEl(h, "saat").hidden, false);
  h.close();
  const eski = await open({ catalog: true, settings: [] });
  assert.ok(eski.$("#srv-body .ntp-card"), "settings'siz sürümde eski kart"); eski.close();
});

test("kart bildirilmemişse değişken grubu kart olur", async () => {
  const h = await open({ cards: null });
  assert.deepEqual(h.$$("#srv-body [data-setcard]").map(c => [c.dataset.setcard, txt(c.querySelector(".app-title span"))]),
    [["g:NTP", "NTP"], ["g:Zaman", "Zaman"], ["g:Dil", "Dil"]]);
  await openCard(h, "g:Zaman"); assert.equal(h.$("#set-title").textContent, "Zaman"); assert.ok(field(h, "zone")); h.close();
});

test("pencere: istenen değer kutuda, sunucudaki değer ve durum altında; seçenekler ayrı bölümde", async () => {
  const h = await open();
  await openCard(h, "saat");
  assert.ok(isOpen(h)); assert.equal(h.$("#set-title").textContent, "Saat (NTP)");
  assert.match(txt(h.$("#set-sub")), /Sunucu: web1 · Son okuma: henüz okunmadı/);
  assert.equal(h.calls.filter(c => c.path === "/api/vars?server=web1&package=utility").length >= 1, true);
  assert.equal(ctl(h, "v_kaynak").value, "0.tr.pool.ntp.org");
  assert.equal(ctl(h, "v_adim").type, "checkbox"); assert.equal(ctl(h, "v_adim").checked, true, "ayarsız: varsayılan");
  const k = field(h, "kaynak");
  assert.equal(txt(k.querySelector('[data-live="state"]')), "⚠ sapma");
  assert.match(txt(k.querySelector('[data-live="have"]')), /Sunucuda:.*eski\.example\.org/);
  assert.match(txt(k.querySelector("details")), /uzun açıklama.*risk notu/);
  assert.deepEqual([...k.querySelectorAll(".set-fhead .badge")].map(txt), ["saati sıçratır", "⚠ sapma"], "sözlük dışı risk gösterilmez");
  assert.match(txt(field(h, "platform")), /okunamaz.*bu sunucuda okunamıyor/);
  const opts = h.$("#set-body .set-optbox");
  assert.deepEqual([...opts.querySelectorAll(".set-opt")].map(o => o.dataset.set), ["adim"]); assert.doesNotMatch(txt(opts), /okunamaz/);
  assert.deepEqual(btns(h), ["Kontrol et", "Kaydet", "Önizle", "Uygula"]);
  assert.ok(btn(h, "Kaydet").disabled);
  assert.equal(btn(h, "Önizle").dataset.go, "term"); assert.equal(btn(h, "Uygula").dataset.action, "ntp_ayarla");
  await h.click("#set-x"); assert.ok(!isOpen(h));
  await openCard(h, "sistem");
  assert.equal(txt(field(h, "zone").querySelector('[data-live="have"] .chip')), "Europe/Istanbul", "uyumluysa istenen yazım");
  assert.ok(btn(h, "Uygula").disabled); assert.equal(btn(h, "Uygula").title, "önce preview");
  assert.deepEqual(h.errors, []); h.close();
});

test("değişiklik: Kaydet açılır, Önizle ve Uygula kaydedene kadar kapalı; sayfa tazelenince yazılan silinmez, durum yenilenir", async () => {
  const h = await open();
  await openCard(h, "saat");
  await edit(h, "v_kaynak", "yeni.example.org");
  assert.ok(!btn(h, "Kaydet").disabled);
  for (const t of ["Önizle", "Uygula"]) { assert.ok(btn(h, t).disabled, t); assert.equal(btn(h, t).title, "önce kaydedin"); }
  assert.ok(!btn(h, "Kontrol et").disabled);
  h.w.settings[0].status = "match"; h.w.settings[0].desired = ["a"]; h.w.settings[0].actual = ["a"];
  await h.click('#srv-tabs [data-p="linux-hardening"]'); await h.click('#srv-tabs [data-p="utility"]');
  h.win.dispatchEvent(new h.win.CustomEvent("keylab:taskexit", { detail: { server: "web1", pkg: "utility", action: "preview", code: 0 } })); await h.settle();
  assert.equal(ctl(h, "v_kaynak").value, "yeni.example.org", "yazılan değer korunur");
  assert.equal(txt(field(h, "kaynak").querySelector('[data-live="state"]')), "✔ uyumlu", "sunucu tarafı yenilenir");
  await edit(h, "v_kaynak", "0.tr.pool.ntp.org");
  assert.ok(btn(h, "Kaydet").disabled, "eski değere dönünce değişiklik yok"); assert.ok(!btn(h, "Uygula").disabled);
  h.close();
});

test("düğmeler yerinde kalır: kutudan çıkışta gelen 'change' ve sayfa tazelemesi tıklanmakta olan düğmeyi sökmez", async () => {
  const h = await open();
  await openCard(h, "saat");
  await edit(h, "v_kaynak", "yeni.example.org");
  const save = btn(h, "Kaydet"), check = btn(h, "Kontrol et");
  ctl(h, "v_kaynak").dispatchEvent(new h.win.Event("change", { bubbles: true })); await h.settle();
  h.win.dispatchEvent(new h.win.CustomEvent("keylab:taskexit", { detail: { server: "web1", pkg: "utility", action: "preview", code: 0 } })); await h.settle();
  assert.equal(btn(h, "Kaydet"), save); assert.equal(btn(h, "Kontrol et"), check);
  await h.click(save); assert.equal(h.w.posts.length, 1); h.close();
});

test("kaydet: yalnız değişenler yazılır; boş bırakılan satır silinir; pencere açık kalır ve sunucuya eylem başlatılmaz", async () => {
  const h = await open();
  await openCard(h, "saat");
  await edit(h, "v_kaynak", "  yeni.example.org  "); await edit(h, "v_adim", false);
  await h.click(btn(h, "Kaydet"));
  assert.deepEqual(h.w.posts.map(p => [p.server, p.package, p.name, p.value, p.action, p.ack]),
    [["web1", "utility", "v_kaynak", "yeni.example.org", "set", false], ["web1", "utility", "v_adim", "false", "set", false]]);
  assert.ok(isOpen(h)); assert.match(txt(h.$("#set-msg")), /Kaydedildi/); assert.ok(btn(h, "Kaydet").disabled);
  assert.equal(ctl(h, "v_kaynak").value, "yeni.example.org");
  await edit(h, "v_kaynak", ""); await h.click(btn(h, "Kaydet"));
  assert.deepEqual(h.w.posts.at(-1).action, "unset"); assert.equal(h.w.posts.length, 3);
  assert.equal(h.calls.filter(c => c.method === "POST" && c.path !== "/api/vars").length, 0);
  assert.deepEqual(h.errors, []); h.close();
});

test("kaydet: uyarı eşiği ikinci tıklamayla onaylanır", async () => {
  const h = await open({ reply: b => (b.name === "v_kaynak" && !b.ack ? { status: 409, body: { error: "riskli değer.", needAck: true } } : null) });
  await openCard(h, "saat");
  await edit(h, "v_kaynak", "x.example.org"); await h.click(btn(h, "Kaydet"));
  assert.match(txt(h.$("#set-msg")), /riskli değer\. Yine de kaydetmek için tekrar tıklayın/);
  assert.ok(btn(h, "Onayla ve kaydet")); assert.equal(h.w.posts.length, 1);
  await h.click(btn(h, "Onayla ve kaydet"));
  assert.equal(h.w.posts[1].ack, true); assert.match(txt(h.$("#set-msg")), /Kaydedildi/); h.close();
});

test("kaydet: onay isteyen değer değiştirilirse onay sıfırlanır", async () => {
  const h = await open({ reply: b => (!b.ack ? { status: 409, body: { error: "riskli", needAck: true } } : null) });
  await openCard(h, "saat");
  await edit(h, "v_kaynak", "a"); await h.click(btn(h, "Kaydet")); assert.ok(btn(h, "Onayla ve kaydet"));
  await edit(h, "v_kaynak", "b"); assert.ok(btn(h, "Kaydet")); await h.click(btn(h, "Kaydet"));
  assert.deepEqual(h.w.posts.map(p => [p.value, p.ack]), [["a", false], ["b", false]]); h.close();
});

test("kaydet: hata gösterilir, değişiklik korunur", async () => {
  const h = await open({ reply: () => ({ status: 400, body: { error: "geçersiz değer" } }) });
  await openCard(h, "saat");
  await edit(h, "v_kaynak", "bad"); await h.click(btn(h, "Kaydet"));
  assert.match(txt(h.$("#set-msg")), /v_kaynak: geçersiz değer/); assert.equal(ctl(h, "v_kaynak").value, "bad"); assert.ok(!btn(h, "Kaydet").disabled); assert.ok(btn(h, "Uygula").disabled);
  h.close();
});

test("kaydedilmemiş değişiklikle kapatma ikinci tıklama ister; Esc de aynı", async () => {
  const h = await open();
  await openCard(h, "saat"); await esc(h); assert.ok(!isOpen(h), "temizken Esc kapatır");
  await openCard(h, "saat"); await edit(h, "v_kaynak", "z");
  await h.click("#set-x"); assert.ok(isOpen(h)); assert.match(txt(h.$("#set-msg")), /Kaydedilmemiş değişiklik/);
  await esc(h); assert.ok(!isOpen(h));
  await openCard(h, "saat"); assert.equal(ctl(h, "v_kaynak").value, "0.tr.pool.ntp.org", "atılan değişiklik geri gelmez"); assert.equal(h.w.posts.length, 0); h.close();
});

test("eylem düğmesi mevcut onay penceresini açar; o sırada ayar penceresi gizlenir, kapanınca geri gelir; Esc yalnız üsttekini kapatır", async () => {
  const h = await open();
  await openCard(h, "saat");
  await h.click(btn(h, "Kontrol et"));
  assert.ok(h.$("#runmodal").classList.contains("open")); assert.match(txt(h.$("#run-title")), /ntp_durum/);
  assert.ok(modal(h).classList.contains("behind")); assert.ok(isOpen(h));
  await esc(h);
  assert.ok(!h.$("#runmodal").classList.contains("open")); assert.ok(isOpen(h), "ayar penceresi kalır"); assert.ok(!modal(h).classList.contains("behind"));
  assert.equal(h.calls.filter(c => c.method === "POST").length, 0, "onaysız hiçbir şey çalışmaz");
  await h.click(btn(h, "Önizle")); assert.match(txt(h.$("#run-title")), /preview/); await h.click("#run-close");
  await h.click(btn(h, "Uygula")); assert.match(txt(h.$("#run-title")), /ntp_ayarla.*yazar/); assert.ok(h.$("#w-snap"), "snapshot onayı aynen"); assert.ok(h.$("#run-go").disabled);
  h.close();
});

test("üstte ortak iletişim penceresi (ör. Python denetimi) varken Esc yalnız onu kapatır", async () => {
  const h = await open();
  await openCard(h, "saat");
  h.$("#dlg").classList.add("open"); await h.settle();
  assert.ok(modal(h).classList.contains("behind"));
  await esc(h);
  assert.ok(!h.$("#dlg").classList.contains("open")); assert.ok(isOpen(h)); assert.ok(!modal(h).classList.contains("behind"));
  await esc(h); assert.ok(!isOpen(h)); h.close();
});

test("yönetilmeyen kartta uygulama düğmesi yok; birden çok uygulama eylemi adıyla; eskimiş okuma kartta uyarılır", async () => {
  const h = await open({ settings: [set("kaynak", "unmanaged", { readAction: "ntp_durum" })] });
  await openCard(h, "saat"); assert.deepEqual(btns(h), ["Kontrol et", "Kaydet"]); h.close();
  const iki = await open({ settings: [set("kaynak", "match", { desired: ["a"], actual: ["a"], stale: true }), set("adim", "drift", { desired: ["2"], applyVia: ["apply"] })] });
  assert.deepEqual([...cardEl(iki, "saat").querySelectorAll(".app-title .badge")].map(txt), ["1 sapma", "güncel olmayabilir"]);
  await openCard(iki, "saat"); assert.deepEqual(btns(iki), ["Kontrol et", "Kaydet", "Önizle", "Uygula: ntp_ayarla", "Uygula: apply"]); iki.close();
});

const H = (label, value, state, unit) => ({ label, value, state, unit });
const healthCards = (extra, items) => [{ ...CARDS[0], readAction: "ntp_durum", health: items, ...extra }, CARDS[1]];

test("sağlık özeti kartta ve pencerede: sözcükler okunur yazılır, birim eklenir, sorun varsa kart yalnız yeşil görünmez", async () => {
  const iyi = [set("kaynak", "match", { desired: ["a"], actual: ["a"] })].concat(SETTINGS.slice(3));
  const h = await open({ settings: iyi, cards: healthCards({}, [H("Senkron", "no", "warn"), H("Servis", "active", "ok"), H("Ofset", "-0.5", "ok", "ms"), H("Kaynak", "unknown", "", "ms")]) });
  const card = cardEl(h, "saat");
  assert.deepEqual([...card.querySelectorAll(".app-title .badge")].map(txt), ["uyumlu", "⚠ dikkat"]);
  assert.deepEqual([...card.querySelectorAll(".set-hitem")].map(i => [txt(i), i.dataset.st]),
    [["Senkron hayır", "warn"], ["Servis çalışıyor", "ok"], ["Ofset -0.5 ms", "ok"], ["Kaynak bilinmiyor", ""]]);
  assert.equal(cardEl(h, "sistem").querySelector(".set-health"), null, "özet bildirmeyen kartta yok");
  await openCard(h, "saat");
  assert.equal(h.$("#set-body").firstElementChild.id, "set-health", "pencerenin başında");
  assert.equal(h.$$("#set-health .set-hitem").length, 4);
  h.w.cards[0].health = [H("Senkron", "yes", "ok")];
  h.win.dispatchEvent(new h.win.CustomEvent("keylab:taskexit", { detail: { server: "web1", pkg: "utility", action: "ntp_durum", code: 0 } })); await h.settle();
  assert.deepEqual(h.$$("#set-health .set-hitem").map(txt), ["Senkron evet"], "okuma sonrası tazelenir");
  assert.deepEqual([...cardEl(h, "saat").querySelectorAll(".app-title .badge")].map(txt), ["uyumlu"]);
  assert.deepEqual(h.errors, []); h.close();
});

test("sağlık yoksa nedeni yazılır; özetin okuma eylemi 'Kontrol et' ile çalıştırılabilir; eskimiş özet uyarılır", async () => {
  const only = [set("adim", "unreadable", { desired: ["true"], readAction: "" })].concat(SETTINGS.slice(3));
  const h = await open({ settings: only, cards: healthCards({ note: "son okuma başarısız", stale: true }, []) });
  assert.match(txt(cardEl(h, "saat")), /Sağlık bilgisi: son okuma başarısız.*Son okuma: henüz okunmadı/);
  assert.deepEqual([...cardEl(h, "saat").querySelectorAll(".app-title .badge")].map(txt), ["güncel olmayabilir"]);
  await openCard(h, "saat");
  assert.match(txt(h.$("#set-health")), /son okuma başarısız/);
  assert.equal(btn(h, "Kontrol et").dataset.action, "ntp_durum"); h.close();
});

test("sağlık metni HTML olarak yorumlanmaz", async () => {
  const bad = '<img src=x onerror="window.__x=1">';
  const h = await open({ cards: healthCards({}, [H(bad, bad, bad, bad)]) });
  await openCard(h, "saat");
  assert.equal(h.$("#srv-body img"), null); assert.equal(h.$("#setmodal img"), null); h.close();
  const n = await open({ cards: healthCards({ note: bad }, []) });
  assert.equal(n.$("#srv-body img"), null); assert.equal(n.win.__x, undefined); n.close();
});

test("uygulama ile aynı kimlikli ayar kartı katalogdaki kartın yerine geçer: tek kart, kurulu bilgisi ve süzgeç korunur", async () => {
  const cards = [{ id: "nginx", label: "nginx", description: "web sunucusu" }];
  const rows = [set("port", "match", { card: "nginx", desired: ["8080"], actual: ["8080"] })];
  const katalog = kurulu => p => { p.actions.find(a => a.id === "kurulum_durum").keys = kurulu; };
  const h = await open({ catalog: true, cards, settings: rows, f: katalog({ kurulu: "nginx:docker", portlar: "nginx:8080" }) });
  assert.equal(h.$('#store-grid [data-app="nginx"]'), null, "eski nginx kartı çizilmez");
  assert.deepEqual(h.$$("#store-grid > .app-card").map(c => c.dataset.setcard || c.dataset.app), ["nginx", "docker", "mariadb", "redis"]);
  assert.deepEqual([...cardEl(h, "nginx").querySelectorAll(".app-title .badge")].map(txt), ["✔ Kurulu (docker)", "uyumlu"]);
  assert.equal(txt(h.$('[data-sf="all"]')), "Tümü 4"); assert.equal(txt(h.$('[data-sf="on"]')), "Kurulu 1"); assert.equal(txt(h.$('[data-sf="off"]')), "Kurulabilir 3");
  await h.click('[data-sf="on"]'); assert.equal(cardEl(h, "nginx").hidden, false);
  await h.click('[data-sf="off"]'); assert.equal(cardEl(h, "nginx").hidden, true);
  assert.deepEqual(h.errors, []); h.close();
  const yok = await open({ catalog: true, cards, settings: rows, f: katalog({ kurulu: "yok", portlar: "yok" }) });
  assert.match(txt(cardEl(yok, "nginx")), /Kurulmadı/); assert.equal(cardEl(yok, "nginx").dataset.st, "off"); yok.close();
  const okunmadi = await open({ catalog: true, cards, settings: rows, f: katalog({}) });
  assert.match(txt(cardEl(okunmadi, "nginx")), /durum okunmadı/); okunmadi.close();
});

test("sunucuda yok: ayrı durum, sapma sayılmaz; kurmak için uygulama düğmesi açık kalır", async () => {
  const h = await open({ settings: [set("kaynak", "absent", { desired: ["8080"] })], cards: [CARDS[0]] });
  assert.deepEqual([...cardEl(h, "saat").querySelectorAll(".app-title .badge")].map(txt), ["sunucuda yok"]);
  await openCard(h, "saat");
  assert.equal(txt(field(h, "kaynak").querySelector('[data-live="state"]')), "sunucuda yok");
  assert.match(txt(field(h, "kaynak").querySelector('[data-live="have"]')), /Sunucuda:\s*—/);
  assert.deepEqual(btns(h), ["Kontrol et", "Kaydet", "Önizle", "Uygula"]); h.close();
});

test("paket metni HTML olarak yorumlanmaz", async () => {
  const bad = '<img src=x onerror="window.__x=1">';
  const h = await open({
    cards: [{ id: "saat", label: bad, description: bad }],
    settings: [set("kaynak", "drift", { label: bad, group: bad, description: bad, desired: [bad], actual: [bad], note: bad, riskNote: bad, risk: ["data-loss"] }),
      set("adim", "unreadable", { label: bad, description: bad, desired: [bad], readAction: "" })],
    vars: [V("v_kaynak", { value: bad, set: true, default: bad }), V("v_adim", { type: "enum", values: [bad], default: bad })],
  });
  assert.equal(h.$("#srv-body img"), null);
  await openCard(h, "saat");
  assert.equal(h.$("#setmodal img"), null); assert.equal(ctl(h, "v_kaynak").value, bad); assert.equal(h.win.__x, undefined); h.close();
});

test("settings yoksa kart yok", async () => {
  const h = await open({ settings: [] });
  assert.equal(h.$("#srv-body [data-setcard]"), null); h.close();
});

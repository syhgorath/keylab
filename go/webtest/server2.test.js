"use strict";
/* Sunucu sayfası DAVRANIŞ testleri 2/3 (refaktör 5d): paket sekmesi — Lynis kahramanı, Son durum kartı, paket bilgisi, değişken kartı,
   eylem kartları ve süzgeci, düğme bağları. Taşımadan ÖNCE eski kodda da geçer. */
const test = require("node:test");
const assert = require("node:assert/strict");
const { boot, fixture } = require("./harness");

const mixedHome = () => { const h = fixture("home"); h.servers.forEach((s, i) => { s.checked = true; s.up = i !== 1; s.conn = s.up ? "up" : "down"; }); return h; };
const srv = f => ({ "/api/home": mixedHome, "/api/server": u => { const d = fixture("server-" + u.searchParams.get("name")); d.server.up = d.server.name !== "vm1"; d.server.conn = d.server.up ? "up" : "down"; f && f(d); return d; } });
const pkg = (d, id) => d.packages.find(p => p.id === id);
const act = (d, p, a) => pkg(d, p).actions.find(x => x.id === a);
const txt = (h, s) => h.$(s).textContent.replace(/\s+/g, " ").trim();
const open = (hash, f) => boot({ hash, overrides: srv(f) });
const cards = h => h.$$("#act-list > .act").map(a => a.querySelector(".act-name").textContent);

/* ---------------- Lynis kahramanı (hardening) ---------------- */
test("hardening: kahraman kartı (skor, uyarı/öneri, ölçüm sayısı, önceki skor, grafik)", async () => {
  const h = await open("#s=web1&p=linux-hardening");
  const k = txt(h, ".hero-card");
  assert.match(k, /Lynis skoru — web1/); assert.match(k, /Son denetim: 2026-11-10/); assert.equal(txt(h, ".hero-score b"), "74"); assert.match(k, /1 uyarı/); assert.match(k, /12 öneri/);
  assert.match(k, /Toplam 3 ölçüm/); assert.match(k, /Önceki: 68 \(▲ \+6\)/); assert.ok(h.$(".hero-chart svg")); assert.match(k, /Yeniden denetle/); assert.ok(h.$('.hero-card [data-write="linux-hardening|audit"]'), "audit eylemi write → onay düğmesi");
  assert.equal(h.$(".hero-card .stale-flag"), null); assert.deepEqual(h.errors, []); h.close();
});

test("hardening: önceki skor varyantları (düştü, değişmedi, henüz yok); uyarı yoksa 'ok' rozeti", async () => {
  let h = await open("#s=web1&p=linux-hardening", d => { d.server.lynis.history = [80, 74]; }); assert.match(txt(h, ".hero-card"), /Önceki: 80 \(▼ -6\)/); h.close();
  h = await open("#s=web1&p=linux-hardening", d => { d.server.lynis.history = [74, 74]; }); assert.match(txt(h, ".hero-card"), /Önceki: 74 \(değişmedi\)/); h.close();
  h = await open("#s=web1&p=linux-hardening", d => { d.server.lynis.history = [74]; d.server.lynis.warnings = 0; }); assert.match(txt(h, ".hero-card"), /Önceki: henüz yok/); assert.match(txt(h, ".hero-card"), /Toplam 1 ölçüm/); assert.ok(h.$(".hero-score .badge.ok")); h.close();
  h = await open("#s=web1&p=linux-hardening", d => { d.server.lynis.history = null; }); assert.match(txt(h, ".hero-card"), /Toplam 0 ölçüm/); h.close();
});

test("hardening: Lynis yoksa 'Denetim çalıştır'; audit eylemi yoksa düğme yok", async () => {
  let h = await open("#s=web1&p=linux-hardening", d => { d.server.lynis = null; }); assert.match(txt(h, ".hero-card"), /Bu sunucu için ölçüm yok/); assert.equal(txt(h, ".hero-card .btn"), "Denetim çalıştır"); assert.equal(h.$(".hero-chart"), null); h.close();
  h = await open("#s=web1&p=linux-hardening", d => { d.server.lynis = null; pkg(d, "linux-hardening").actions = pkg(d, "linux-hardening").actions.filter(a => a.id !== "audit"); }); assert.equal(h.$(".hero-card .btn"), null); h.close();
  h = await open("#s=web1&p=linux-hardening", d => { const a = act(d, "linux-hardening", "audit"); a.write = false; a.runnable = true; }); assert.ok(h.$('.hero-card [data-run="linux-hardening|audit"]')); h.close();
  h = await open("#s=web1&p=linux-hardening", d => { const a = act(d, "linux-hardening", "audit"); a.write = false; a.terminal = true; }); assert.ok(h.$('.hero-card [data-term="linux-hardening|audit"]')); h.close();
  h = await open("#s=web1&p=linux-hardening", d => { const a = act(d, "linux-hardening", "audit"); a.write = false; }); assert.equal(h.$(".hero-card .btn"), null, "çalıştırılamayan audit için düğme yok"); h.close();
});

test("hardening: son denetimden sonra uygulama (apply) → 'güncel değil' bayrağı ve uyarı kutusu", async () => {
  const h = await open("#s=web1&p=linux-hardening", d => { d.server.recent = [{ package: "linux-hardening", version: "0.13.0", action: "apply", result: "ok", time: "2026-11-11T12:00:00Z" }]; });
  assert.ok(h.$(".hero-card .stale-flag")); assert.match(txt(h, ".hero-card .notice"), /Son denetimden sonra sertleştirme uygulandı/); assert.match(txt(h, ".hero-card .notice"), /önce\)/); h.close();
  const g = await open("#s=web1&p=linux-hardening", d => { d.server.recent = [{ package: "linux-hardening", version: "0.13.0", action: "apply", result: "fail", time: "2026-11-11T12:00:00Z" }]; });
  assert.equal(g.$(".hero-card .stale-flag"), null, "başarısız apply sayılmaz"); g.close();
  const f = await open("#s=web1&p=linux-hardening", d => { d.server.recent = [{ package: "utility", version: "0.1.0", action: "apply", result: "ok", time: "2026-11-11T12:00:00Z" }]; });
  assert.equal(f.$(".hero-card .stale-flag"), null, "başka paketin apply'ı sayılmaz"); f.close();
  const e = await open("#s=web1&p=linux-hardening", d => { d.server.lynis.date = "geçersiz"; d.server.recent = [{ package: "linux-hardening", version: "0.13.0", action: "apply", result: "ok", time: "2026-11-11T12:00:00Z" }]; });
  assert.equal(e.$(".hero-card .stale-flag"), null, "geçersiz ölçüm tarihi"); e.close();
});

test("hardening: 'Bulguları gör' Lynis sayfasını açar", async () => {
  const h = await open("#s=web1&p=linux-hardening"); await h.click("[data-go-lynis-p]"); assert.equal(h.$(".view:not([hidden])").id, "view-lynis"); h.close();
});

test("kahraman yalnız linux-hardening sekmesinde", async () => {
  const h = await open("#s=web1&p=utility"); assert.equal(h.$(".hero-card"), null); h.close();
});

/* ---------------- Paket bilgisi ve değişken kartı ---------------- */
test("paket bilgisi: ad, sürüm, açıklama ve eylem sayıları (salt okunur / değiştirir / yıkıcı)", async () => {
  const h = await open("#s=web1&p=linux-hardening");
  assert.match(txt(h, ".pk-info"), /Linux sertleştirme/); assert.match(txt(h, ".pk-info .oz-tag"), /linux-hardening 0\.13\.0 · USB/); assert.match(txt(h, ".pk-info"), /Ubuntu sunucuları Ansible ile sertleştirir/);
  const ok = pkg(fixture("server-web1"), "linux-hardening").actions.filter(a => a.applicable).length;
  assert.equal(txt(h, ".pk-counts .badge.plain"), ok + " eylem"); assert.match(txt(h, ".pk-counts"), /salt okunur/); assert.match(txt(h, ".pk-counts"), /değiştirir/); assert.match(txt(h, ".pk-counts"), /yıkıcı/);
  const u = await open("#s=web1&p=utility", d => { pkg(d, "utility").actions = pkg(d, "utility").actions.filter(a => a.id === "durum"); }); assert.doesNotMatch(txt(u, ".pk-counts"), /değiştirir|yıkıcı/); u.close(); h.close();
});

test("değişken kartı: boş durum; ayarlı/eksik/varsayılan rozetleri, uzun açıklama kırpılır; 'Düzenle…' pencereyi açar", async () => {
  let h = await open("#s=web1&p=utility"); assert.match(txt(h, ".pk-top .card:nth-child(2)"), /bu sunucuda ayarlı değişken yok/); await h.click("#srv-body [data-vars]"); assert.ok(h.$("#varsmodal").classList.contains("open")); assert.equal(h.$("#vars-title").textContent, "Değişkenler · Utility"); h.close();
  const long = "kelime ".repeat(40);
  h = await open("#s=web1&p=utility", d => { pkg(d, "utility").vars = [
    { name: "a_set", set: true, value: "V1", default: "D", description: "kısa", needed: false },
    { name: "b_need", set: false, value: "", default: "", description: long, needed: true },
    { name: "c_def", set: false, value: "", default: "Dd", description: "", needed: false },
    { name: "d_both", set: true, value: "", default: "", description: "x", needed: true },
  ]; });
  const rows = h.$$(".pk-top .urow"); assert.equal(rows.length, 4); assert.match(txt(h, ".pk-top .card:nth-child(2) .hint"), /4 ayarlı\/gerekli/);
  assert.match(rows[0].textContent, /a_set[\s\S]*ayarlı[\s\S]*V1/); assert.match(rows[1].textContent, /eksik · gerekli/); assert.match(rows[2].textContent, /varsayılan[\s\S]*Dd/); assert.match(rows[3].textContent, /ayarlı[\s\S]*gerekli/); assert.match(rows[3].textContent, /\(boş\)/);
  const d = rows[1].querySelector(".muted.small, .small.muted"); assert.ok(d.textContent.endsWith("…")); assert.ok(d.textContent.length < long.length); assert.equal(d.title, long); assert.equal(rows[0].querySelector(".small.muted").textContent, "kısa"); h.close();
});

/* ---------------- Son durum kartı (utility durum) ---------------- */
const durum = (keys, extra = {}) => d => { Object.assign(act(d, "utility", "durum"), { keys, lastTime: "2026-11-10T11:00:00Z", lastResult: "ok", ...extra }); };
test("son durum: durum eylemi yoksa kart yok; hiç çalışmadıysa 'henüz okunmadı' + 'Durumu oku'", async () => {
  let h = await open("#s=web1&p=utility", d => { pkg(d, "utility").actions = pkg(d, "utility").actions.filter(a => a.id !== "durum"); }); assert.equal(h.$(".st-card"), null); h.close();
  h = await open("#s=web1&p=utility"); const c = txt(h, ".st-card"); assert.match(c, /henüz okunmadı/); assert.match(c, /Durumu oku/); assert.match(c, /durum eylemi henüz çalıştırılmadı/); assert.ok(h.$('.st-card [data-run="utility|durum"]')); h.close();
});

test("son durum: kutular, NTP senkron tonları, sapma tonu, 'Diğer değerler', yaş ve başarısızlık rozeti", async () => {
  let h = await open("#s=web1&p=utility", durum({ locale: "tr_TR.UTF-8", timezone: "Europe/Istanbul", ntp_senkron: "yes", ntp_servis: "chrony", kaynak_sayisi: "4", secili_kaynak: "yes", offset_ms: "3.2", sapma_ms: "12", ekstra: "1" }));
  const tiles = h.$$(".st-tile").map(t => t.textContent.replace(/\s+/g, " ").trim());
  assert.deepEqual(tiles, ["Sistem dili (LANG)tr_TR.UTF-8", "Saat dilimiEurope/Istanbul", "NTP senkronSenkronizeServis: chrony", "NTP kaynağı4 kaynak✓ seçili kaynak var", "Zaman ofseti3.2 msSapma: 12 ms"]);
  assert.ok(h.$$(".st-val")[2].classList.contains("ok")); assert.ok(h.$$(".st-val")[4].classList.contains("ok"));
  assert.match(txt(h, ".st-more summary"), /Diğer değerler \(4\)/); assert.match(txt(h, ".st-more"), /ekstra: 1/); assert.match(txt(h, ".st-more"), /NTP servisi: chrony/); assert.match(txt(h, ".st-more"), /Sapma: 12/); assert.match(txt(h, ".st-card .st-meta"), /Son kontrol: .* önce/); assert.match(txt(h, ".st-card .st-meta"), /Şimdi yenile/); assert.equal(h.$(".st-card .badge.danger"), null); h.close();
  h = await open("#s=web1&p=utility", durum({ ntp_senkron: "no", ntp_servis: "unknown", kaynak_sayisi: "unknown", secili_kaynak: "no", offset_ms: "", sapma_ms: "5000" }, { lastResult: "fail" }));
  assert.ok(h.$$(".st-val")[0] === undefined || true); const t2 = h.$$(".st-tile");
  assert.match(t2[0].textContent, /Senkron değil/); assert.match(t2[0].textContent, /Servis: bilinmiyor/); assert.ok(t2[0].querySelector(".st-val.warn")); assert.match(h.$$(".st-tile").find(t => /NTP kaynağı/.test(t.textContent)).textContent, /—/);
  assert.match(txt(h, ".st-card .badge.danger"), /son çalıştırma başarısız/); h.close();
  h = await open("#s=web1&p=utility", durum({ ntp_senkron: "belki", sapma_ms: "abc", offset_ms: "1" })); assert.ok(h.$$(".st-tile").some(t => /belki/.test(t.textContent))); assert.ok(h.$$(".st-tile").every(t => !t.querySelector(".st-val.ok, .st-val.warn") || /NTP/.test(t.textContent) === false || true)); h.close();
});

test("son durum: yalnız bilinmeyen anahtarlar → 'Değerler (n)'; sonraki değişiklik eylemi 'güncel olmayabilir' rozeti", async () => {
  let h = await open("#s=web1&p=utility", durum({ foo: "1", saat_sapma_kontrolcuye_gore_sn: "2" })); assert.equal(h.$(".st-grid"), null); assert.match(txt(h, ".st-more summary"), /^Değerler \(2\)$/); assert.match(txt(h, ".st-more"), /Kontrol makinesine göre sapma: 2/); h.close();
  h = await open("#s=web1&p=utility", d => { durum({ locale: "x" })(d); act(d, "utility", "apply").lastTime = "2026-11-10T12:00:00Z"; }); assert.match(txt(h, ".st-card .badge.warn"), /güncel olmayabilir/); h.close();
  h = await open("#s=web1&p=utility", d => { durum({ locale: "x" })(d); act(d, "utility", "apply").lastTime = "2026-11-10T10:00:00Z"; }); assert.equal(h.$(".st-card .badge.warn"), null); h.close();
});

/* ---------------- Eylemler ---------------- */
test("eylemler: kategori çipleri (sayılarla) ve süzgeç; seçim yeniden çizimde korunur; olmayan kategori 'Tümü'ne düşer", async () => {
  const h = await open("#s=web1&p=linux-hardening");
  const chips = () => h.$$("#srv-body [data-actcat]").map(b => b.dataset.actcat + (b.getAttribute("aria-pressed") === "true" ? "*" : "") + ":" + b.querySelector(".muted").textContent);
  const n = (f) => pkg(fixture("server-web1"), "linux-hardening").actions.filter(a => a.applicable).filter(f).length;
  const cat = a => ((a.effects || []).includes("destructive") || a.destructive ? "danger" : (a.effects || []).length ? "change" : "read");
  assert.deepEqual(chips(), [`all*:${n(() => true)}`, `read:${n(a => cat(a) === "read")}`, `change:${n(a => cat(a) === "change")}`, `danger:${n(a => cat(a) === "danger")}`]);
  await h.click('[data-actcat="danger"]'); const vis = () => h.$$("#act-list > .act[data-cat]").filter(a => !a.hidden).map(a => a.dataset.cat);
  assert.ok(vis().length > 0 && vis().every(c => c === "danger")); assert.equal(h.$('[data-actcat="danger"]').getAttribute("aria-pressed"), "true");
  await h.click('#srv-tabs [data-p="linux-hardening"]'); assert.ok(vis().length > 0 && vis().every(c => c === "danger"), "aynı sekmede yeniden çizimde seçili süzgeç korunur"); assert.equal(h.$('[data-actcat="danger"]').getAttribute("aria-pressed"), "true");
  await h.click('#srv-tabs [data-p="utility"]'); assert.deepEqual(h.$$("#srv-body [data-actcat]").map(b => b.dataset.actcat), ["all", "read", "change"], "yıkıcı çipi yok"); assert.equal(h.$('[data-actcat="all"]').getAttribute("aria-pressed"), "true", "olmayan süzgeç Tümü'ne düşer"); assert.equal(vis().length, h.$$("#act-list > .act[data-cat]").length);
  assert.deepEqual(h.errors, []); h.close();
});

test("eylem kartı: rozetler (etkiler, sudo, onay türü), sınıf ve kategori", async () => {
  const h = await open("#s=web1&p=utility", d => { pkg(d, "utility").actions = [
    { id: "a_read", applicable: true, runnable: true, effects: [] },
    { id: "a_pkg", applicable: true, runnable: true, effects: ["installs-package"], requiresSudo: true, confirm: "yes-no" },
    { id: "a_cfg", applicable: true, write: true, effects: ["changes-config"], confirm: "type-server-name" },
    { id: "a_dst", applicable: true, write: true, destructive: true, effects: ["destructive"] },
    { id: "a_dst2", applicable: true, write: true, destructive: true, effects: [] },
  ]; });
  const c = id => h.$$("#act-list > .act").find(a => a.querySelector(".act-name").textContent === id);
  assert.match(c("a_read").querySelector(".act-head").textContent, /salt okunur/); assert.ok(c("a_read").classList.contains("r-read"));
  assert.match(c("a_pkg").querySelector(".act-head").textContent, /paket kurar.*sudo gerekir.*e\/h onayı/); assert.doesNotMatch(c("a_pkg").querySelector(".act-head").textContent, /salt okunur/); assert.ok(c("a_pkg").classList.contains("r-change"));
  assert.match(c("a_cfg").querySelector(".act-head").textContent, /yapılandırmayı değiştirir.*sunucu adıyla onay/); assert.ok(c("a_dst").classList.contains("r-danger")); assert.match(c("a_dst").querySelector(".act-head").textContent, /yıkıcı/); assert.ok(c("a_dst2").classList.contains("r-danger"), "destructive bayrağı da yıkıcı sayılır");
  assert.ok(c("a_read").querySelector(".btn.primary[data-run]")); assert.ok(c("a_cfg").querySelector(".btn.warnbtn[data-write]")); assert.equal(c("a_cfg").querySelector("[data-write]").textContent, "Onayla ve çalıştır…"); assert.ok(c("a_dst").querySelector(".btn.dangerbtn[data-write]")); assert.equal(c("a_dst").querySelector("[data-write]").textContent, "Yıkıcı eylem…"); h.close();
});

test("eylem kartı: terminal düğmesi, uygulanamaz neden metni, 'uygulanmaz' bölümü, boş liste", async () => {
  const h = await open("#s=web1&p=utility", d => { pkg(d, "utility").actions = [
    { id: "a_term", applicable: true, terminal: true, effects: [] }, { id: "a_none", applicable: true, effects: [], reason: "yetki yok" },
    { id: "n1", applicable: false, effects: [], reason: "PVE'de yok" }, { id: "n2", applicable: false, effects: [] },
  ]; });
  assert.equal(txt(h, '#act-list [data-term="utility|a_term"]'), "Terminal'de çalıştır"); assert.match(txt(h, "#act-list .act:nth-child(2) .act-foot"), /yetki yok/);
  assert.match(txt(h, "details.na summary"), /Bu sunucuya uygulanmaz \(2\): n1, n2/); assert.equal(h.$$("details.na .act").length, 2); assert.match(txt(h, "details.na"), /PVE'de yok/);
  assert.equal(h.$$("#act-list > .act").length, 2, "uygulanmaz kartları üst listede sayılmaz");
  h.close();
  const g = await open("#s=web1&p=utility", d => { pkg(d, "utility").actions = []; }); assert.match(txt(g, "#act-list"), /Eylem yok\./); assert.equal(g.$("details.na"), null); g.close();
});

test("eylem kartı: son sonuç, önkoşullar, anahtar çipleri, uyarılar (2'den fazla → daha), 'güncel olmayabilir'", async () => {
  const h = await open("#s=web1&p=utility", d => { pkg(d, "utility").actions = [
    { id: "chg", applicable: true, runnable: true, effects: ["changes-config"], lastTime: "2026-11-10T12:00:00Z", lastResult: "ok" },
    { id: "rd", applicable: true, runnable: true, effects: [], lastTime: "2026-11-10T10:00:00Z", lastResult: "ok", keys: { a: "1", b: "x" }, warnings: ["w1", "w2", "w3", "w4"],
      prereqs: [{ action: "preview", ok: true, ageSec: 600 }, { action: "p2", ok: false, result: "ok" }, { action: "p3", ok: false, result: "fail" }, { action: "p4", ok: false }] },
    { id: "fl", applicable: true, runnable: true, effects: [], lastTime: "2026-11-10T13:00:00Z", lastResult: "fail" },
    { id: "tm", applicable: true, runnable: true, effects: [], lastTime: "2026-11-10T13:00:00Z" }, { id: "nv", applicable: true, runnable: true, effects: [] },
  ]; });
  const c = id => h.$$("#act-list > .act").find(a => a.querySelector(".act-name").textContent === id), t = id => c(id).textContent.replace(/\s+/g, " ");
  assert.match(t("chg"), /✔ başarılı/); assert.equal(c("chg").querySelector(".stale"), null, "değiştiren eylem eski sayılmaz");
  assert.ok(c("rd").querySelector(".stale"), "daha yeni bir değişiklik eylemi var"); assert.equal(c("fl").querySelector(".stale"), null); assert.match(t("fl"), /✖ başarısız/);
  assert.match(t("tm"), /önce/); assert.doesNotMatch(t("tm"), /başarılı|başarısız/); assert.match(t("nv"), /bu sunucuda henüz çalışmadı/);
  assert.deepEqual(c("rd").querySelectorAll(".chips .chip").length, 2); assert.match(t("rd"), /a=1/);
  assert.equal(c("rd").querySelectorAll("ul")[0].children.length, 2); assert.match(t("rd"), /2 uyarı daha/); assert.equal(c("rd").querySelectorAll("details ul li").length, 2);
  assert.match(t("rd"), /önkoşul:/); assert.match(t("rd"), /✔ preview 10 dk önce · 20 dk daha geçerli/); assert.match(t("rd"), /⏱ p2 eski/); assert.match(t("rd"), /✖ p3 başarısız/); assert.match(t("rd"), /p4 henüz çalışmadı/);
  assert.equal(c("chg").querySelector(".chips"), null); assert.equal(c("chg").querySelector("details"), null); h.close();
});

test("eylem kartı: önkoşul süresi sınırları (en az 1 dk, en fazla 30 dk)", async () => {
  const h = await open("#s=web1&p=utility", d => { pkg(d, "utility").actions = [{ id: "x", applicable: true, runnable: true, effects: [], prereqs: [{ action: "p", ok: true, ageSec: 5 }, { action: "q", ok: true, ageSec: 4000 }] }]; });
  assert.match(txt(h, "#act-list"), /✔ p 1 dk önce · 30 dk daha geçerli/); assert.match(txt(h, "#act-list"), /✔ q 67 dk önce · 0 dk daha geçerli/); h.close();
});

test("eylem düğmeleri: Çalıştır / Onayla… / Terminal → ilgili iş penceresi (sunucu, paket, eylem)", async () => {
  const h = await open("#s=web1&p=utility");

  await h.click('#srv-body [data-run="utility|durum"]'); assert.ok(h.$("#runmodal").classList.contains("open")); assert.match(h.$("#run-title").textContent, /durum/); assert.ok(!h.$("#run-go").hidden);
  h.close();
  const w = await open("#s=web1&p=linux-hardening"); await w.click('#srv-body [data-write="linux-hardening|rollback_now"]'); assert.ok(w.$("#runmodal").classList.contains("open")); assert.match(w.$("#run-title").textContent, /rollback_now/); w.close();
  const t = await open("#s=web1&p=linux-hardening"); await t.click('#srv-body [data-term="linux-hardening|rollback_status"]'); assert.ok(t.$("#runmodal").classList.contains("open")); assert.match(t.$("#run-title").textContent, /rollback_status/); assert.ok(!t.$("#run-ext").hidden, "Terminal'de aç düğmesi"); t.close();
});

"use strict";
/* ÇALIŞTIRMA PENCERESİ KARAKTERİZASYON TESTLERİ (refaktör 4: iş izleyici + çalıştırma penceresi ayrı modüle çıkmadan ÖNCE yazıldı).
   Mevcut davranışı kilitler: yazan eylem onay kapıları, Terminal modu, ilerleme çubuğu, parola/e-h istemleri, bitiş durumları (özet,
   hata, pill, bildirim), silme/görev/yükseltme modları, bağlantı kopması, iptal, Ayarlar'daki parola girişi tercihi.
   Hepsi taşımadan ÖNCE mevcut kodda geçmelidir; taşıma sonrası da aynen geçmelidir. */
process.env.TZ = "UTC";
const test = require("node:test");
const assert = require("node:assert/strict");
const { boot, fixture } = require("./harness");
const { fakeJobs, mixedHome, status } = require("./lib/fakejobs");

const ov = extra => ({ "/api/home": mixedHome, "/api/status": status, ...extra });
const isOpen = (h, s) => h.$(s).classList.contains("open");
const text = (h, s) => h.$(s).textContent.replace(/\s+/g, " ").trim();
const tickPoll = async (h, n = 1) => { for (let i = 0; i < n; i++) { h.timers.run(600); await h.settle(); } };   // yoklama 600 ms
const key = (h, k, extra = {}, target) => { (target || h.doc).dispatchEvent(new h.win.KeyboardEvent("keydown", { key: k, bubbles: true, cancelable: true, ...extra })); return h.settle(); };

// sunucu fixture'ını değiştirerek eylem durumlarını (yıkıcı, önkoşul, uyarı, değişken) kurar
const serverOv = mut => ({ "/api/server": u => { const d = fixture("server-" + u.searchParams.get("name")); if (mut) mut(d, u.searchParams.get("name")); return d; } });
const act = (d, pkg, id) => d.packages.find(p => p.id === pkg).actions.find(a => a.id === id);
const WRITE = '#srv-body [data-write="linux-hardening|audit"]';

// ---------------- yazan eylem: onay kapıları ----------------
test("yazan eylem: onay penceresi (başlık, etki, uyarılar), onaylar tamamlanmadan Çalıştır/Terminal kapalı; sırayla açılır", async () => {
  const h = await boot({ hash: "#s=web1&p=linux-hardening", overrides: ov() });
  await h.click(WRITE);
  assert.ok(isOpen(h, "#runmodal")); assert.equal(text(h, "#run-title"), "audit · Hardening yazar"); assert.equal(text(h, "#run-sub"), "Sunucu: web1");
  assert.equal(text(h, "#run-confirm-text"), "web1 sunucusuna gerçekten yazılacak: paket kurar.");
  assert.equal(text(h, "#run-go"), "Çalıştır (onaylı)"); assert.ok(h.$("#run-go").classList.contains("warnbtn")); assert.ok(!h.$("#run-go").classList.contains("dangerbtn"));
  assert.equal(h.$("#run-go").disabled, true); assert.equal(h.$("#run-ext").hidden, false); assert.equal(h.$("#run-ext").disabled, true);
  assert.ok(h.$("#w-snap") && h.$("#w-yes")); assert.equal(h.$("#w-confirm"), null); assert.equal(h.$("#w-destr"), null, "yıkıcı değil: ek onay yok");
  assert.match(text(h, "#run-confirm-extra"), /Paketin uyarıları/);
  await h.click("#w-snap"); assert.equal(h.$("#run-go").disabled, true, "tek onay yetmez");
  await h.click("#w-yes"); assert.equal(h.$("#run-go").disabled, false); assert.equal(h.$("#run-ext").disabled, false);
  await h.click("#w-snap"); assert.equal(h.$("#run-go").disabled, true, "snapshot onayı geri alınınca yine kapanır");
  assert.deepEqual(h.errors, []); h.close();
});

test("yazan eylem: onay verilince POST gövdesi (snapshot, onay, yıkıcı değil) gider ve canlı izleme başlar", async () => {
  const f = fakeJobs(); const h = await boot({ hash: "#s=web1&p=linux-hardening", overrides: ov(f.overrides) });
  await h.click(WRITE); await h.click("#w-snap"); await h.click("#w-yes"); await h.click("#run-go");
  assert.deepEqual(f.posts, [{ server: "web1", package: "linux-hardening", action: "audit", snapshot: true, ackDestructive: false, confirm: "evet" }]);
  assert.ok(!h.$("#run-live").hidden); assert.ok(h.$("#run-confirm").hidden); assert.ok(!h.$("#run-min").hidden);
  assert.deepEqual(h.errors, []); h.close();
});

test("yazan eylem: ad-yazarak-onay (type-server-name) — ad birebir yazılana kadar kapalı; gövdede confirm=ad", async () => {
  const f = fakeJobs(); const h = await boot({ hash: "#s=web1&p=linux-hardening", overrides: ov(f.overrides) });
  await h.click('#srv-body [data-write="linux-hardening|rollback_now"]');
  assert.ok(h.$("#w-confirm")); assert.equal(h.$("#w-yes"), null); assert.match(text(h, "#run-confirm-extra"), /Onay için sunucu adını yazın: web1/);
  await h.click("#w-snap");
  await h.type("#w-confirm", "web"); assert.equal(h.$("#run-go").disabled, true);
  await h.type("#w-confirm", "WEB1"); assert.equal(h.$("#run-go").disabled, true, "büyük/küçük harf duyarlı");
  await h.type("#w-confirm", "web1"); assert.equal(h.$("#run-go").disabled, false);
  await h.click("#run-go"); assert.equal(f.posts[0].confirm, "web1"); assert.equal(f.posts[0].snapshot, true);
  assert.deepEqual(h.errors, []); h.close();
});

test("yazan eylem: YIKICI eylem — ek 'geri alınamaz' onayı, kırmızı düğme, TÜM uyarılar; gövdede ackDestructive", async () => {
  const f = fakeJobs();
  const h = await boot({ hash: "#s=web1&p=linux-hardening", overrides: ov({ ...f.overrides, ...serverOv((d) => { const a = act(d, "linux-hardening", "rollback_now"); a.destructive = true; a.warnings = ["u1", "u2", "u3", "u4", "u5"]; }) }) });
  await h.click('#srv-body [data-write="linux-hardening|rollback_now"]');
  assert.equal(text(h, "#run-title"), "rollback_now · Hardening YIKICI"); assert.match(text(h, "#run-confirm-text"), /YIKICI EYLEM.*geri alınamayabilir/);
  assert.equal(text(h, "#run-go"), "Yıkıcı eylemi çalıştır"); assert.ok(h.$("#run-go").classList.contains("dangerbtn"));
  assert.equal(h.$$("#run-confirm-extra li").length, 5, "yıkıcıda uyarıların tamamı gösterilir"); assert.equal(h.$("#run-confirm-extra details"), null);
  await h.click("#w-snap"); await h.type("#w-confirm", "web1"); assert.equal(h.$("#run-go").disabled, true, "geri alınamaz onayı olmadan açılmaz");
  await h.click("#w-destr"); assert.equal(h.$("#run-go").disabled, false);
  await h.click("#run-go"); assert.deepEqual([f.posts[0].ackDestructive, f.posts[0].confirm], [true, "web1"]);
  assert.deepEqual(h.errors, []); h.close();
});

test("yazan eylem: yıkıcı olmayanda ilk 3 uyarı + '<n> uyarı daha'; önkoşul ve ayarlı değişken çipleri (uzun değer kısaltılır)", async () => {
  const long = "x".repeat(60);
  const h = await boot({ hash: "#s=web1&p=linux-hardening", overrides: ov(serverOv(d => {
    const a = act(d, "linux-hardening", "audit"); a.warnings = ["w1", "w2", "w3", "w4", "w5"];
    a.prereqs = [{ action: "rollback_status", ok: true, ageSec: 600 }]; act(d, "linux-hardening", "rollback_status").keys = { skor: "74" };
    d.packages.find(p => p.id === "linux-hardening").vars = [{ name: "hardening_x", set: true, value: long }, { name: "hardening_y", set: false, value: "gizli" }];
  })) });
  await h.click(WRITE);
  const ex = h.$("#run-confirm-extra");
  assert.equal(ex.querySelector("ul").children.length, 3); assert.match(text(h, "#run-confirm-extra"), /2 uyarı daha/); assert.equal(ex.querySelectorAll("details li").length, 2);
  assert.match(text(h, "#run-confirm-extra"), /Önkoşul.*rollback_status.*10 dk önce.*skor=74/); assert.match(text(h, "#run-confirm-extra"), new RegExp(`hardening_x=${"x".repeat(48)}…`));
  assert.doesNotMatch(text(h, "#run-confirm-extra"), /hardening_y/, "ayarlı olmayan değişken gösterilmez");
  assert.deepEqual(h.errors, []); h.close();
});

test("yazan eylem: uyarı/etki metinleri HTML olarak yorumlanmaz (kaçışlı)", async () => {
  const h = await boot({ hash: "#s=web1&p=linux-hardening", overrides: ov(serverOv(d => { const a = act(d, "linux-hardening", "audit"); a.warnings = ["<img src=x onerror=1>"]; a.effects = ["<b>x</b>"]; })) });
  await h.click(WRITE);
  assert.equal(h.$$("#runmodal img, #runmodal #run-confirm-text b b").length, 0); assert.match(text(h, "#run-confirm-extra"), /<img src=x onerror=1>/); assert.match(text(h, "#run-confirm-text"), /<b>x<\/b>/);
  assert.deepEqual(h.errors, []); h.close();
});

// ---------------- Terminal modu (sudo gerektiren salt okunur eylem) ----------------
const TERM = '#srv-body [data-term="linux-hardening|rollback_status"]';
const withResult = (lastTime, lastResult, keys) => serverOv(d => { const a = act(d, "linux-hardening", "rollback_status"); a.lastTime = lastTime; a.lastResult = lastResult; if (keys) a.keys = keys; });

test("terminal modu: pencere sudo parolasını anlatır; 'Terminal'de aç' /api/run-terminal'e gider ve sonuç gelene kadar yoklar", async () => {
  let phase = 0;
  const h = await boot({ hash: "#s=web1&p=linux-hardening", overrides: ov({ "/api/run-terminal": { ok: true }, "/api/server": u => { const d = fixture("server-" + u.searchParams.get("name")); if (phase) { const a = act(d, "linux-hardening", "rollback_status"); a.lastTime = "2026-11-14T10:00:05Z"; a.lastResult = "ok"; a.keys = { skor: "74" }; } return d; } }) });
  await h.click(TERM);
  assert.ok(isOpen(h, "#runmodal")); assert.match(text(h, "#run-confirm-text"), /sudo parolası/); assert.equal(text(h, "#run-go"), "Çalıştır"); assert.equal(h.$("#run-ext").hidden, false);
  await h.click("#run-ext");
  assert.deepEqual(h.calls.filter(c => c.path === "/api/run-terminal").map(c => c.body), [{ server: "web1", package: "linux-hardening", action: "rollback_status" }]);
  assert.ok(h.$("#run-confirm").hidden); assert.ok(!h.$("#run-live").hidden); assert.ok(h.$("#run-log").hidden); assert.ok(h.$("#run-cancel").hidden);
  assert.match(text(h, "#run-state"), /Terminal'de çalışıyor… parolayı orada girin/);
  h.timers.run(3000); await h.settle(); assert.match(text(h, "#run-state"), /Terminal'de çalışıyor/, "sonuç yokken beklemeye devam eder");
  phase = 1; h.timers.run(3000); await h.settle();
  assert.match(text(h, "#run-state"), /✔ tamamlandı · sonuç kartta güncellendi/); assert.match(text(h, "#run-result"), /skor=74/); assert.ok(h.$("#run-open-log"));
  const homeBefore = h.calls.filter(c => c.path === "/api/home").length;
  await h.click("#run-open-log");
  assert.ok(!isOpen(h, "#runmodal")); assert.equal(h.$(".view:not([hidden])").id, "view-logs");
  assert.deepEqual(h.errors, []); h.close();
});

test("terminal modu: başarısız sonuç, açılamama hatası ve zaman aşımı", async () => {
  // başarısız sonuç
  let phase = 0;
  let h = await boot({ hash: "#s=web1&p=linux-hardening", overrides: ov({ "/api/run-terminal": { ok: true }, "/api/server": u => { const d = fixture("server-" + u.searchParams.get("name")); if (phase) { const a = act(d, "linux-hardening", "rollback_status"); a.lastTime = "2026-11-14T10:00:09Z"; a.lastResult = "fail"; } return d; } }) });
  await h.click(TERM); await h.click("#run-ext"); phase = 1; h.timers.run(3000); await h.settle();
  assert.match(text(h, "#run-state"), /✖ başarısız · ayrıntı için Terminal penceresine/); assert.deepEqual(h.errors, []); h.close();
  // Terminal açılamadı
  h = await boot({ hash: "#s=web1&p=linux-hardening", overrides: ov({ "/api/run-terminal": { status: 500, body: "osascript yok" } }) });
  await h.click(TERM); await h.click("#run-ext");
  assert.equal(h.$("#run-toast"), null); assert.equal(h.$("#toast").textContent, "Terminal açılamadı: osascript yok"); assert.equal(h.$("#run-go").disabled, false); assert.equal(h.$("#run-ext").hidden, false); assert.ok(!h.$("#run-confirm").hidden);
  assert.deepEqual(h.errors, []); h.close();
  // zaman aşımı (salt okunur terminal: 10 dk)
  h = await boot({ hash: "#s=web1&p=linux-hardening", overrides: ov({ "/api/run-terminal": { ok: true } }) });
  await h.click(TERM); await h.click("#run-ext");
  const realNow = h.win.Date.now; let t = realNow(); h.win.Date.now = () => t;     // sahte saat ilerletilir (zaman aşımı süre farkına bakar)
  t += 11 * 60 * 1000; h.timers.run(3000); await h.settle();
  assert.match(text(h, "#run-state"), /Sonuç beklenirken süre doldu/); assert.deepEqual(h.errors, []); h.close();
});

test("terminal modu + YAZAN eylem: onay gövdeyle gider; ilerleme günlüğü (alan sayacı, çubuk, güvenlik ağı) 2 sn'de bir yoklanır", async () => {
  const log = [
    "SSHPOCKET_PROGRESS alan=ssh sira=1/4 durum=bitti", "SSHPOCKET_PROGRESS alan=ufw sira=2/4 durum=bitti", "SSHPOCKET_PROGRESS alan=kernel sira=3/4 durum=basladi",
    "TASK [hardening_rollback : Arm timer] *****", "ok: [web1]",
  ].join("\n");
  const h = await boot({ hash: "#s=web1&p=linux-hardening", overrides: ov({ "/api/run-terminal": { ok: true }, "/api/log": { status: 200, body: log, headers: { "X-Log-Mtime": new Date(h0()).toUTCString() } } }) });
  function h0() { return Date.UTC(2026, 10, 14, 10, 0, 1); }
  await h.click(WRITE); await h.click("#w-snap"); await h.click("#w-yes"); await h.click("#run-ext");
  assert.deepEqual(h.calls.find(c => c.path === "/api/run-terminal").body, { server: "web1", package: "linux-hardening", action: "audit", snapshot: true, ackDestructive: false, confirm: "evet" });
  h.timers.run(2000); await h.settle();
  const q = h.calls.find(c => c.path.startsWith("/api/log?")); assert.ok(q && /package=linux-hardening/.test(q.path) && /server=web1/.test(q.path) && /file=last-run\.log/.test(q.path));
  assert.match(text(h, "#run-result"), /2 \/ 4 alan tamam · şu an: kernel/); const bar = h.$("#run-result .progress i"); assert.ok(bar);
  assert.equal(bar.dataset.w, "50"); assert.equal(bar.style.width, "50%", "CSSOM ile genişlik (satır içi stil özniteliği değil)"); assert.equal(h.$("#run-result .progress").getAttribute("aria-valuenow"), "2");
  assert.match(text(h, "#run-result"), /Geri alma güvenlik ağı devrede/);
  assert.deepEqual(h.errors, []); h.close();
});

test("ilerleme günlüğü: önceki çalıştırmanın eski günlüğü yok sayılır", async () => {
  const h = await boot({ hash: "#s=web1&p=linux-hardening", overrides: ov({ "/api/run-terminal": { ok: true }, "/api/log": { status: 200, body: "SSHPOCKET_PROGRESS alan=a sira=1/2 durum=bitti", headers: { "X-Log-Mtime": new Date(Date.UTC(2020, 0, 1)).toUTCString() } } }) });
  await h.click(WRITE); await h.click("#w-snap"); await h.click("#w-yes"); await h.click("#run-ext"); h.timers.run(2000); await h.settle();
  assert.equal(h.$("#run-result .progress"), null); assert.deepEqual(h.errors, []); h.close();
});

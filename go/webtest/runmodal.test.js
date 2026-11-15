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
// sudo'lu/yazan eylemler gömülü terminalde çalışır: /api/term/task gövdeleri tasks'a yazılır (rows/cols terminal boyutudur)
const taskOv = tasks => ({ "/api/term/task": (u, { body }) => { tasks.push(body); return { id: "k1", kind: "task", title: "görev" }; }, "/api/term/close": { ok: true }, "/api/term/resize": { ok: true }, "/api/term/input": { ok: true } });
const SZ = { rows: 24, cols: 80 };

// ---------------- yazan eylem: onay kapıları ----------------
test("yazan eylem: onay penceresi (başlık, etki, uyarılar), onaylar tamamlanmadan Çalıştır/Terminal kapalı; sırayla açılır", async () => {
  const h = await boot({ hash: "#s=web1&p=linux-hardening", overrides: ov() });
  await h.click(WRITE);
  assert.ok(isOpen(h, "#runmodal")); assert.equal(text(h, "#run-title"), "audit · Hardening yazar"); assert.equal(text(h, "#run-sub"), "Sunucu: web1");
  assert.equal(text(h, "#run-confirm-text"), "web1 sunucusuna gerçekten yazılacak: paket kurar.");
  assert.equal(text(h, "#run-go"), "Çalıştır (onaylı)"); assert.ok(h.$("#run-go").classList.contains("warnbtn")); assert.ok(!h.$("#run-go").classList.contains("dangerbtn"));
  assert.equal(h.$("#run-go").disabled, true); assert.equal(h.$("#run-ext"), null, "macOS Terminal alternatifi kalktı");
  assert.ok(h.$("#w-snap") && h.$("#w-yes")); assert.equal(h.$("#w-confirm"), null); assert.equal(h.$("#w-destr"), null, "yıkıcı değil: ek onay yok");
  assert.match(text(h, "#run-confirm-extra"), /Paketin uyarıları/);
  await h.click("#w-snap"); assert.equal(h.$("#run-go").disabled, true, "tek onay yetmez");
  await h.click("#w-yes"); assert.equal(h.$("#run-go").disabled, false);
  await h.click("#w-snap"); assert.equal(h.$("#run-go").disabled, true, "snapshot onayı geri alınınca yine kapanır");
  assert.deepEqual(h.errors, []); h.close();
});

test("yazan eylem: onay verilince /api/term/task'a (snapshot, onay, yıkıcı değil) gider; terminal penceresinde görev sekmesi açılır, modal HEMEN kapanır", async () => {
  const tasks = []; const h = await boot({ hash: "#s=web1&p=linux-hardening", overrides: ov(taskOv(tasks)) });
  await h.click(WRITE); await h.click("#w-snap"); await h.click("#w-yes"); await h.click("#run-go");
  assert.deepEqual(tasks, [{ server: "web1", package: "linux-hardening", action: "audit", snapshot: true, ackDestructive: false, confirm: "evet", ...SZ }]);
  assert.ok(!isOpen(h, "#runmodal"), "çalıştırma penceresi kapandı: terminal küçültülünce arkada sayfayı kilitleyen modal kalmaz"); assert.ok(h.$("#termmodal").classList.contains("open"), "terminal penceresi açıldı");
  assert.equal(h.$("#term-title").textContent, "⚙ linux-hardening · audit"); assert.match(h.$("#toast").textContent, /web1: audit · Hardening terminalde çalışıyor — parola ve onayları orada yazın/);
  assert.deepEqual(h.errors, []); h.close();
});

test("yazan eylem: ad-yazarak-onay (type-server-name) — ad birebir yazılana kadar kapalı; gövdede confirm=ad", async () => {
  const tasks = []; const h = await boot({ hash: "#s=web1&p=linux-hardening", overrides: ov(taskOv(tasks)) });
  await h.click('#srv-body [data-write="linux-hardening|rollback_now"]');
  assert.ok(h.$("#w-confirm")); assert.equal(h.$("#w-yes"), null); assert.match(text(h, "#run-confirm-extra"), /Onay için sunucu adını yazın: web1/);
  await h.click("#w-snap");
  await h.type("#w-confirm", "web"); assert.equal(h.$("#run-go").disabled, true);
  await h.type("#w-confirm", "WEB1"); assert.equal(h.$("#run-go").disabled, true, "büyük/küçük harf duyarlı");
  await h.type("#w-confirm", "web1"); assert.equal(h.$("#run-go").disabled, false);
  await h.click("#run-go"); assert.equal(tasks[0].confirm, "web1"); assert.equal(tasks[0].snapshot, true);
  assert.deepEqual(h.errors, []); h.close();
});

test("yazan eylem: YIKICI eylem — ek 'geri alınamaz' onayı, kırmızı düğme, TÜM uyarılar; gövdede ackDestructive", async () => {
  const tasks = [];
  const h = await boot({ hash: "#s=web1&p=linux-hardening", overrides: ov({ ...taskOv(tasks), ...serverOv((d) => { const a = act(d, "linux-hardening", "rollback_now"); a.destructive = true; a.warnings = ["u1", "u2", "u3", "u4", "u5"]; }) }) });
  await h.click('#srv-body [data-write="linux-hardening|rollback_now"]');
  assert.equal(text(h, "#run-title"), "rollback_now · Hardening YIKICI"); assert.match(text(h, "#run-confirm-text"), /YIKICI EYLEM.*geri alınamayabilir/);
  assert.equal(text(h, "#run-go"), "Yıkıcı eylemi çalıştır"); assert.ok(h.$("#run-go").classList.contains("dangerbtn"));
  assert.equal(h.$$("#run-confirm-extra li").length, 5, "yıkıcıda uyarıların tamamı gösterilir"); assert.equal(h.$("#run-confirm-extra details"), null);
  await h.click("#w-snap"); await h.type("#w-confirm", "web1"); assert.equal(h.$("#run-go").disabled, true, "geri alınamaz onayı olmadan açılmaz");
  await h.click("#w-destr"); assert.equal(h.$("#run-go").disabled, false);
  await h.click("#run-go"); assert.deepEqual([tasks[0].ackDestructive, tasks[0].confirm], [true, "web1"]);
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

test("terminal modu: pencere sudo parolasını anlatır; 'Çalıştır' /api/term/task'a gider, modal kapanır; görev bitince sonuç kartı yenilenir ve bildirim çıkar", async () => {
  let phase = 0;
  const h = await boot({ hash: "#s=web1&p=linux-hardening", overrides: ov({ ...taskOv([]), "/api/server": u => { const d = fixture("server-" + u.searchParams.get("name")); if (phase) { const a = act(d, "linux-hardening", "rollback_status"); a.lastTime = "2026-11-14T10:00:05Z"; a.lastResult = "ok"; a.keys = { zzz_sonuc: "74" }; } return d; } }) });
  await h.click(TERM); assert.doesNotMatch(text(h, "#srv-body"), /zzz_sonuc/, "eylem çalışmadan sonuç anahtarı yok");
  assert.ok(isOpen(h, "#runmodal")); assert.match(text(h, "#run-confirm-text"), /sudo parolası/); assert.equal(text(h, "#run-go"), "Çalıştır"); assert.match(text(h, "#run-confirm-text"), /terminal penceresinde yazarsınız/);
  await h.click("#run-go");
  assert.deepEqual(h.calls.filter(c => c.path === "/api/term/task").map(c => c.body), [{ server: "web1", package: "linux-hardening", action: "rollback_status", ...SZ }]); assert.ok(h.$("#termmodal").classList.contains("open"), "terminal penceresi açıldı");
  assert.ok(!isOpen(h, "#runmodal"), "modal kapandı"); assert.match(h.$("#toast").textContent, /rollback_status · Hardening terminalde çalışıyor/);
  const srvCalls = () => h.calls.filter(c => c.path === "/api/server?name=web1").length; const before = srvCalls();
  h.timers.run(10000); await h.settle(); assert.equal(h.calls.filter(c => c.path.startsWith("/api/log?")).length, 0, "modal yok: günlük aynası yoklanmaz");
  phase = 1; h.win.__esObj[0].emit("exit", "0"); await h.settle();
  assert.ok(srvCalls() > before, "görev bitince sunucu sonucu yeniden okunur"); assert.match(h.$("#toast").textContent, /web1: ✔ rollback_status · Hardening tamamlandı/);
  assert.match(text(h, "#srv-body"), /zzz_sonuc=74/, "sonuç kartı (anahtar değerler) sayfada yenilendi");
  assert.deepEqual(h.errors, []); h.close();
});

test("terminal modu: başarısız sonuç bildirimi ve açılamama hatası", async () => {
  // başarısız sonuç: çıkış kodu sıfır değil
  let h = await boot({ hash: "#s=web1&p=linux-hardening", overrides: ov({ ...taskOv([]) }) });
  await h.click(TERM); await h.click("#run-go"); h.win.__esObj[0].emit("exit", "3"); await h.settle();
  assert.match(h.$("#toast").textContent, /web1: ✖ rollback_status · Hardening başarısız \(çıkış kodu 3\); ayrıntı terminal sekmesinde/); assert.deepEqual(h.errors, []); h.close();
  // Terminal açılamadı
  h = await boot({ hash: "#s=web1&p=linux-hardening", overrides: ov({ "/api/term/task": { status: 409, body: "bu sunucuda zaten çalışan bir eylem var" } }) });
  await h.click(TERM); await h.click("#run-go");
  assert.equal(h.$("#run-toast"), null); assert.equal(h.$("#toast").textContent, "Eylem başlatılamadı: bu sunucuda zaten çalışan bir eylem var"); assert.equal(h.$("#run-go").disabled, false); assert.ok(!h.$("#run-confirm").hidden); assert.ok(!h.$("#termmodal").classList.contains("open"), "başlamayan görev için boş terminal penceresi kalmaz");
  assert.ok(isOpen(h, "#runmodal"), "başlamadıysa modal açık kalır (yeniden denenebilsin)");
  assert.deepEqual(h.errors, []); h.close();
});

test("terminal modu + YAZAN eylem: onay gövdeyle gider; modal kapanır ve ilerleme günlüğü AYNALANMAZ (ilerleme terminalde)", async () => {
  const h = await boot({ hash: "#s=web1&p=linux-hardening", overrides: ov({ ...taskOv([]) }) });
  await h.click(WRITE); await h.click("#w-snap"); await h.click("#w-yes"); await h.click("#run-go");
  assert.deepEqual(h.calls.find(c => c.path === "/api/term/task").body, { server: "web1", package: "linux-hardening", action: "audit", snapshot: true, ackDestructive: false, confirm: "evet", ...SZ });
  assert.ok(!isOpen(h, "#runmodal")); h.timers.run(10000); await h.settle();
  assert.equal(h.calls.filter(c => c.path.startsWith("/api/log?")).length, 0, "günlük yoklanmaz: ikinci bir ilerleme görünümü yok");
  assert.deepEqual(h.errors, []); h.close();
});

test("terminal modu: görev terminali bitince ('keylab:taskexit') sonuç hemen yenilenir; olay alanları eksikse yok sayılır", async () => {
  let phase = 0;
  const h = await boot({ hash: "#s=web1&p=linux-hardening", overrides: ov({ ...taskOv([]), "/api/server": u => { const d = fixture("server-" + u.searchParams.get("name")); if (phase) { const a = act(d, "linux-hardening", "rollback_status"); a.lastTime = "2026-11-14T10:00:05Z"; a.lastResult = "ok"; } return d; } }) });
  await h.click(TERM); await h.click("#run-go");
  const srvCalls = () => h.calls.filter(c => c.path === "/api/server?name=web1").length;
  const n0 = srvCalls(); h.win.dispatchEvent(new h.win.CustomEvent("keylab:taskexit", { detail: { server: "web1" } })); await h.settle();
  assert.equal(srvCalls(), n0, "eksik alanlı olay yok sayılır");
  phase = 1; h.win.__esObj[0].emit("exit", "0"); await h.settle(); // zamanlayıcı İLERLETİLMEDİ: yoklama yok, olayla hemen
  assert.ok(srvCalls() > n0); assert.match(h.$("#toast").textContent, /✔ rollback_status · Hardening tamamlandı/); assert.deepEqual(h.errors, []); h.close();
});

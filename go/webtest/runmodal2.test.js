"use strict";
/* ÇALIŞTIRMA PENCERESİ KARAKTERİZASYON TESTLERİ, 2. grup: parola/e-h istemleri, bitiş durumları, pill, kapatma, bağlantı kopması,
   silme/görev/yükseltme modları ve Ayarlar'daki parola girişi tercihi. (1. grup: runmodal.test.js.) */
process.env.TZ = "UTC";
const test = require("node:test");
const assert = require("node:assert/strict");
const { boot, fixture } = require("./harness");
const { fakeJobs, mixedHome, status } = require("./lib/fakejobs");

const ov = extra => ({ "/api/home": mixedHome, "/api/status": status, ...extra });
const isOpen = (h, s) => h.$(s).classList.contains("open");
const text = (h, s) => h.$(s).textContent.replace(/\s+/g, " ").trim();
const tickPoll = async (h, n = 1) => { for (let i = 0; i < n; i++) { h.timers.run(600); await h.settle(); } };
const key = (h, k, extra = {}, target) => { (target || h.doc).dispatchEvent(new h.win.KeyboardEvent("keydown", { key: k, bubbles: true, cancelable: true, ...extra })); return h.settle(); };
const runBtn = h => h.$$("#srv-body button[data-run]").find(b => b.textContent.trim() === "Çalıştır");
const toast = h => h.$("#toast").textContent;

// utility/durum işini başlatır (web1); iş j1
async function startJob(opts = {}) {
  const f = fakeJobs();
  const h = await boot({ hash: "#s=web1&p=utility", overrides: ov({ ...f.overrides, ...(opts.over || {}) }), storage: opts.storage });
  await h.click(runBtn(h)); await h.click("#run-go");
  return { f, h };
}
const askInline = { "sshpocket.askpass": "inline" };

// ---------------- istemler (parola, e/h) ----------------
test("istem: parola — 'sistem' modu işletim sistemi penceresini ister (sayfa içi kutu açılmaz), aynı çıktıda yeniden sormaz", async () => {
  const { f, h } = await startJob({ over: { "/api/run/askpass": { ok: true } } });
  f.feed("j1", "[sudo] password for root:"); await tickPoll(h, 3);
  assert.deepEqual(h.calls.filter(c => c.path === "/api/run/askpass").map(c => c.body), [{ id: "j1", prompt: "[sudo] password for root:" }]);
  assert.ok(h.$("#run-input").hidden); assert.match(text(h, "#run-state"), /parola için sistem penceresi açıldı/);
  await tickPoll(h, 4); assert.equal(h.calls.filter(c => c.path === "/api/run/askpass").length, 1, "aynı çıktı uzunluğunda yeniden sorulmaz");
  assert.deepEqual(h.errors, []); h.close();
});

test("istem: sistem penceresi hata/iptal verirse bildirim + SAYFA İÇİ kutuya düşer", async () => {
  const { f, h } = await startJob({ over: { "/api/run/askpass": { status: 500, body: "iptal edildi" } } });
  f.feed("j1", "Enter passphrase for key:"); await tickPoll(h, 3);
  assert.equal(toast(h), "Parola penceresi: iptal edildi — sayfa içi kutu açıldı");
  await tickPoll(h, 1); assert.ok(!h.$("#run-input").hidden, "bir sonraki turda sayfa içi kutu açılır"); assert.equal(h.$("#run-prompt").textContent, "Enter passphrase for key:");
  assert.deepEqual(h.errors, []); h.close();
});

test("istem: sayfa içi parola kutusu (inline modu) — gizli yazı, Enter gönderir, kutu temizlenip gizlenir", async () => {
  const { f, h } = await startJob({ storage: askInline, over: { "/api/run/input": { ok: true } } });
  f.feed("j1", "[sudo] password for root:"); await tickPoll(h, 3);
  assert.deepEqual(h.calls.filter(c => c.path === "/api/run/askpass"), [], "inline modda sistem penceresi istenmez");
  assert.ok(!h.$("#run-input").hidden); assert.equal(h.$("#run-prompt").textContent, "[sudo] password for root:"); assert.equal(h.$("#run-in").type, "password");
  assert.ok(h.$("#run-yes").hidden && h.$("#run-no").hidden, "parola isteminde e/h düğmesi yok"); assert.equal(h.$("#run-in").getAttribute("autocomplete"), "off"); assert.equal(h.$("#run-in").getAttribute("data-lpignore"), "true");
  h.$("#run-in").value = "gizli-parola"; await key(h, "Enter", {}, h.$("#run-in"));
  assert.deepEqual(h.calls.filter(c => c.path === "/api/run/input").map(c => c.body), [{ id: "j1", text: "gizli-parola" }]);
  assert.equal(h.$("#run-in").value, ""); assert.ok(h.$("#run-input").hidden);
  assert.deepEqual(h.errors, []); h.close();
});

test("istem: parola gönderilemezse bildirim; Gönder düğmesi de aynı yolu kullanır", async () => {
  const { f, h } = await startJob({ storage: askInline, over: { "/api/run/input": { status: 500, body: "iş bitti" } } });
  f.feed("j1", "password:"); await tickPoll(h, 3);
  h.$("#run-in").value = "x"; await h.click("#run-send"); assert.equal(toast(h), "Gönderilemedi: iş bitti");
  assert.deepEqual(h.errors, []); h.close();
});

test("istem: e/h, yes/no, y/n soruları hızlı yanıt düğmeleri çıkarır ve doğru metni gönderir", async () => {
  for (const [prompt, yes, no, yTxt, nTxt] of [["Devam edilsin mi? [e/h]", "e", "H", "Evet (e)", "Hayır (H)"], ["Continue connecting (yes/no)?", "yes", "no", "Evet (yes)", "Hayır (no)"], ["Overwrite? (y/n)", "y", "n", "Evet (y)", "Hayır (n)"]]) {
    const { f, h } = await startJob({ storage: askInline, over: { "/api/run/input": { ok: true } } });
    f.feed("j1", prompt); await tickPoll(h, 3);
    assert.ok(!h.$("#run-yes").hidden && !h.$("#run-no").hidden, prompt); assert.equal(h.$("#run-yes").textContent, yTxt); assert.equal(h.$("#run-no").textContent, nTxt); assert.equal(h.$("#run-in").type, "text", "gizli olmayan istem düz metin");
    await h.click("#run-yes"); assert.deepEqual(h.calls.filter(c => c.path === "/api/run/input").map(c => c.body.text), [yes]); assert.ok(h.$("#run-input").hidden);
    h.close();
  }
  const { f, h } = await startJob({ storage: askInline, over: { "/api/run/input": { ok: true } } });
  f.feed("j1", "Devam? [e/h]"); await tickPoll(h, 3); await h.click("#run-no");
  assert.deepEqual(h.calls.filter(c => c.path === "/api/run/input").map(c => c.body.text), ["H"]); assert.deepEqual(h.errors, []); h.close();
});

test("istem: yeni çıktı gelince kutu gizlenir; pencere küçükken istem gelirse pencere açılır ve uyarı çıkar", async () => {
  const { f, h } = await startJob({ storage: askInline });
  f.feed("j1", "Devam? [e/h]"); await tickPoll(h, 3); assert.ok(!h.$("#run-input").hidden);
  f.feed("j1", " cevap alındı\nsonraki satır\n"); await tickPoll(h, 1); assert.ok(h.$("#run-input").hidden, "yeni bayt gelince istem kutusu kapanır");
  await h.click("#run-min"); assert.ok(!isOpen(h, "#runmodal"));
  f.feed("j1", "başka istem [y/n]"); await tickPoll(h, 3);
  assert.ok(isOpen(h, "#runmodal")); assert.equal(toast(h), "İş sizden yanıt bekliyor."); assert.ok(!h.$("#run-input").hidden);
  assert.deepEqual(h.errors, []); h.close();
});

// ---------------- bitiş durumları, günlük gösterimi, pill ----------------
test("bitiş: başarı — süre, 'Sonuç:' çipleri, düğmeler; sunucu verisi ve liste yenilenir", async () => {
  const { f, h } = await startJob();
  f.feed("j1", "TASK [x] ***\nok: [web1]\nSonuç:\n  skor=74\n  uyari=1\n  bu_satir_degil\n"); f.finish("j1", 0);
  const srvBefore = h.calls.filter(c => c.path === "/api/server?name=web1").length, homeBefore = h.calls.filter(c => c.path === "/api/home").length;
  await tickPoll(h, 1);
  assert.equal(text(h, "#run-state"), "✔ başarılı · 0 sn"); assert.deepEqual(h.$$("#run-result .chip").map(c => c.textContent), ["skor=74", "uyari=1"]);
  assert.ok(h.$("#run-cancel").hidden && h.$("#run-min").hidden); assert.ok(!h.$("#run-close").hidden); assert.equal(h.$("#run-close").textContent, "Kapat");
  assert.equal(h.calls.filter(c => c.path === "/api/server?name=web1").length, srvBefore + 1); assert.equal(h.calls.filter(c => c.path === "/api/home").length, homeBefore + 1);
  assert.deepEqual(h.errors, []); h.close();
});

test("bitiş: sunucu sayfası YENİ veriyle yeniden çizilir (eylemin son sonucu ekrana yansır); arka plandaki iş bitince de", async () => {
  let phase = 0;
  const fresh = u => { const d = fixture("server-" + u.searchParams.get("name")); if (phase && u.searchParams.get("name") === "web1") { const a = d.packages.find(p => p.id === "utility").actions.find(x => x.id === "durum"); a.lastTime = "2026-11-14T09:59:00Z"; a.lastResult = "ok"; } return d; };
  const f = fakeJobs(); const h = await boot({ hash: "#s=web1&p=utility", overrides: ov({ ...f.overrides, "/api/server": fresh }) });
  const okCount = () => h.$$("#srv-body .res.ok").length, before = okCount();
  await h.click(runBtn(h)); await h.click("#run-go"); phase = 1; f.finish("j1", 0); await tickPoll(h, 1);
  assert.ok(okCount() > before, "iş bitince sunucu sayfası yeni sonucu gösterir (✔ başarılı)");
  // arka planda biten iş: başka sunucuda iken bitince ve o sunucu sayfası açıkken yeniden çizilir
  phase = 0; await h.click("#run-close");
  await h.click(runBtn(h)); await h.click("#run-go"); await h.click("#run-min");
  await h.click('[data-tsrv="db1"]'); await h.click('#srv-tabs button[data-p="utility"]'); await h.click(runBtn(h));   // web1 işi arka plana geçer
  await h.click('[data-tsrv="web1"]'); await h.click('#srv-tabs button[data-p="utility"]'); const b2 = okCount();
  phase = 1; f.finish("j2", 0); h.timers.run(1500); await h.settle();
  assert.equal(h.$("#srv-head h1").textContent, "web1"); assert.ok(okCount() > b2, "arka plan işi bitince açık sunucu sayfası yenilenir");
  assert.deepEqual(h.errors, []); h.close();
});

test("bitiş: hata — çıkış kodu, uyarı kutusu; pill kırmızı; pencere kapalıyken bildirim", async () => {
  const { f, h } = await startJob();
  await h.click("#run-min"); f.finish("j1", 3); await tickPoll(h, 1);
  assert.equal(toast(h), "✖ durum · Utility · web1"); assert.ok(!h.$("#job-pill").hidden); assert.equal(h.$("#job-pill").className, "job-pill fail"); assert.equal(h.$("#job-pill").textContent, "✖ durum · Utility · web1");
  await h.click("#job-pill"); assert.ok(isOpen(h, "#runmodal")); assert.ok(h.$("#job-pill").hidden, "pencere açıkken pill gizli");
  assert.equal(text(h, "#run-state"), "✖ başarısız · çıkış kodu 3 · 0 sn"); assert.match(text(h, "#run-result"), /Eylem başarısız oldu/);
  await h.click("#run-close"); assert.ok(!isOpen(h, "#runmodal")); assert.ok(h.$("#job-pill").hidden, "kapatınca pill temizlenir");
  assert.deepEqual(h.errors, []); h.close();
});

test("bitiş: başarıda pill yeşil + bildirim; çalışırken küçültülmüş pill '⏳ … sn' gösterir ve tıklayınca pencereyi açar", async () => {
  const { f, h } = await startJob();
  await h.click("#run-min"); assert.equal(h.$("#job-pill").className, "job-pill run"); assert.match(h.$("#job-pill").textContent, /^⏳ durum · Utility · web1 · \d+ sn$/);
  assert.equal(h.$("#job-pill").getAttribute("aria-label"), h.$("#job-pill").textContent);
  await h.click("#job-pill"); assert.ok(isOpen(h, "#runmodal")); await h.click("#run-min");
  f.finish("j1", 0); await tickPoll(h, 1); assert.equal(toast(h), "✔ durum · Utility · web1"); assert.equal(h.$("#job-pill").className, "job-pill ok");
  assert.deepEqual(h.errors, []); h.close();
});

test("günlük: ANSI kaçışları ve satır başı (\\r) işlenir, satır sınıfları (hata/ok/görev), HTML kaçışlı", async () => {
  const { f, h } = await startJob();
  f.feed("j1", "\x1b[31mkırmızı\x1b[0m\nilk\rAB\nTASK [a] **\nok: [web1]\nfatal: [web1]: FAILED!\n<img src=x onerror=1>\n"); await tickPoll(h, 1);
  const rows = h.$$("#run-log > div"), by = c => rows.filter(r => r.className === c).map(r => r.textContent);
  assert.ok(rows.some(r => r.textContent === "kırmızı"), "ANSI kaçışları atılır"); assert.ok(rows.some(r => r.textContent === "ABk"), "\\r ile satır başına dönüp üzerine yazar");
  assert.deepEqual(by("t"), ["TASK [a] **"]); assert.deepEqual(by("ok"), ["ok: [web1]"]); assert.deepEqual(by("err"), ["fatal: [web1]: FAILED!"]);
  assert.equal(h.$$("#run-log img").length, 0); assert.ok(rows.some(r => r.textContent === "<img src=x onerror=1>"));
  assert.deepEqual(h.errors, []); h.close();
});

test("günlük: SSHPOCKET_PROGRESS satırları canlı ilerleme çubuğuna dönüşür (genişlik CSSOM ile)", async () => {
  const { f, h } = await startJob();
  f.feed("j1", "SSHPOCKET_PROGRESS alan=a sira=1/3 durum=bitti\nSSHPOCKET_PROGRESS alan=b sira=2/3 durum=basladi\n"); await tickPoll(h, 1);
  assert.match(text(h, "#run-progress"), /1 \/ 3 alan tamam · şu an: b/); const bar = h.$("#run-progress .progress i");
  assert.equal(bar.dataset.w, "33"); assert.equal(bar.style.width, "33%"); assert.equal(h.$("#run-progress .progress").getAttribute("aria-valuemax"), "3");
  assert.deepEqual(h.errors, []); h.close();
});

// ---------------- kapatma, iptal, kopma ----------------
test("kapatma: onay ekranında Vazgeç/Esc kapatır; iş sürerken Esc kapatmaz", async () => {
  const f = fakeJobs(); const h = await boot({ hash: "#s=web1&p=utility", overrides: ov(f.overrides) });
  await h.click(runBtn(h)); assert.ok(isOpen(h, "#runmodal")); assert.equal(h.$("#run-close").textContent, "Vazgeç");
  await h.click("#run-close"); assert.ok(!isOpen(h, "#runmodal"));
  await h.click(runBtn(h)); await key(h, "Escape"); assert.ok(!isOpen(h, "#runmodal"), "Esc de kapatır");
  await h.click(runBtn(h)); await h.click("#run-go"); await key(h, "Escape"); assert.ok(isOpen(h, "#runmodal"), "iş sürerken Esc pencereyi kapatmaz"); assert.ok(h.$("#run-close").hidden);
  assert.deepEqual(h.errors, []); h.close();
});

test("iptal: sunucu reddederse bildirim; iptal edilen iş pencerede 'başarısız' bitirilir", async () => {
  const { f, h } = await startJob({ over: { "/api/run/cancel": { status: 409, body: "paket kurulumu atomiktir" } } });
  await h.click("#run-cancel"); assert.equal(toast(h), "İptal edilemedi: paket kurulumu atomiktir"); assert.equal(h.$("#run-cancel").disabled, true);
  assert.deepEqual(h.errors, []); h.close();
});

test("bağlantı kopması: yoklama hata verirse 'Bağlantı koptu', Kapat görünür, pill ve Küçült gizlenir", async () => {
  const { h } = await startJob({ over: { "/api/run": (u, { method }) => (method === "POST" ? { id: "x1" } : { status: 500, body: "ağ koptu" }) } });
  assert.match(text(h, "#run-state"), /^Bağlantı koptu: ağ koptu$/); assert.ok(isOpen(h, "#runmodal")); assert.ok(!h.$("#run-close").hidden); assert.equal(h.$("#run-close").textContent, "Kapat");
  assert.ok(h.$("#run-min").hidden); assert.ok(h.$("#job-pill").hidden);
  await h.click("#run-close"); assert.ok(!isOpen(h, "#runmodal"), "kopma sonrası kapatılabilir (iş izlenmiyor)");
  assert.deepEqual(h.errors, []); h.close();
});

// ---------------- silme / görev / yükseltme modları ----------------
const deleteOv = (f, extra = {}) => ({
  ...f.overrides, "/api/server-files": { files: [{ name: "server.env", isKey: false }], state: ["x"] },
  "/api/server-remove": () => ({ id: f.start("db1", "remove", "") }), "/api/server-purge": { ok: true }, ...extra,
});
const openDeleteDlg = async (h, name = "db1") => {
  h.$$("#servers [data-name]").find(e => e.dataset.name === name).dispatchEvent(new h.win.MouseEvent("contextmenu", { bubbles: true, cancelable: true, clientX: 5, clientY: 5 })); await h.settle();
  await h.click('#ctx button[data-ctx="delete"]');
};

test("silme modu: onay (ad yazılınca açılır) → /api/server-remove → 'Sunucu siliniyor' penceresi → bitince günlükler silinir, liste yenilenir, Sunucular sayfasına dönülür", async () => {
  const f = fakeJobs(); const h = await boot({ overrides: ov(deleteOv(f)) });
  await openDeleteDlg(h); assert.equal(h.$("#dlg-ok").disabled, true);
  await h.type("#dl-name", "db1"); assert.equal(h.$("#dlg-ok").disabled, false); await h.click("#dl-logs");
  await h.click("#dlg-ok");
  assert.deepEqual(h.calls.find(c => c.path === "/api/server-remove").body, { name: "db1", confirm: "db1", ack: false });
  assert.ok(isOpen(h, "#runmodal")); assert.ok(!isOpen(h, "#dlg")); assert.equal(text(h, "#run-title"), "Sunucu siliniyor db1"); assert.equal(text(h, "#run-sub"), "KeyLab motoru (ModuleManager.sh remove)");
  f.finish("j1", 0); const homeBefore = h.calls.filter(c => c.path === "/api/home").length; await tickPoll(h, 1);
  assert.deepEqual(h.calls.filter(c => c.path === "/api/server-purge").map(c => c.body), [{ name: "db1" }]);
  assert.equal(h.calls.filter(c => c.path === "/api/home").length, homeBefore + 1); assert.equal(h.$(".view:not([hidden])").id, "view-servers"); assert.equal(toast(h), "db1 silindi");
  assert.deepEqual(h.errors, []); h.close();
});

test("silme modu: günlük silme işaretlenmediyse purge yok; iş başarısızsa gezinme/purge/bildirim yok", async () => {
  let f = fakeJobs(); let h = await boot({ overrides: ov(deleteOv(f)) });
  await openDeleteDlg(h); await h.type("#dl-name", "db1"); await h.click("#dlg-ok"); f.finish("j1", 0); await tickPoll(h, 1);
  assert.equal(h.calls.filter(c => c.path === "/api/server-purge").length, 0, "işaretsiz: günlük silinmez"); assert.equal(toast(h), "db1 silindi"); h.close();
  f = fakeJobs(); h = await boot({ overrides: ov(deleteOv(f)) });
  await openDeleteDlg(h); await h.type("#dl-name", "db1"); await h.click("#dl-logs"); await h.click("#dlg-ok"); f.finish("j1", 1); await tickPoll(h, 1);
  assert.equal(h.calls.filter(c => c.path === "/api/server-purge").length, 0); assert.notEqual(toast(h), "db1 silindi"); assert.equal(h.$(".view:not([hidden])").id, "view-dash", "başarısız silmede sayfa değişmez");
  assert.match(text(h, "#run-state"), /başarısız/); assert.deepEqual(h.errors, []); h.close();
});

test("silme modu: günlük silinemezse bildirim; yine de liste yenilenir ve sayfa değişir", async () => {
  const f = fakeJobs(); const h = await boot({ overrides: ov(deleteOv(f, { "/api/server-purge": { status: 500, body: "izin yok" } })) });
  await openDeleteDlg(h); await h.type("#dl-name", "db1"); await h.click("#dl-logs"); await h.click("#dlg-ok"); f.finish("j1", 0); await tickPoll(h, 1);
  assert.equal(h.$(".view:not([hidden])").id, "view-servers"); assert.equal(toast(h), "db1 silindi", "son bildirim silindi mesajıdır"); assert.deepEqual(h.errors, []); h.close();
});

test("görev modu (anahtar üret): /api/server-task, bitince liste ve sunucu verisi yenilenir", async () => {
  const f = fakeJobs();
  const h = await boot({ hash: "#s=web1", overrides: ov({ ...f.overrides, "/api/server-task": () => ({ id: f.start("web1", "keygen", "") }) }) });
  await h.click('#srv-body [data-task="keygen"]');
  assert.ok(isOpen(h, "#runmodal")); assert.equal(text(h, "#run-title"), "Anahtar oluştur web1"); assert.equal(text(h, "#run-sub"), "Bağlantı hazırlığı");
  await h.click("#run-go"); assert.deepEqual(h.calls.find(c => c.path === "/api/server-task").body, { server: "web1", task: "keygen" });
  f.finish("j1", 0); const homeB = h.calls.filter(c => c.path === "/api/home").length, srvB = h.calls.filter(c => c.path === "/api/server?name=web1").length; await tickPoll(h, 1);
  assert.equal(text(h, "#run-state"), "✔ başarılı · 0 sn"); assert.equal(h.calls.filter(c => c.path === "/api/home").length, homeB + 1); assert.equal(h.calls.filter(c => c.path === "/api/server?name=web1").length, srvB + 1);
  assert.deepEqual(h.errors, []); h.close();
});

test("yükseltme modu: Paketler > Güncellemeler > yükselt → /api/upgrade; bitince paket listesi ve güncelleme raporu yenilenir", async () => {
  const f = fakeJobs();
  const report = { ok: true, serial: "2", valid: "2027-01-01", exit: 0, raw: "ham rapor", rows: [{ id: "utility", installed: "0.1.0", verdict: "upgrade", version: "0.1.1", text: "yükseltilebilir" }] };
  const h = await boot({ hash: "#v=packages", overrides: ov({ ...f.overrides, "/api/updates": report, "/api/upgrade": () => ({ id: f.start("", "upgrade", "utility") }) }) });
  await h.click("#upd-check"); assert.ok(isOpen(h, "#dlg")); const up = h.$("#dlg-body [data-upg]"); assert.ok(up, "yükselt düğmesi"); assert.equal(up.dataset.to, "0.1.1");
  await h.click(up); assert.ok(!isOpen(h, "#dlg")); assert.ok(isOpen(h, "#runmodal")); assert.match(text(h, "#run-title"), /^upgrade · utility USB'ye yazar$/); assert.equal(text(h, "#run-sub"), "KeyLab paket güncellemesi (imzalı indeks)");
  await h.click("#run-go"); assert.deepEqual(h.calls.find(c => c.path === "/api/upgrade").body, { package: "utility", version: "0.1.1", ack: true });
  f.finish("j1", 0); const pk = h.calls.filter(c => c.path === "/api/packages").length, up2 = h.calls.filter(c => c.path === "/api/updates").length; await tickPoll(h, 1);
  assert.equal(h.calls.filter(c => c.path === "/api/packages").length, pk + 1, "paket listesi yenilenir"); assert.equal(h.calls.filter(c => c.path === "/api/updates").length, up2 + 1, "güncelleme raporu yeniden denetlenir");
  assert.deepEqual(h.errors, []); h.close();
});

// ---------------- bitişte zamanlayıcılar durur; yazan iş de arka plana alınır; terminal ilerleme yoklaması durur ----------------
test("bitiş: yoklama ve sayaç durur (artık /api/run istenmez, durum yazısı 'çalışıyor…' ile ezilmez)", async () => {
  const { f, h } = await startJob();
  f.finish("j1", 0); await tickPoll(h, 1);
  const polls = () => h.calls.filter(c => c.path.startsWith("/api/run?id=")).length; const n = polls();
  assert.equal(text(h, "#run-state"), "✔ başarılı · 0 sn");
  h.timers.run(5000); await h.settle();
  assert.equal(polls(), n, "bitmiş iş için yoklama sürmemeli"); assert.equal(text(h, "#run-state"), "✔ başarılı · 0 sn", "sayaç (1 sn) bitiş yazısını ezmemeli");
  assert.deepEqual(h.errors, []); h.close();
});

test("günlük: 'changed:' satırları da yeşil (ok) sınıfı alır", async () => {
  const { f, h } = await startJob();
  f.feed("j1", "changed: [web1]\nok: [web1]\nskipping: [web1]\n"); await tickPoll(h, 1);
  assert.deepEqual(h.$$("#run-log > div.ok").map(r => r.textContent), ["changed: [web1]", "ok: [web1]"]);
  assert.deepEqual(h.errors, []); h.close();
});

test("arka plana alma: YAZAN (onaylı) iş de başka sunucuda yeni eylem başlatılırken arka plana geçer", async () => {
  const f = fakeJobs(); const h = await boot({ hash: "#s=web1&p=linux-hardening", overrides: ov(f.overrides) });
  await h.click('#srv-body [data-write="linux-hardening|audit"]'); await h.click("#w-snap"); await h.click("#w-yes"); await h.click("#run-go");
  assert.deepEqual(f.posts.map(p => `${p.server}/${p.action}`), ["web1/audit"]); await h.click("#run-min");
  await h.click('[data-tsrv="db1"]'); await h.click('#srv-tabs button[data-p="utility"]'); await h.click(runBtn(h));
  assert.match(toast(h), /web1: audit arka planda sürüyor/, "yazan iş atılmaz, arka plana geçer"); assert.ok(isOpen(h, "#runmodal")); assert.equal(text(h, "#run-sub"), "Sunucu: db1");
  assert.deepEqual(h.errors, []); h.close();
});

test("terminal + yazan eylem: sonuç gelince ilerleme günlüğü yoklaması DURUR", async () => {
  let phase = 0;
  const h = await boot({ hash: "#s=web1&p=linux-hardening", overrides: ov({ "/api/run-terminal": { ok: true }, "/api/log": { status: 200, body: "SSHPOCKET_PROGRESS alan=a sira=1/2 durum=bitti", headers: { "X-Log-Mtime": new Date(Date.UTC(2026, 10, 14, 10, 0, 1)).toUTCString() } },
    "/api/server": u => { const d = fixture("server-" + u.searchParams.get("name")); if (phase) { const a = d.packages.find(p => p.id === "linux-hardening").actions.find(x => x.id === "audit"); a.lastTime = "2026-11-14T10:00:09Z"; a.lastResult = "ok"; } return d; } }) });
  await h.click('#srv-body [data-write="linux-hardening|audit"]'); await h.click("#w-snap"); await h.click("#w-yes"); await h.click("#run-ext");
  h.timers.run(2000); await h.settle(); const logCalls = () => h.calls.filter(c => c.path.startsWith("/api/log?")).length; assert.ok(logCalls() >= 1, "ilerleme yoklanıyor");
  phase = 1; h.timers.run(3000); await h.settle(); assert.match(text(h, "#run-state"), /✔ tamamlandı/);
  const n = logCalls(); h.timers.run(10000); await h.settle();
  assert.equal(logCalls(), n, "sonuç geldikten sonra günlük yoklaması sürmemeli");
  assert.deepEqual(h.errors, []); h.close();
});

test("görev modu + Sunucu ekle sihirbazı: bitince adım ✔ olur (markTaskDone + renderAddSteps bağlantısı); başarısızsa olmaz", async () => {
  for (const [exit, expectDone] of [[0, true], [1, false]]) {
    const f = fakeJobs(); let created = false;
    const home = () => { const d = mixedHome(); if (created) d.servers.push({ name: "yeni1", host: "10.9.9.9", user: "root", port: "22", platform: "linux", os: "linux", hasKey: true, up: false, checked: true, conn: "down", valid: true, knock: false, last: null, recent: [], lynis: null }); return d; };
    const h = await boot({ hash: "#v=add", overrides: ov({ ...f.overrides, "/api/home": home,
      "/api/server-draft": { ok: true, envPath: "servers/yeni1/server.env", envText: "IP=10.9.9.9", keyPath: "servers/yeni1/id_ed25519_yeni1", commands: ["ssh-copy-id x"] },
      "/api/server-create": () => { created = true; return { ok: true }; }, "/api/server-task": () => ({ id: f.start("yeni1", "keycopy", "") }) }) });
    for (const [id, v] of [["#add-name", "yeni1"], ["#add-host", "10.9.9.9"], ["#add-user", "root"], ["#add-port", "22"]]) await h.type(id, v);
    h.timers.run(600); await h.settle();   // alan değişince taslak (önizleme) kısa bir gecikmeyle istenir
    await h.click("#add-create"); assert.equal(h.$("#add-steps").hidden, false, "oluşturulunca adımlar gösterilir");
    const step3 = () => h.$$("#add-steps .act")[2];
    assert.equal(step3().querySelector(".badge").textContent.trim(), "3", "anahtar var, anahtarı gönder adımı henüz yapılmadı"); assert.ok(step3().querySelector('[data-task="keycopy"]'));
    await h.click('#add-steps [data-task="keycopy"]'); assert.equal(text(h, "#run-title"), "Anahtarı sunucuya gönder yeni1"); await h.click("#run-go");
    assert.deepEqual(h.calls.find(c => c.path === "/api/server-task").body, { server: "yeni1", task: "keycopy" });
    f.finish("j1", exit); await tickPoll(h, 1);
    assert.equal(step3().querySelector(".badge").textContent.trim(), expectDone ? "✔" : "3", expectDone ? "başarıyla biten görev adımı ✔ yapar" : "başarısız görev adımı ✔ yapmaz");
    assert.deepEqual(h.errors, []); h.close();
  }
});

// ---------------- Ayarlar: parola girişi tercihi ----------------
test("Ayarlar > Parola girişi: sistem penceresi yoksa devre dışı; seçim saklanır ve sonraki açılışta uygulanır", async () => {
  let h = await boot({ hash: "#v=settings", overrides: ov({ "/api/about": { ...fixture("about"), askpass: false } }) });
  const sys = h.$('#set-ask button[data-v="system"]'), inl = h.$('#set-ask button[data-v="inline"]');
  assert.equal(sys.disabled, true); assert.match(sys.title, /Bu platformda sistem penceresi yok/); assert.equal(sys.getAttribute("aria-pressed"), "true", "varsayılan: sistem");
  await h.click(inl); assert.equal(inl.getAttribute("aria-pressed"), "true"); assert.equal(sys.getAttribute("aria-pressed"), "false"); assert.equal(h.win.localStorage.getItem("sshpocket.askpass"), "inline"); h.close();
  h = await boot({ hash: "#v=settings", overrides: ov(), storage: askInline });
  assert.equal(h.$('#set-ask button[data-v="inline"]').getAttribute("aria-pressed"), "true", "kayıtlı tercih okunur"); assert.equal(h.$('#set-ask button[data-v="system"]').disabled, false, "platform destekliyorsa seçilebilir");
  await h.click('#set-ask button[data-v="system"]'); assert.equal(h.win.localStorage.getItem("sshpocket.askpass"), "system");
  assert.deepEqual(h.errors, []); h.close();
});

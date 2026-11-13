"use strict";
/* İş akışı DAVRANIŞ testleri (anlık görüntü değil, açık beklentiler): tek iş, paralel iş, arka plana alma, dock çipinden geri açma,
   aynı sunucuda ret, sayfa yenilenince yeniden bağlanma, toplu işlem ve eşzamanlılık ayarı.
   Sahte iş sunucusu (fakeJobs) gerçek Go sunucusunun /api/run, /api/jobs ve /api/batch sözleşmesini taklit eder. */
const test = require("node:test");
const assert = require("node:assert/strict");
const { boot, fixture } = require("./harness");

const mixedHome = () => { const h = fixture("home"); h.servers.forEach((s, i) => { s.checked = true; s.up = i !== 1; s.conn = s.up ? "up" : "down"; }); return h; };

const { fakeJobs } = require("./lib/fakejobs");

const base = f => ({ "/api/home": mixedHome, ...f.overrides });
const runBtn = h => h.$$("#srv-body button[data-run]").find(b => b.textContent.trim() === "Çalıştır");
const toast = h => h.$("#toast").textContent;
const openModal = h => h.$("#runmodal").classList.contains("open");
const log = h => h.$("#run-log").textContent;
const posts = f => f.posts.map(p => `${p.server}/${p.action}`);

async function startOn(h, server) { // sunucu sayfasında "Çalıştır" → pencere → "Çalıştır"
  await h.click(`[data-tsrv="${server}"]`);
  await h.click(runBtn(h)); assert.ok(openModal(h), "onay penceresi açılmalı");
  await h.click("#run-go");
}

test("tek iş: başlat, çıktı akar, başarıyla biter", async () => {
  const f = fakeJobs(); const h = await boot({ hash: "#s=web1&p=utility", overrides: base(f) });
  await h.click(runBtn(h));
  assert.ok(openModal(h)); assert.match(h.$("#run-title").textContent, /durum/); assert.ok(!h.$("#run-go").hidden);
  await h.click("#run-go");
  assert.deepEqual(posts(f), ["web1/durum"]);
  assert.ok(!h.$("#run-live").hidden); assert.match(log(h), /basladi web1/);
  f.feed("j1", "ikinci satir\n"); h.timers.run(700); await h.settle();
  assert.match(log(h), /ikinci satir/);
  f.finish("j1", 0); h.timers.run(700); await h.settle();
  assert.match(h.$("#run-state").textContent, /başarılı/); assert.ok(!h.$("#run-close").hidden);
  assert.deepEqual(h.errors, []); h.close();
});

test("tek iş: hata koduyla biter", async () => {
  const f = fakeJobs(); const h = await boot({ hash: "#s=web1&p=utility", overrides: base(f) });
  await h.click(runBtn(h)); await h.click("#run-go");
  f.finish("j1", 3); h.timers.run(700); await h.settle();
  assert.match(h.$("#run-state").textContent, /başarısız.*3/); assert.match(h.$("#run-result").textContent, /başarısız/);
  assert.deepEqual(h.errors, []); h.close();
});

test("paralel: ikinci sunucuda başlatınca ilk iş arka plana geçer, dock çipinden geri açılır", async () => {
  const f = fakeJobs(); const h = await boot({ hash: "#s=web1&p=utility", overrides: base(f) });
  await startOn(h, "web1");
  f.feed("j1", "web1 ilerliyor\n"); h.timers.run(700); await h.settle();
  await h.click("#run-min"); assert.ok(!openModal(h));
  await startOn(h, "db1"); // web1 arka plana alınır, db1 pencereye bağlanır
  assert.deepEqual(posts(f), ["web1/durum", "db1/durum"]);
  assert.match(toast(h), /web1.*arka planda/);
  assert.match(h.$("#run-sub").textContent, /db1/);
  h.timers.run(3000); await h.settle();                          // dock iş listesini yeniler
  const chips = h.$$("#dock-jobs .dock-job").map(c => c.textContent.trim().replace(/\s+/g, " "));
  assert.equal(chips.length, 2); assert.ok(chips.some(c => c.includes("web1")) && chips.some(c => c.includes("db1")));
  // arka plandaki iş sürerken çıktı birikir; çipten açınca baştan görünür
  f.feed("j1", "arka planda gelen\n"); h.timers.run(1500); await h.settle();
  await h.click(h.$$("#dock-jobs .dock-job").find(c => c.textContent.includes("web1")));
  assert.match(h.$("#run-sub").textContent, /web1/); assert.ok(openModal(h));
  assert.match(log(h), /web1 ilerliyor/); assert.match(log(h), /arka planda gelen/);
  assert.deepEqual(h.errors, []); h.close();
});

test("paralel: arka plandaki iş bitince bildirim gelir", async () => {
  const f = fakeJobs(); const h = await boot({ hash: "#s=web1&p=utility", overrides: base(f) });
  await startOn(h, "web1"); await h.click("#run-min");
  await startOn(h, "db1");
  f.finish("j1", 0); h.timers.run(1500); await h.settle();
  assert.match(toast(h), /✔ web1: durum/);
  assert.deepEqual(h.errors, []); h.close();
});

test("aynı sunucuda ikinci iş reddedilir (istemci ikinci POST göndermez)", async () => {
  const f = fakeJobs(); const h = await boot({ hash: "#s=web1&p=utility", overrides: base(f) });
  await startOn(h, "web1"); await h.click("#run-min");
  await h.click(runBtn(h)); // web1 için yeniden başlat
  assert.match(toast(h), /Bu sunucuda bir eylem çalışıyor/);
  assert.deepEqual(posts(f), ["web1/durum"]);
  assert.deepEqual(h.errors, []); h.close();
});

test("sunucu 409 dönerse hata bildirimi gösterilir", async () => {
  const f = fakeJobs(); f.start("web1"); // başka bir yerden başlamış iş
  const h = await boot({ hash: "#s=web1&p=utility", overrides: { ...base(f), "/api/jobs": () => ({ jobs: [], running: "", runningIds: [] }) } });
  await h.click(runBtn(h)); await h.click("#run-go");
  assert.match(toast(h), /Çalıştırılamadı: bu sunucuda başka bir eylem çalışıyor/);
  assert.deepEqual(h.errors, []); h.close();
});

test("sayfa yenilenince çalışan işlere yeniden bağlanır (en yenisi pencerede, diğeri arka planda)", async () => {
  const f = fakeJobs(); f.start("web1"); f.feed("j1", "eski satir\n"); f.start("db1"); f.feed("j2", "yeni satir\n");
  const h = await boot({ hash: "", overrides: base(f) });
  h.timers.run(3000); await h.settle();
  assert.equal(h.$$("#dock-jobs .dock-job").length, 2);
  assert.ok(!h.$("#job-pill").hidden, "üst çubukta iş düğmesi görünmeli");
  assert.ok(!openModal(h), "pencere kendiliğinden açılmaz");
  await h.click("#job-pill"); assert.match(h.$("#run-sub").textContent, /db1/); assert.match(log(h), /yeni satir/);
  assert.deepEqual(h.errors, []); h.close();
});

test("iptal: çalışan iş iptal edilir", async () => {
  const f = fakeJobs(); const h = await boot({ hash: "#s=web1&p=utility", overrides: base(f) });
  await h.click(runBtn(h)); await h.click("#run-go");
  await h.click("#run-cancel");
  assert.ok(h.calls.some(c => c.path === "/api/run/cancel" && c.body.id === "j1"));
  h.timers.run(700); await h.settle(); assert.match(h.$("#run-state").textContent, /başarısız/);
  assert.deepEqual(h.errors, []); h.close();
});

// ---- toplu işlem ----
test("toplu işlem: seçim → başlat → ilerleme → bitti", async () => {
  let state = null; const posted = [];
  const ov = {
    "/api/home": mixedHome,
    "/api/batch/options": { options: [{ package: "utility", action: "durum", ok: 3, total: 3 }], total: 3 },
    "/api/batch": (u, { method, body }) => {
      if (method === "POST") { posted.push(body); state = { id: "b1", package: body.package, action: body.action, stopOnFail: body.stopOnFail, parallel: body.parallel, done: false, cancelled: false, items: body.servers.map(s => ({ server: s, state: "pending", exit: 0 })) }; return { batch: "b1" }; }
      return { batch: state };
    },
  };
  const h = await boot({ hash: "#v=servers", overrides: ov });
  await h.click("#batch-open");
  assert.equal(h.$$("#batch-targets input").length, 3);
  assert.equal(h.$("#batch-par").value, "4"); assert.equal(h.$("#batch-par").max, "4");
  await h.type("#batch-par", "2");
  await h.click("#batch-go");
  assert.equal(posted.length, 1); assert.deepEqual(posted[0].servers.sort(), ["db1", "vm1", "web1"]); assert.equal(posted[0].parallel, 2); assert.equal(posted[0].stopOnFail, false);
  state.items[0].state = "running"; state.items[1].state = "running"; h.timers.run(1000); await h.settle();
  assert.equal(h.$$("#batch-rows .badge").map(b => b.textContent).filter(t => t.includes("çalışıyor")).length, 2);
  assert.equal(h.$$("#batch-rows .badge").filter(b => b.textContent.includes("bekliyor")).length, 1);
  state.items.forEach(i => { i.state = "ok"; }); state.done = true; h.timers.run(1000); await h.settle();
  assert.equal(h.$("#batch-status").textContent, "Bitti"); assert.ok(h.$("#batch-cancel").hidden);
  assert.deepEqual(h.errors, []); h.close();
});

test("toplu işlem: paralellik Ayarlar sınırını aşamaz (alan max'ı sınırdır)", async () => {
  const h = await boot({ hash: "#v=servers", overrides: { "/api/home": mixedHome, "/api/ui-settings": { maxJobs: 2, min: 1, max: 16, default: 4 }, "/api/batch/options": { options: [{ package: "utility", action: "durum", ok: 3, total: 3 }], total: 3 } } });
  await h.click("#batch-open");
  assert.equal(h.$("#batch-par").max, "2"); assert.equal(h.$("#batch-par").value, "2");
  assert.deepEqual(h.errors, []); h.close();
});

// ---- ayar ----
test("Ayarlar: aynı anda en çok değişince sunucuya kaydedilir; geçersiz değer kaydedilmez", async () => {
  const saved = [];
  const h = await boot({ hash: "#v=settings", overrides: { "/api/ui-settings": (u, { method, body }) => { if (method === "POST") { saved.push(body.maxJobs); return { maxJobs: body.maxJobs, min: 1, max: 16, default: 4 }; } return { maxJobs: 4, min: 1, max: 16, default: 4 }; } } });
  assert.equal(h.$("#set-maxjobs").value, "4");
  await h.type("#set-maxjobs", "2"); h.$("#set-maxjobs").dispatchEvent(new h.win.Event("change", { bubbles: true })); await h.settle();
  assert.deepEqual(saved, [2]); assert.equal(h.$("#set-maxjobs-st").textContent, "kaydedildi");
  h.$("#set-maxjobs").value = "99"; h.$("#set-maxjobs").dispatchEvent(new h.win.Event("change", { bubbles: true })); await h.settle();
  assert.deepEqual(saved, [2]); assert.equal(h.$("#set-maxjobs").value, "2"); assert.match(h.$("#set-maxjobs-st").textContent, /arasında/);
  assert.deepEqual(h.errors, []); h.close();
});

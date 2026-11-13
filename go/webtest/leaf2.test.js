"use strict";
/* Refaktör 2c: js/ui/wizard.js ve js/ui/knock.js için davranış + birim testleri. */
process.env.TZ = "UTC";
const test = require("node:test");
const assert = require("node:assert/strict");
const path = require("path");
const { pathToFileURL } = require("url");
const { boot, fixture } = require("./harness");

const mixedHome = () => { const h = fixture("home"); h.servers.forEach((s, i) => { s.checked = true; s.up = i !== 1; s.conn = s.up ? "up" : "down"; }); return h; };
const key = (h, k, extra = {}, target) => { (target || h.doc).dispatchEvent(new h.win.KeyboardEvent("keydown", { key: k, bubbles: true, cancelable: true, ...extra })); return h.settle(); };
const isOpen = (h, sel) => h.$(sel).classList.contains("open");

// ---------------- knockSeqOK (saf işlev) ----------------
test("knockSeqOK: 1-10 port, virgülle, her biri 1-65535, boşluksuz", async () => {
  const { knockSeqOK } = await import(pathToFileURL(path.join(__dirname, "..", "web", "js", "ui", "knock.js")).href);
  for (const ok of ["1", "65535", "7000,8000", "7000,8000,9000", "1,2,3,4,5,6,7,8,9,10", "007"]) assert.equal(knockSeqOK(ok), true, ok);
  for (const bad of ["", "0", "65536", "1,,2", "1,2,", ",1", " 1", "1 ", "1, 2", "a", "1,a", "1,2,3,4,5,6,7,8,9,10,11", "1;2", "-1", "1.5", "99999999999"]) assert.equal(knockSeqOK(bad), false, JSON.stringify(bad));
});

// ---------------- sihirbaz ----------------
const tileOf = (h, name) => h.$$("#servers [data-name]").find(el => el.dataset.name === name);
const openWizard = async (h, name) => {
  tileOf(h, name).dispatchEvent(new h.win.MouseEvent("contextmenu", { bubbles: true, cancelable: true, clientX: 5, clientY: 5 })); await h.settle();
  await h.click('#ctx button[data-ctx="draft"]');
};
const visibleStep = h => [1, 2, 3].filter(i => !h.$("#step-" + i).hidden);

test("sihirbaz: açılışta paket/sunucu bilgisi, çevrimiçi/çevrimdışı durumu, 1. adım", async () => {
  const status = () => ({ db1: { up: true, conn: "up" }, vm1: { up: false, conn: "down" }, web1: { up: true, conn: "up" } }); // durum, /api/status'tan gelir
  const h = await boot({ overrides: { "/api/home": mixedHome, "/api/status": status } });
  await openWizard(h, "web1");
  assert.ok(isOpen(h, "#wizard")); assert.deepEqual(visibleStep(h), [1]);
  assert.equal(h.$("#wiz-desc").textContent, "Paket: linux-hardening · Sunucu: web1");
  assert.equal(h.$("#w-pkg").textContent, `ad:     linux-hardening\nsürüm:  ${fixture("home").packages.find(p => p.id === "linux-hardening").version}\nkaynak: USB`);
  assert.equal(h.$("#w-srv").textContent, "ad:     web1\ndurum:  çevrimiçi");
  assert.equal(h.$("#wiz-confirm-name").textContent, "web1"); assert.equal(h.$("#confirm-input").placeholder, "web1");
  assert.ok(!h.$("#next-btn").hidden); assert.ok(h.$("#apply-btn").hidden); assert.ok(h.$("#finish-btn").hidden);
  await h.click("#wizard [data-close]"); assert.ok(!isOpen(h, "#wizard"));
  await openWizard(h, "vm1");   // son çalıştırması yok → ilk paket; çevrimdışı
  assert.equal(h.$("#wiz-desc").textContent, "Paket: kurulum · Sunucu: vm1"); assert.match(h.$("#w-srv").textContent, /durum:  bağlantı yok/);
  assert.deepEqual(h.errors, []); h.close();
});

test("sihirbaz: paket yoksa '?' gösterilir", async () => {
  const h = await boot({ overrides: { "/api/home": () => ({ ...mixedHome(), packages: [] }) } });
  await openWizard(h, "vm1");
  assert.equal(h.$("#wiz-desc").textContent, "Paket: ? · Sunucu: vm1"); assert.match(h.$("#w-pkg").textContent, /ad:     \?\nsürüm:  \?/);
  assert.deepEqual(h.errors, []); h.close();
});

test("sihirbaz: adım akışı; onay için sunucu adı birebir yazılmalı (boşluklar kırpılır); bitince kapanır; yeniden açınca 1. adım", async () => {
  const h = await boot({ overrides: { "/api/home": mixedHome } });
  await openWizard(h, "web1");
  await h.click("#next-btn"); assert.deepEqual(visibleStep(h), [2]); assert.ok(!h.$("#apply-btn").hidden); assert.equal(h.$("#apply-btn").disabled, true);
  assert.ok(h.$$("#wiz-steps .step")[0].classList.contains("done")); assert.ok(h.$$("#wiz-steps .step")[1].classList.contains("active"));
  await h.type("#confirm-input", "web"); assert.equal(h.$("#apply-btn").disabled, true, "eksik ad");
  await h.type("#confirm-input", "WEB1"); assert.equal(h.$("#apply-btn").disabled, true, "büyük/küçük harf duyarlı");
  await h.type("#confirm-input", "  web1  "); assert.equal(h.$("#apply-btn").disabled, false, "kırpılmış ad eşleşir");
  await h.click("#apply-btn"); assert.deepEqual(visibleStep(h), [3]); assert.ok(!h.$("#finish-btn").hidden); assert.ok(h.$("#next-btn").hidden);
  await h.click("#finish-btn"); assert.ok(!isOpen(h, "#wizard"));
  await openWizard(h, "db1"); assert.deepEqual(visibleStep(h), [1], "yeniden açınca 1. adım"); assert.equal(h.$("#wiz-confirm-name").textContent, "db1");
  await h.click("#next-btn"); assert.equal(h.$("#confirm-input").value, "", "2. adıma geçince onay kutusu temizlenir");
  await h.type("#confirm-input", "web1"); assert.equal(h.$("#apply-btn").disabled, true, "önceki sunucunun adı kabul edilmez");
  await h.click("#apply-btn"); assert.deepEqual(visibleStep(h), [2], "devre dışıyken uygulanmaz");
  assert.deepEqual(h.errors, []); h.close();
});

test("sihirbaz: Esc kapatır", async () => {
  const h = await boot({ overrides: { "/api/home": mixedHome } });
  await openWizard(h, "web1"); await key(h, "Escape"); assert.ok(!isOpen(h, "#wizard"));
  assert.deepEqual(h.errors, []); h.close();
});

// ---------------- port knocking formu ----------------
const knockState = { enabled: false, hasSeq: false, proto: "tcp", delay: "300", hint: "" };
const knockOv = (state, extra = {}) => ({
  "/api/home": mixedHome,
  "/api/knock": (u, { method, body }) => (method === "POST" ? (extra.post ? extra.post(body) : { ok: true }) : state),
  ...extra.over,
});
const gotoKnock = async (h, name = "web1") => { await h.click(`[data-tsrv="${name}"]`).catch(async () => {}); };
const openKnockForm = async (h) => { await h.click('#srv-body [data-task="knock"]'); };
const knockPosts = h => h.calls.filter(c => c.method === "POST" && c.path === "/api/knock").map(c => c.body);
const dlgErr = h => h.$("#dlg-err").textContent;
const fill = async (h, { seq, proto, delay }) => {
  if (seq !== undefined) { h.$("#kn-seq").value = seq; h.$("#kn-seq").dispatchEvent(new h.win.Event("input", { bubbles: true })); }
  if (proto !== undefined) h.$("#kn-proto").value = proto;
  if (delay !== undefined) h.$("#kn-delay").value = delay;
  await h.settle();
};

test("knock formu: kapalıyken açılış (başlık, varsayılan değerler, parola alanı, gönder/kapat düğmeleri yok)", async () => {
  const h = await boot({ hash: "#s=web1", overrides: knockOv(knockState) });
  await openKnockForm(h);
  assert.ok(isOpen(h, "#dlg")); assert.equal(h.$("#dlg-title").textContent, "Port knocking"); assert.equal(h.$("#dlg-sub").textContent, "web1");
  assert.equal(h.$("#kn-proto").value, "tcp"); assert.equal(h.$("#kn-delay").value, "300");
  assert.equal(h.$("#kn-seq").type, "password"); assert.equal(h.$("#kn-seq").value, "");
  assert.equal(h.$("#kn-send"), null); assert.equal(h.$("#kn-off"), null); assert.equal(h.$("#dlg-ok").textContent, "Kaydet");
  await h.click("#kn-show"); assert.equal(h.$("#kn-seq").type, "text", "göster işaretlenince metin");
  await h.click("#kn-show"); assert.equal(h.$("#kn-seq").type, "password");
  assert.deepEqual(h.errors, []); h.close();
});

test("knock formu: okuma hatası bildirim olur, pencere açılmaz", async () => {
  const h = await boot({ hash: "#s=web1", overrides: { "/api/home": mixedHome, "/api/knock": { status: 500, body: "okunamadı!" } } });
  await openKnockForm(h);
  assert.ok(!isOpen(h, "#dlg")); assert.equal(h.$("#toast").textContent, "Okunamadı: okunamadı!");
  assert.deepEqual(h.errors, []); h.close();
});

test("knock formu: doğrulama (dizi, bekleme), ipucu rengi; geçersizken hiçbir istek gitmez", async () => {
  const h = await boot({ hash: "#s=web1", overrides: knockOv(knockState) });
  await openKnockForm(h);
  await h.click("#dlg-ok"); assert.match(dlgErr(h), /^Dizi gerekli/, "kayıtlı dizi yokken boş dizi");
  for (const bad of ["70000", "1,2,abc", "1,,2", "1,2,3,4,5,6,7,8,9,10,11", "0"]) {
    await fill(h, { seq: bad }); assert.equal(dlgErr(h), "", "yazarken hata temizlenir"); assert.ok(h.$("#kn-hint").classList.contains("err"), bad + " için ipucu kırmızı");
    await h.click("#dlg-ok"); assert.match(dlgErr(h), /^Dizi 1-10 port olmalı/, bad);
  }
  await fill(h, { seq: "7000,8000" }); assert.ok(!h.$("#kn-hint").classList.contains("err"));
  for (const bad of ["abc", "6000", "5001", "", "-5", "1.5"]) { await fill(h, { delay: bad }); await h.click("#dlg-ok"); assert.equal(dlgErr(h), "Bekleme 0 ile 5000 arasında bir sayı olmalı.", JSON.stringify(bad)); }
  assert.deepEqual(knockPosts(h), [], "doğrulamadan geçmeyen form istek göndermez"); assert.equal(h.$("#dlg-ok").disabled, false);
  assert.deepEqual(h.errors, []); h.close();
});

test("knock formu: kaydet → POST, alan temizlenir, pencere kapanır, bildirim, sunucu sayfası yenilenir", async () => {
  const h = await boot({ hash: "#s=web1", overrides: knockOv(knockState) });
  await openKnockForm(h);
  await fill(h, { seq: "7000,8000,9000", proto: "udp", delay: "500" });
  const srvBefore = h.calls.filter(c => c.path === "/api/server?name=web1").length;
  await h.click("#dlg-ok");
  assert.deepEqual(knockPosts(h), [{ name: "web1", action: "save", proto: "udp", delay: "500", seq: "7000,8000,9000" }]);
  assert.equal(h.$("#kn-seq").value, "", "dizi alanı kayıttan sonra temizlenir"); assert.ok(!isOpen(h, "#dlg")); assert.equal(h.$("#toast").textContent, "Knock kaydedildi");
  assert.equal(h.calls.filter(c => c.path === "/api/server?name=web1").length, srvBefore + 1, "sunucu yeniden okunur");
  assert.deepEqual(h.errors, []); h.close();
});

test("knock formu: bekleme sınır değerleri 0 ve 5000 kabul edilir", async () => {
  for (const delay of ["0", "5000"]) {
    const h = await boot({ hash: "#s=web1", overrides: knockOv(knockState) });
    await openKnockForm(h); await fill(h, { seq: "7000", delay }); await h.click("#dlg-ok");
    assert.deepEqual(knockPosts(h), [{ name: "web1", action: "save", proto: "tcp", delay, seq: "7000" }], delay);
    assert.deepEqual(h.errors, []); h.close();
  }
});

test("knock formu: Enter kaydeder; sunucu hatası gösterilir ve düğme yeniden etkinleşir", async () => {
  const h = await boot({ hash: "#s=web1", overrides: knockOv(knockState, { post: () => ({ status: 500, body: "yazılamadı" }) }) });
  await openKnockForm(h);
  await fill(h, { seq: "7000" }); await key(h, "Enter", {}, h.$("#kn-seq"));
  assert.equal(knockPosts(h).length, 1, "Enter istek gönderir"); assert.equal(dlgErr(h), "✖ yazılamadı"); assert.equal(h.$("#dlg-ok").disabled, false); assert.ok(isOpen(h, "#dlg"));
  assert.deepEqual(h.errors, []); h.close();
});

test("knock formu: dizi kayıtlıyken boş dizi mevcut diziyi korur (seq boş gider); dizi gösterilmez", async () => {
  const h = await boot({ hash: "#s=web1", overrides: knockOv({ enabled: true, hasSeq: true, proto: "udp", delay: "250", hint: "70**" }) });
  await openKnockForm(h);
  assert.equal(h.$("#kn-seq").value, "", "kayıtlı dizi geri okunmaz"); assert.match(h.$("#kn-seq").placeholder, /kayıtlı/);
  assert.equal(h.$("#kn-proto").value, "udp"); assert.equal(h.$("#kn-delay").value, "250");
  assert.match(h.$("#dlg-body").textContent, /Knock açık: udp, 250 ms/); assert.match(h.$("#dlg-body").textContent, /70\*\*/);
  await h.click("#dlg-ok");
  assert.deepEqual(knockPosts(h), [{ name: "web1", action: "save", proto: "udp", delay: "250", seq: "" }]); assert.ok(!isOpen(h, "#dlg"));
  assert.deepEqual(h.errors, []); h.close();
});

test("knock formu: açık ama dizi yok uyarısı; boş dizi kaydedilemez", async () => {
  const h = await boot({ hash: "#s=web1", overrides: knockOv({ enabled: true, hasSeq: false, proto: "tcp", delay: "300", hint: "" }) });
  await openKnockForm(h);
  assert.ok(h.$("#dlg-body .notice.warn"), "uyarı kutusu"); assert.match(h.$("#dlg-body .notice").textContent, /dizi bu bilgisayarın Keychain'inde yok/);
  assert.equal(h.$("#kn-send"), null, "dizi yokken şimdi gönder yok"); assert.ok(h.$("#kn-off"), "kapatma var");
  await h.click("#dlg-ok"); assert.match(dlgErr(h), /^Dizi gerekli/); assert.deepEqual(knockPosts(h), []);
  assert.deepEqual(h.errors, []); h.close();
});

test("knock formu: 'şimdi gönder' ve iki tıklamalı 'kapat ve kaydı sil'", async () => {
  const h = await boot({ hash: "#s=web1", overrides: knockOv({ enabled: true, hasSeq: true, proto: "tcp", delay: "300", hint: "" }) });
  await openKnockForm(h);
  await h.click("#kn-send"); assert.deepEqual(knockPosts(h), [{ name: "web1", action: "send" }]); assert.equal(h.$("#toast").textContent, "Knock gönderildi"); assert.ok(isOpen(h, "#dlg"));
  await h.click("#kn-off"); assert.equal(h.$("#kn-off").textContent, "Emin misiniz? Tekrar tıklayın"); assert.equal(knockPosts(h).length, 1, "ilk tıklama istek göndermez");
  const srvBefore = h.calls.filter(c => c.path === "/api/server?name=web1").length;
  await h.click("#kn-off"); assert.deepEqual(knockPosts(h)[1], { name: "web1", action: "disable" });
  assert.ok(!isOpen(h, "#dlg")); assert.equal(h.$("#toast").textContent, "Knock kapatıldı"); assert.equal(h.calls.filter(c => c.path === "/api/server?name=web1").length, srvBefore + 1);
  assert.deepEqual(h.errors, []); h.close();
});

test("knock formu: gönder/kapat hataları ✖ ile gösterilir; HTML kaçışlanır", async () => {
  const h = await boot({ hash: "#s=web1", overrides: knockOv({ enabled: true, hasSeq: true, proto: "<i>x</i>", delay: "300", hint: "<b>h</b>" }, { post: () => ({ status: 500, body: "ağ hatası" }) }) });
  await openKnockForm(h);
  assert.equal(h.$$("#dlg-body .notice i, #dlg-body .notice b b").length, 0, "proto/ipucu eleman olarak enjekte edilmemeli");
  assert.match(h.$("#dlg-body .notice").textContent, /<i>x<\/i>/);
  await h.click("#kn-send"); assert.equal(dlgErr(h), "✖ ağ hatası");
  await h.click("#kn-off"); await h.click("#kn-off"); assert.equal(dlgErr(h), "✖ ağ hatası"); assert.ok(isOpen(h, "#dlg"), "hatada pencere kapanmaz");
  assert.deepEqual(h.errors, []); h.close();
});

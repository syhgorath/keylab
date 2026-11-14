"use strict";
/* Tema, Ayarlar sayfası ve ortak iletişim penceresi DAVRANIŞ testleri (refaktör 5e). Taşımadan ÖNCE eski kodda da geçer. */
const test = require("node:test");
const assert = require("node:assert/strict");
const { boot, fixture } = require("./harness");

const mixedHome = () => { const h = fixture("home"); h.servers.forEach((s, i) => { s.checked = true; s.up = i !== 1; s.conn = s.up ? "up" : "down"; }); return h; };
const base = (o = {}) => ({ "/api/home": mixedHome, ...o });
const txt = (h, s) => h.$(s).textContent.replace(/\s+/g, " ").trim();
const pressed = (h, sel) => h.$$(sel).filter(b => b.getAttribute("aria-pressed") === "true").map(b => b.dataset.v);

/* ---------------- tema ---------------- */
test("tema: varsayılan koyu; düğme koyu → otomatik → açık → koyu döner, kaydeder, öznitelik/etiket günceller", async () => {
  const h = await boot({ hash: "#v=settings", overrides: base() });
  const root = h.doc.documentElement, ico = () => h.$("#theme-ico").textContent;
  assert.equal(root.getAttribute("data-theme"), "dark"); assert.equal(ico(), "☾"); assert.equal(h.$("#theme-btn").getAttribute("aria-label"), "Tema: koyu"); assert.deepEqual(pressed(h, "#set-theme button"), ["dark"]);
  await h.click("#theme-btn"); assert.equal(root.getAttribute("data-theme"), null); assert.equal(ico(), "◐"); assert.equal(h.win.localStorage.getItem("sshpocket.theme"), "auto"); assert.deepEqual(pressed(h, "#set-theme button"), ["auto"]);
  await h.click("#theme-btn"); assert.equal(root.getAttribute("data-theme"), "light"); assert.equal(ico(), "☀"); assert.equal(h.$("#theme-btn").title, "Tema: açık (tıklayınca değişir)"); assert.equal(h.win.localStorage.getItem("sshpocket.theme"), "light");
  await h.click("#theme-btn"); assert.equal(root.getAttribute("data-theme"), "dark"); assert.equal(h.win.localStorage.getItem("sshpocket.theme"), "dark");
  await h.click('#set-theme button[data-v="light"]'); assert.equal(root.getAttribute("data-theme"), "light"); assert.deepEqual(pressed(h, "#set-theme button"), ["light"]);
  assert.deepEqual(h.errors, []); h.close();
});

test("tema: kayıtlı tercih açılışta uygulanır; geçersiz değer varsayılana (koyu) düşer", async () => {
  let h = await boot({ hash: "", overrides: base(), storage: { "sshpocket.theme": "auto" } }); assert.equal(h.doc.documentElement.getAttribute("data-theme"), null); assert.equal(h.$("#theme-ico").textContent, "◐"); h.close();
  h = await boot({ hash: "", overrides: base(), storage: { "sshpocket.theme": "light" } }); assert.equal(h.doc.documentElement.getAttribute("data-theme"), "light"); h.close();
  h = await boot({ hash: "", overrides: base(), storage: { "sshpocket.theme": "mor" } }); assert.equal(h.doc.documentElement.getAttribute("data-theme"), "dark"); h.close();
});

/* ---------------- Ayarlar: görünüm / parola girişi ---------------- */
test("ayarlar: sunucu görünümü (simgeler/liste) kaydedilir ve işaretlenir", async () => {
  const h = await boot({ hash: "#v=settings", overrides: base() });
  assert.deepEqual(pressed(h, "#set-mode button"), ["tiles"]);
  await h.click('#set-mode button[data-v="list"]'); assert.deepEqual(pressed(h, "#set-mode button"), ["list"]); assert.equal(h.win.localStorage.getItem("sshpocket.mode"), "list");
  await h.click('[data-nav="servers"]'); assert.ok(h.$("#servers-all").classList.contains("tiles") === false, "liste kipi sunucular sayfasına uygulanır");
  await h.click('[data-nav="settings"]'); assert.deepEqual(pressed(h, "#set-mode button"), ["list"], "ayarlara dönünce işaret korunur"); h.close();
});

/* ---------------- Ayarlar: eşzamanlı iş sınırı ---------------- */
test("ayarlar: eşzamanlı iş sınırı — okuma (değer/min/max), geçersiz sayı, kayıt, kayıt hatası, okuma hatası", async () => {
  let mode = "ok"; const saved = [];
  const h = await boot({ hash: "#v=settings", overrides: base({ "/api/ui-settings": (u, { method, body }) => { if (method === "POST") { if (mode === "fail") return { status: 500, body: "disk dolu" }; saved.push(body.maxJobs); return { maxJobs: body.maxJobs, min: 1, max: 8 }; } return { maxJobs: 3, min: 1, max: 8 }; } }) });
  const el = h.$("#set-maxjobs"), st = () => txt(h, "#set-maxjobs-st"), change = async v => { el.value = v; el.dispatchEvent(new h.win.Event("change", { bubbles: true })); await h.settle(); };
  assert.deepEqual([el.value, el.min, el.max], ["3", "1", "8"]);
  await change("9"); assert.equal(st(), "1 ile 8 arasında olmalı"); assert.equal(el.value, "3"); assert.deepEqual(saved, []);
  await change("0"); assert.equal(el.value, "3"); await change("abc"); assert.equal(el.value, "3"); assert.deepEqual(saved, []);
  await change("8"); assert.deepEqual(saved, [8]); assert.equal(st(), "kaydedildi"); assert.equal(el.value, "8"); await change("1"); assert.deepEqual(saved, [8, 1], "alt sınır geçerli");
  mode = "fail"; await change("5"); assert.match(st(), /^kaydedilemedi: /); assert.equal(el.value, "1", "başarısızlıkta son kayıtlı değer geri gelir");
  assert.deepEqual(h.errors, []); h.close();
  const g = await boot({ hash: "#v=settings", overrides: base({ "/api/ui-settings": { status: 500, body: "yok" } }) }); assert.match(txt(g, "#set-maxjobs-st"), /^okunamadı: /); g.close();
});

/* ---------------- Ayarlar sayfası: yollar ve yapılandırma ---------------- */
test("ayarlar: yollar ve sshpocket.conf listelenir; boş değer '—'; conf yoksa uyarı; hata kutusu", async () => {
  let h = await boot({ hash: "#v=settings", overrides: base() });
  const rows = h.$$("#set-paths .urow").map(r => r.textContent.replace(/\s+/g, " ").trim());
  assert.deepEqual(rows, ["KeyLab sürümü0.0.0-test", "Kök klasör/fixture/path", "Sunucular/fixture/srv", "Paketler/fixture/path", "Durum ve günlükler/fixture/state", "Platformtest/test", "Gogo-test"]);
  assert.deepEqual(h.$$("#set-conf .urow").map(r => r.textContent.replace(/\s+/g, " ").trim()), ["STATUSon", "STATUS_TIMEOUT2"]); h.close();
  h = await boot({ hash: "#v=settings", overrides: base({ "/api/about": () => ({ ...fixture("about"), root: "", conf: {} }) }) });
  assert.match(txt(h, "#set-paths .urow:nth-child(2)"), /Kök klasör—/); assert.match(txt(h, "#set-conf"), /sshpocket\.conf bulunamadı ya da boş\./); h.close();
  h = await boot({ hash: "#v=settings", overrides: base({ "/api/about": () => ({ ...fixture("about"), conf: null }) }) }); assert.match(txt(h, "#set-conf"), /bulunamadı/); h.close();
  h = await boot({ hash: "#v=settings", overrides: base({ "/api/about": { status: 500, body: "bozuk" } }) }); assert.match(txt(h, "#set-paths .notice.danger"), /Yüklenemedi/); h.close();
});

test("ayarlar: sayfaya her girişte işaretler (görünüm, tema, parola) yeniden çizilir", async () => {
  const h = await boot({ hash: "#v=settings", overrides: base() });
  h.$$("#set-mode button").forEach(b => b.setAttribute("aria-pressed", "false")); h.$$("#set-theme button").forEach(b => b.setAttribute("aria-pressed", "false")); h.$$("#set-ask button").forEach(b => b.setAttribute("aria-pressed", "false"));
  await h.click('[data-nav="dash"]'); await h.click('[data-nav="settings"]');
  assert.deepEqual(pressed(h, "#set-mode button"), ["tiles"]); assert.deepEqual(pressed(h, "#set-theme button"), ["dark"]); assert.deepEqual(pressed(h, "#set-ask button"), ["system"]); h.close();
});

/* ---------------- ortak iletişim penceresi ---------------- */
test("iletişim penceresi: openDlg (ad, alt başlık, gövde, düğmeler, genişlik), Tamam çağrısı, Esc/X/Vazgeç kapatır", async () => {
  const h = await boot({ hash: "#s=web1", overrides: base() });
  await h.click("#srv-edit"); // genel pencere: adres düzenleme (dar, Kaydet + Vazgeç)
  assert.ok(h.$("#dlg").classList.contains("open")); assert.equal(h.$("#dlg-ok").textContent, "Kaydet"); assert.equal(h.$("#dlg-ok").className, "btn primary"); assert.equal(h.$("#dlg-ok").hidden, false); assert.equal(h.$("#dlg-cancel").textContent, "Vazgeç");
  assert.equal(h.$("#dlg .modal").style.width, "min(580px,94vw)"); assert.equal(h.$("#dlg-err").textContent, "");
  await h.click("#dlg-ok"); assert.equal(h.$("#dlg-err").textContent, "Değişiklik yok.", "Tamam onOk'u çağırır");
  h.doc.dispatchEvent(new h.win.KeyboardEvent("keydown", { key: "Escape", bubbles: true })); assert.ok(!h.$("#dlg").classList.contains("open"), "Esc kapatır");
  await h.click("#srv-edit"); assert.equal(h.$("#dlg-err").textContent, "", "yeniden açılışta hata temizlenir"); await h.click("#dlg-x"); assert.ok(!h.$("#dlg").classList.contains("open"));
  await h.click("#srv-edit"); await h.click("#dlg-cancel"); assert.ok(!h.$("#dlg").classList.contains("open"));
  await h.click('[data-nav="packages"]'); await h.click(h.$('#pkg-page [data-pk="utility"]')); // geniş, Tamam gizli, Kapat
  assert.equal(h.$("#dlg-ok").hidden, true); assert.equal(h.$("#dlg-cancel").textContent, "Kapat"); assert.equal(h.$("#dlg .modal").style.width, "min(860px,94vw)"); assert.equal(h.$("#dlg .modal").style.maxWidth, "860px");
  await h.click("#dlg-cancel"); await h.click("#srv-edit".replace("#srv-edit", '[data-nav="servers"]')); h.close();
});

test("iletişim penceresi: Tamam düğmesi sınıfı (okClass) ve kapalıyken Tamam tıklaması çağrı yapmaz", async () => {
  const h = await boot({ hash: "", overrides: base({ "/api/server-files": { files: [], state: [] } }) });
  await h.click('[data-tsrv="db1"]'); await h.click("#srv-more"); await h.click('#ctx [data-ctx="delete"]');
  assert.ok(h.$("#dlg").classList.contains("open")); assert.equal(h.$("#dlg-ok").className, "btn dangerbtn"); assert.equal(h.$("#dlg-ok").textContent, "Sil"); h.close();
});

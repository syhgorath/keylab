"use strict";
/* Çıkış düğmesi: onay penceresi, USB bilgisi, çalışan iş (409 → ikinci onay → force), kapatıldı perdesi. Gerçek çıkarma Go testlerinde (quit_test.go). */
const test = require("node:test");
const assert = require("node:assert/strict");
const { boot, fixture } = require("./harness");

const base = (o = {}) => ({ "/api/home": () => fixture("home"), ...o });
const about = usb => () => ({ ...fixture("about"), usb });
const posts = h => h.calls.filter(c => c.method === "POST" && c.path === "/api/quit").map(c => c.body);
const noclose = h => { h.closed = 0; h.win.close = () => { h.closed++; }; return h; }; // jsdom window.close() belgeyi yok eder
const dlgText = h => h.$("#dlg").textContent.replace(/\s+/g, " ").trim();

test("çıkış: düğme onay penceresini açar; USB varsa adı ve anahtar silme bilgisi görünür; Vazgeç hiçbir şey göndermez", async () => {
  const h = await boot({ hash: "", overrides: base({ "/api/about": about("test") }) });
  await h.click("#quit-btn");
  assert.ok(h.$("#dlg").classList.contains("open")); assert.equal(h.$("#dlg-title").textContent, "Çıkış"); assert.equal(h.$("#dlg-ok").textContent, "Çıkış yap"); assert.equal(h.$("#dlg-ok").className, "btn danger");
  assert.match(dlgText(h), /Açık terminaller kapanır/); assert.match(dlgText(h), /USB çıkarılır: test/); assert.match(dlgText(h), /Keychain kayıtlarına dokunulmaz/);
  await h.click("#dlg-cancel"); assert.ok(!h.$("#dlg").classList.contains("open")); assert.deepEqual(posts(h), []); assert.equal(h.$("#quit-done").hidden, true);
  assert.deepEqual(h.errors, []); h.close();
});

test("çıkış: USB yoksa USB satırı çıkmaz; onaylayınca {force:false} gider ve perde 'durduruldu' der", async () => {
  const h = await boot({ hash: "", overrides: base({ "/api/about": about(""), "/api/quit": { ok: true, terms: 0, usb: "", ejected: false, log: "" } }) }); noclose(h);
  await h.click("#quit-btn"); assert.doesNotMatch(dlgText(h), /USB çıkarılır/);
  await h.click("#dlg-ok");
  assert.equal(h.closed, 1, "window.close denenir"); assert.deepEqual(posts(h), [{ force: false }]); assert.ok(!h.$("#dlg").classList.contains("open"));
  assert.equal(h.$("#quit-done").hidden, false); assert.equal(h.$("#quit-done-text").textContent, "KeyLab durduruldu. Bu pencereyi kapatabilirsiniz."); assert.equal(h.$("#quit-done-log").textContent, "");
  assert.deepEqual(h.errors, []); h.close();
});

test("çıkış: USB çıkarıldı / çıkarılamadı iletisi ve günlük perdede görünür", async () => {
  let h = await boot({ hash: "", overrides: base({ "/api/about": about("test"), "/api/quit": { ok: true, usb: "test", ejected: true, log: "✅ USB çıkarıldı" } }) }); noclose(h);
  await h.click("#quit-btn"); await h.click("#dlg-ok");
  assert.match(h.$("#quit-done-text").textContent, /^USB çıkarıldı: test\. KeyLab durduruldu/); assert.equal(h.$("#quit-done-log").textContent, "✅ USB çıkarıldı"); h.close();
  h = await boot({ hash: "", overrides: base({ "/api/about": about("test"), "/api/quit": { ok: true, usb: "test", ejected: false, log: "⚠️  USB çıkarılamadı" } }) }); noclose(h);
  await h.click("#quit-btn"); await h.click("#dlg-ok");
  assert.match(h.$("#quit-done-text").textContent, /^USB çıkarılamadı, elle çıkarın: test\./); h.close();
});

test("çıkış: uygulama USB'den çalışıyorsa çıkarma ertelenir; perde bunu söyler (yanlışlıkla 'çıkarıldı' denmez)", async () => {
  const h = await boot({ hash: "", overrides: base({ "/api/about": about("test"), "/api/quit": { ok: true, usb: "test", ejected: false, deferred: true, log: "📤 ertelendi" } }) }); noclose(h);
  await h.click("#quit-btn"); await h.click("#dlg-ok");
  assert.match(h.$("#quit-done-text").textContent, /^USB uygulama kapandıktan hemen sonra çıkarılacak: test\. KeyLab durduruldu/);
  assert.doesNotMatch(h.$("#quit-done-text").textContent, /USB çıkarıldı/); assert.deepEqual(h.errors, []); h.close();
});

test("çıkış: çalışan iş varsa 409 → uyarılı ikinci onay → force:true ile çıkar", async () => {
  let n = 0;
  const h = await boot({ hash: "", overrides: base({ "/api/about": about("test"), "/api/quit": () => (++n === 1 ? { status: 409, body: { error: "running", jobs: 2 } } : { ok: true, usb: "test", ejected: true, log: "" }) }) }); noclose(h);
  await h.click("#quit-btn"); await h.click("#dlg-ok");
  assert.deepEqual(posts(h), [{ force: false }]); assert.equal(h.$("#quit-done").hidden, true); assert.ok(h.$("#dlg").classList.contains("open"));
  assert.match(dlgText(h), /2 iş çalışıyor: çıkarsanız yarıda kesilir/); assert.equal(h.$("#dlg-ok").textContent, "Yine de çık"); assert.equal(h.$("#dlg-title").textContent, "Çıkış");
  await h.click("#dlg-ok"); assert.deepEqual(posts(h), [{ force: false }, { force: true }]); assert.equal(h.$("#quit-done").hidden, false);
  assert.deepEqual(h.errors, []); h.close();
});

test("çıkış: sunucu hata verirse pencerede gösterilir, düğme yeniden açılır, perde çıkmaz", async () => {
  const h = await boot({ hash: "", overrides: base({ "/api/about": about(""), "/api/quit": { status: 500, body: "bozuk" } }) });
  await h.click("#quit-btn"); await h.click("#dlg-ok");
  assert.equal(h.$("#dlg-err").textContent, "bozuk"); assert.equal(h.$("#dlg-ok").disabled, false); assert.equal(h.$("#quit-done").hidden, true); assert.ok(h.$("#dlg").classList.contains("open"));
  h.close();
});

test("çıkış: /api/about USB bilgisi vermezse (eski yanıt) USB satırı olmadan açılır", async () => {
  const h = await boot({ hash: "", overrides: base({ "/api/about": () => fixture("about") }) });
  await h.click("#quit-btn"); assert.ok(h.$("#dlg").classList.contains("open")); assert.doesNotMatch(dlgText(h), /USB çıkarılır/); h.close();
});

test("çıkış: İngilizce arayüzde metinler çevrilir", async () => {
  const h = await boot({ hash: "", overrides: base({ "/api/about": about("test") }), storage: { "keylab.lang": "en" } });
  await h.click("#quit-btn"); assert.equal(h.$("#dlg-title").textContent, "Quit"); assert.equal(h.$("#dlg-ok").textContent, "Quit"); assert.match(dlgText(h), /The USB is ejected: test/); h.close();
});

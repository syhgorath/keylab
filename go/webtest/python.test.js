"use strict";
/* Python ön denetimi (js/ui/python.js): Python/Ansible gerektiren paket eylemi başlamadan önce uygun Python yoksa "önce kurun" penceresi çıkar; eylem başlamaz. */
process.env.TZ = "UTC";
const test = require("node:test");
const assert = require("node:assert/strict");
const { boot } = require("./harness");
const { mixedHome, status } = require("./lib/fakejobs");

const MISSING = { needed: true, ok: false, range: ">=3.12", found: ["3.9.6"], stub: true, guide: "g" };
const ov = extra => ({ "/api/home": mixedHome, "/api/status": status, ...extra });
const isOpen = (h, s) => h.$(s).classList.contains("open");
const WRITE = '#srv-body [data-write="linux-hardening|audit"]';

test("python yok: eylem düğmesi 'Önce Python kurmalısınız' penceresini açar, çalıştırma penceresi AÇILMAZ", async () => {
  for (const sel of [WRITE, '#srv-body [data-term]']) {
    const h = await boot({ hash: "#s=web1&p=linux-hardening", overrides: ov({ "/api/python": () => MISSING }) });
    const b = h.$(sel); if (!b) { h.close(); continue; }
    await h.click(sel); await h.settle();
    assert.ok(isOpen(h, "#dlg"), sel); assert.equal(h.$("#dlg-title").textContent, "Önce Python kurmalısınız");
    const body = h.$("#dlg-body").textContent; assert.match(body, /3\.9\.6/); assert.match(body, /brew install python@3\.13/); assert.match(body, /Command Line Tools/); assert.match(body, /lisans/);
    assert.match(h.$("#dlg-sub").textContent, />=3\.12/); assert.equal(h.$("#dlg-ok").textContent, "Yeniden denetle");
    assert.ok(!isOpen(h, "#runmodal"), "eylem başlamadı"); assert.equal(h.calls.filter(c => c.path.startsWith("/api/python"))[0].path, "/api/python?package=linux-hardening");
    assert.deepEqual(h.errors, []); h.close();
  }
});

test("python yok → kurulunca 'Yeniden denetle': hâlâ yoksa uyarı kalır; varsa pencere kapanır ve eylem penceresi açılır", async () => {
  let st = MISSING;
  const h = await boot({ hash: "#s=web1&p=linux-hardening", overrides: ov({ "/api/python": () => st }) });
  await h.click(WRITE); await h.settle(); assert.ok(isOpen(h, "#dlg"));
  await h.click("#dlg-ok"); await h.settle(); assert.ok(isOpen(h, "#dlg")); assert.match(h.$("#dlg-err").textContent, /Hâlâ uygun Python/); assert.ok(!isOpen(h, "#runmodal"));
  st = { needed: true, ok: true, range: ">=3.12", found: [], stub: false, guide: "" };
  await h.click("#dlg-ok"); await h.settle(); assert.ok(!isOpen(h, "#dlg")); assert.ok(isOpen(h, "#runmodal"), "eylem kaldığı yerden açıldı");
  assert.deepEqual(h.errors, []); h.close();
});

test("python var ya da gerekmiyorsa ya da denetlenemezse eylem engellenmez (denetim hatası eylemi kilitlemez)", async () => {
  for (const o of [{ "/api/python": () => ({ needed: true, ok: true, range: ">=3.12", found: [], stub: false, guide: "" }) }, { "/api/python": () => ({ needed: false, ok: true, range: "", found: [], stub: false, guide: "" }) }, { "/api/python": () => ({ status: 500, body: "x" }) }]) {
    const h = await boot({ hash: "#s=web1&p=linux-hardening", overrides: ov(o) });
    await h.click(WRITE); await h.settle(); assert.ok(isOpen(h, "#runmodal")); assert.ok(!isOpen(h, "#dlg")); h.close();
  }
});

test("python: İngilizce arayüzde pencere çevrilir", async () => {
  const h = await boot({ hash: "#s=web1&p=linux-hardening", lang: "en", overrides: ov({ "/api/python": () => MISSING }) });
  await h.click(WRITE); await h.settle(); assert.equal(h.$("#dlg-title").textContent, "Install Python first"); assert.equal(h.$("#dlg-ok").textContent, "Check again"); h.close();
});

"use strict";
/* 10 sn'lik otomatik yenileme (app.js sonundaki setInterval): sihirbaz açıkken ya da sayfa gizliyken yenilemez.
   Refaktör 2c'de sihirbaz app.js'ten çıkarılırken bu zamanlayıcının `wiz` başvurusu kalmıştı; hiçbir test 10 sn'lik zamanlayıcıyı
   çalıştırmadığı için yakalanmadı. Bu test o açığı kapatır. */
const test = require("node:test");
const assert = require("node:assert/strict");
const { boot, fixture } = require("./harness");

const mixedHome = () => { const h = fixture("home"); h.servers.forEach((s, i) => { s.checked = true; s.up = i !== 1; s.conn = s.up ? "up" : "down"; }); return h; };

test("otomatik yenileme: 10 sn'de yenilenir; sihirbaz açıkken ve sayfa gizliyken yenilenmez; hata vermez", async () => {
  const h = await boot({ overrides: { "/api/home": mixedHome } });
  const homeCalls = () => h.calls.filter(c => c.path === "/api/home").length;
  const n0 = homeCalls(); h.timers.run(10000); await h.settle();
  assert.equal(homeCalls(), n0 + 1, "10 sn sonra yenilenir");
  // sihirbaz açıkken yenileme atlanır
  const tile = h.$$("#servers [data-name]")[0];
  tile.dispatchEvent(new h.win.MouseEvent("contextmenu", { bubbles: true, cancelable: true, clientX: 5, clientY: 5 })); await h.settle();
  await h.click('#ctx button[data-ctx="draft"]'); assert.ok(h.$("#wizard").classList.contains("open"));
  const n1 = homeCalls(); h.timers.run(10000); await h.settle(); assert.equal(homeCalls(), n1, "sihirbaz açıkken yenilenmez");
  await h.click("#wizard [data-close]"); h.timers.run(10000); await h.settle(); assert.equal(homeCalls(), n1 + 1, "kapanınca yeniden yenilenir");
  // sayfa gizliyken yenilenmez
  Object.defineProperty(h.doc, "hidden", { value: true, configurable: true });
  const n2 = homeCalls(); h.timers.run(10000); await h.settle(); assert.equal(homeCalls(), n2, "sayfa gizliyken yenilenmez");
  assert.deepEqual(h.errors, []); h.close();
});

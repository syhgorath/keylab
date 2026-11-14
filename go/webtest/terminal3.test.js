"use strict";
/* Görev terminali (paket eylemi gömülü terminalde): window.KeyLabTerminal.openTask → /api/term/task, sekme adı, bitince "keylab:taskexit" olayı,
   çalışan eylemi kapatmak için iki adımlı onay (gömülü pencerede confirm() çalışmaz), sayfa yenilenince geri bağlanma. */
const test = require("node:test");
const assert = require("node:assert/strict");
const { boot, fixture } = require("./harness");

const mixedHome = () => { const h = fixture("home"); h.servers.forEach((s, i) => { s.checked = true; s.up = i !== 1; s.conn = s.up ? "up" : "down"; }); return h; };
const api = (o = {}) => ({
  "/api/home": mixedHome, "/api/term/open": { id: "t1" }, "/api/term/task": { id: "k1", kind: "task", title: "linux-hardening · apply" },
  "/api/term/close": { ok: true }, "/api/term/resize": { ok: true }, "/api/term/input": { ok: true }, ...o
});
const posts = (h, p) => h.calls.filter(c => c.method === "POST" && c.path === p).map(c => c.body);
const isOpen = h => h.$("#termmodal").classList.contains("open");
const plain = v => JSON.parse(JSON.stringify(v)); // jsdom penceresinden gelen nesneler başka bir bölgede (realm) üretilir
const SPEC = { server: "web1", package: "linux-hardening", action: "apply", snapshot: true, confirm: "web1", ackDestructive: false };

test("görev: openTask /api/term/task'a eylem bilgisini ve terminal boyutunu gönderir; sekme '⚙ paket · eylem' adıyla açılır", async () => {
  const h = await boot({ hash: "", overrides: api() });
  const r = await h.win.KeyLabTerminal.openTask(SPEC); await h.settle();
  assert.deepEqual(plain(r), { ok: true, id: "k1" });
  assert.deepEqual(posts(h, "/api/term/task"), [{ ...SPEC, rows: 24, cols: 80 }]);
  assert.ok(isOpen(h)); assert.equal(h.$("#term-title").textContent, "⚙ linux-hardening · apply");
  assert.equal(h.$("#term-tabs .term-chip .mono").textContent, "⚙ linux-hardening · apply");
  assert.deepEqual(h.win.__es, ["/api/term/stream?id=k1&ticket=tk-test"]); assert.deepEqual(h.errors, []); h.close();
});

test("görev: sunucu başlatamazsa (kapı/çakışma) boş sekme kalmaz, hata çağırana döner", async () => {
  const h = await boot({ hash: "", overrides: api({ "/api/term/task": { status: 409, body: "bu sunucuda zaten çalışan bir eylem var" } }) });
  const r = await h.win.KeyLabTerminal.openTask(SPEC); await h.settle();
  assert.deepEqual(plain(r), { ok: false, error: "bu sunucuda zaten çalışan bir eylem var" });
  assert.equal(h.$$("#term-tabs .term-chip").length, 0); assert.equal(h.$$("#term-dock .term-chip").length, 0); assert.ok(!isOpen(h));
  assert.equal((h.win.__es || []).length, 0); assert.deepEqual(posts(h, "/api/term/close"), []); assert.deepEqual(h.errors, []); h.close();
});

test("görev: bitince 'keylab:taskexit' olayı (sunucu, paket, eylem, çıkış kodu) ve 'eylem bitti' satırı gelir; ssh oturumu olay göndermez", async () => {
  const h = await boot({ hash: "", overrides: api() });
  const got = []; h.win.addEventListener("keylab:taskexit", e => got.push(plain(e.detail)));
  await h.win.KeyLabTerminal.openTask(SPEC); await h.settle();
  h.win.__esObj[0].emit("exit", "3"); await h.settle();
  assert.deepEqual(got, [{ server: "web1", pkg: "linux-hardening", action: "apply", code: 3 }]);
  // aynı pencerede bir ssh oturumu açılıp bitince görev olayı GELMEZ
  await h.win.KeyLabTerminal.open("web1"); await h.settle();
  h.win.__esObj[1].emit("exit", "0"); await h.settle();
  assert.equal(got.length, 1); assert.deepEqual(h.errors, []); h.close();
});

test("görev: çalışan eylemin ✕'i iki adımlıdır (ilk basış uyarır, 6 sn içindeki ikinci kapatır); biten eylem tek basışta kapanır", async () => {
  const h = await boot({ hash: "", overrides: api() });
  await h.win.KeyLabTerminal.openTask(SPEC); await h.settle();
  const x = () => h.$("#term-tabs .term-chip .term-chip-x");
  await h.click(x());
  assert.deepEqual(posts(h, "/api/term/close"), [], "ilk basış kapatmaz");
  assert.match(h.$("#term-note").textContent, /Eylem hâlâ çalışıyor/); assert.equal(h.$("#term-tabs .term-chip") !== null, true);
  await h.click(x());
  assert.deepEqual(posts(h, "/api/term/close"), [{ id: "k1" }], "ikinci basış kapatır"); assert.equal(h.$$("#term-tabs .term-chip").length, 0);
  // eylem bitmişse onay istemez
  const g = await boot({ hash: "", overrides: api() });
  await g.win.KeyLabTerminal.openTask(SPEC); await g.settle();
  g.win.__esObj[0].emit("exit", "0"); await g.settle();
  await g.click(g.$("#term-tabs .term-chip .term-chip-x"));
  assert.deepEqual(posts(g, "/api/term/close"), [{ id: "k1" }]); assert.deepEqual([...h.errors, ...g.errors], []); h.close(); g.close();
});

test("görev: kırmızı düğme (pencereyi kapat) da çalışan eylem varken iki adımlıdır", async () => {
  const h = await boot({ hash: "", overrides: api() });
  await h.win.KeyLabTerminal.openTask(SPEC); await h.settle();
  await h.click("[data-term-close-all]");
  assert.deepEqual(posts(h, "/api/term/close"), []); assert.ok(isOpen(h)); assert.match(h.$("#term-note").textContent, /yarıda keser/);
  await h.click("[data-term-close-all]");
  assert.deepEqual(posts(h, "/api/term/close"), [{ id: "k1" }]); assert.ok(!isOpen(h)); assert.deepEqual(h.errors, []); h.close();
});

test("görev: sayfa yenilenince /api/term/list'ten görev sekmesi türü ve adıyla geri gelir", async () => {
  const h = await boot({ hash: "", overrides: api({ "/api/term/list": [{ id: "k9", server: "web1", done: false, kind: "task", title: "utility · durum", pkg: "utility", action: "durum" }, { id: "t5", server: "web1", done: false, kind: "", title: "" }] }) });
  await h.settle();
  const names = h.$$("#term-dock .term-chip .mono").map(n => n.textContent);
  assert.deepEqual(names, ["⚙ utility · durum", ">_ web1"]); assert.deepEqual(h.errors, []); h.close();
});
